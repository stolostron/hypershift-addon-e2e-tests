package hypershift_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	ginkgotypes "github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	libgocmd "github.com/stolostron/library-e2e-go/pkg/cmd"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	ClusterName      string
	InstanceType     string
	BaseDomain       string
	Region           string
	NodePoolReplicas string
	ReleaseImage     string
	Namespace        string
	PullSecret       string
	AWSCreds         string
	ExternalDNS      string
	SecretCredsName  string
	ClusterArch      string
	AWSStsCreds      string
	AWSRoleArn       string
}

const (
	eventuallyTimeout      = 30 * time.Minute
	eventuallyTimeoutShort = 10 * time.Minute
	// acm48634EventuallyTimeout bounds addon-ns-config / CMA polling for ACM-48634 (shorter than eventuallyTimeoutShort).
	acm48634EventuallyTimeout = 2 * time.Minute
	eventuallyInterval        = 5 * time.Second
	TYPE_AWS                  = "AWS"
	TYPE_KUBEVIRT             = "KubeVirt"
)

var (
	dynamicClient                dynamic.Interface
	kubeClient                   kubernetes.Interface
	routeClient                  routeclient.Interface
	httpc                        *http.Client
	clientClient                 client.Client
	addonClient                  addonv1alpha1client.Interface
	apiExtensionsClient          apiextensionsclient.Interface
	defaultManagedCluster        string
	defaultInstallNamespace      string
	mceNamespace                 string
	config                       Config
	err                          error
	hcpProxyEnvironmentMissing   bool
	hcpProxyCreatePayloadMissing bool
	hcpCliConsoleDownloadSpec    map[string]interface{}
	curatorEnabled               string
	fipsEnabled                  string
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Hypershift E2e Suite")
}

// This suite is sensitive to the following environment variables:
//
// - KUBECONFIG is the location of the kubeconfig file to use
func onlyHCPProxySelected() bool {
	return hcpProxyOnlyFilter(ginkgo.GinkgoLabelFilter())
}

var hcpProxyFilterLabels = map[string]bool{
	"hcp-proxy": true,
	"acm-38016": true,
	"acm-37268": true,
	"acm-37269": true,
	"acm-39216": true,
	"acm-39226": true,
	"acm-39227": true,
	"acm-39228": true,
	"acm-42875": true,
}

func hcpProxyOnlyFilter(filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return false
	}

	if _, err := ginkgotypes.ParseLabelFilter(filter); err != nil {
		return false
	}

	tokens, ok := labelFilterTokens(filter)
	if !ok {
		// Regex filters can match labels that are not explicitly named. Treat
		// them as mixed/non-proxy filters rather than guessing.
		return false
	}

	proxyLabels := make([]string, 0, len(hcpProxyFilterLabels))
	for label := range hcpProxyFilterLabels {
		proxyLabels = append(proxyLabels, label)
	}
	if !ginkgo.Label(proxyLabels...).MatchesLabelFilter(filter) {
		return false
	}

	nonProxyTokens := make([]string, 0)
	for _, token := range tokens {
		if !hcpProxyFilterLabels[token] {
			nonProxyTokens = append(nonProxyTokens, token)
		}
	}
	if nonProxySubsetMatchesFilter(filter, nonProxyTokens) {
		return false
	}
	return true
}

func labelFilterTokens(filter string) ([]string, bool) {
	var tokens []string
	runes := []rune(filter)
	for i := 0; i < len(runes); {
		switch runes[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '&':
			if i+1 >= len(runes) || runes[i+1] != '&' {
				return nil, false
			}
			i += 2
		case '|':
			if i+1 >= len(runes) || runes[i+1] != '|' {
				return nil, false
			}
			i += 2
		case '!', ',', '(', ')':
			i++
		case '/':
			// A regex can match labels that are not explicitly named. Treat it
			// as a mixed/non-proxy filter rather than guessing.
			return nil, false
		default:
			start := i
			for i < len(runes) && !strings.ContainsRune(" \t\r\n&|!,()/", runes[i]) {
				i++
			}
			token := strings.ToLower(strings.TrimSpace(string(runes[start:i])))
			if token != "" {
				tokens = append(tokens, token)
			}
		}
	}
	return tokens, true
}

func nonProxySubsetMatchesFilter(filter string, tokens []string) bool {
	// A non-proxy spec may have any subset of the non-proxy labels mentioned by
	// the filter (and none of the proxy labels). Enumerating those subsets lets
	// boolean and negated filters be evaluated without hard-coding every
	// non-proxy describe or It label.
	if len(tokens) > 20 {
		return true
	}
	for mask := uint64(0); mask < uint64(1)<<len(tokens); mask++ {
		labels := []string{"__non_proxy__"}
		for i, token := range tokens {
			if mask&(uint64(1)<<i) != 0 {
				labels = append(labels, token)
			}
		}
		if ginkgo.Label(labels...).MatchesLabelFilter(filter) {
			return true
		}
	}
	return false
}

func hcpProxyKubeconfigAvailable() bool {
	path := os.Getenv("KUBECONFIG")
	if path == "" {
		home, err := user.Current()
		if err != nil {
			return false
		}
		path = filepath.Join(home.HomeDir, ".kube", "config")
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func initializeHCPProxySuite() {
	defaultManagedCluster = os.Getenv("MANAGED_CLUSTER_NAME")
	if defaultManagedCluster == "" {
		defaultManagedCluster = utils.LocalClusterName
	}
	defaultInstallNamespace = "open-cluster-management-agent-addon"

	if !hcpProxyKubeconfigAvailable() {
		hcpProxyEnvironmentMissing = true
		return
	}

	cfg, err := utils.NewKubeConfig()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	kubeClient, err = kubernetes.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	dynamicClient, err = dynamic.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	httpc, err = rest.HTTPClientFor(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	addonClient, err = addonv1alpha1client.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	libgocmd.InitFlags(nil)
	if optionsErr := utils.InitVars(); optionsErr == nil {
		if image, imageErr := utils.GetReleaseImage(TYPE_AWS); imageErr == nil {
			config.ReleaseImage = image
		}
	}
	if image := os.Getenv("HCP_PROXY_RELEASE_IMAGE"); image != "" {
		config.ReleaseImage = image
	}
	hcpProxyCreatePayloadMissing = config.ReleaseImage == "" && os.Getenv("HCP_PROXY_CREATE_REQUEST_TEMPLATE") == ""
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	var err error

	if onlyHCPProxySelected() {
		return []byte{1}
	}

	defaultManagedCluster = os.Getenv("MANAGED_CLUSTER_NAME")
	if defaultManagedCluster == "" {
		defaultManagedCluster = utils.LocalClusterName
	}

	defaultInstallNamespace = "open-cluster-management-agent-addon"

	defer ginkgo.GinkgoRecover()

	dynamicClient, err = utils.NewDynamicClient()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	// kube client
	kubeClient, err = utils.NewKubeClient()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cfg, err := utils.NewKubeConfig()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	// route client
	routeClient, err = utils.NewRouteV1Client()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	apiExtensionsClient, err = apiextensionsclient.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	clientClient, err = client.New(cfg, client.Options{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// http client
	httpc, err = rest.HTTPClientFor(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	addonClient, err = addonv1alpha1client.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	libgocmd.InitFlags(nil)
	err = utils.InitVars()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("The init options failed due to : %v", err))
	}

	mceNamespace, err = utils.GetMCENamespace(dynamicClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	ginkgo.By("Check & Print the hcp cli version running version on the system")
	// use gomega gexec function to run the command hypershift version and print it out
	command := exec.Command(utils.HypershiftCLIName, "version")
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	defer gexec.KillAndWait()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).Should(gexec.Exit(0))

	// TODO: check if s3 secret exists and is on the hub
	// AWS only
	ginkgo.By("Checking if the oidc aws s3 secret exists on the hub (Required only for AWS)")
	oidcProviderCredential, err := utils.GetS3Creds()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = utils.CreateOIDCProviderSecret(context.TODO(), kubeClient, "acmqe-hypershift", oidcProviderCredential, "us-east-1", defaultManagedCluster)
	if err != nil {
		gomega.Expect(apierrors.IsAlreadyExists(err)).Should(gomega.BeTrue())
		fmt.Printf("Secret hypershift-operator-oidc-provider-s3-credentials already exists in namespace %s\n", defaultManagedCluster)
	} else {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	ginkgo.By("Check if the hypershift operator is healthy by checking both operator and external-dns deployments")
	gomega.Eventually(func() error {
		return utils.IsHypershiftOperatorHealthy(kubeClient)
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	ginkgo.By("Check the addon manager on the hub was installed")
	gomega.Eventually(func() error {
		_, err = kubeClient.AppsV1().Deployments(mceNamespace).Get(context.TODO(), utils.HypershiftAddonMgrName, metav1.GetOptions{})
		ginkgo.GinkgoWriter.Println(err)
		return err
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	ginkgo.By("Check the hypershift-addon on the hub is in Available status")
	gomega.Eventually(func() error {
		fmt.Println("Checking if hypershift-addon is available on the hub...")
		err = utils.ValidateClusterAddOnAvailable(dynamicClient, utils.LocalClusterName, utils.HypershiftAddonName)
		ginkgo.GinkgoWriter.Println(err)
		return err
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	// TODO check if console is enabled first
	ginkgo.By(fmt.Sprintf("Check the ConsoleCLIDownload %s is exists on the hub", utils.HCPCliDownloadName))
	hcpCliDownload, err := utils.GetHCPConsoleCliDownload(dynamicClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	hcpCliConsoleDownloadSpec = hcpCliDownload.Object["spec"].(map[string]interface{})
	gomega.Expect(hcpCliConsoleDownloadSpec).ShouldNot(gomega.BeNil())

	// initialize config object to be used for tests
	// most likely these won't change, but if they do need to they can be changed in the test
	// we won't configure the default name

	// GetInstanceType with error handling
	config.InstanceType, err = utils.GetInstanceType(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetBaseDomain with error handling
	config.BaseDomain, err = utils.GetBaseDomain(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetRegion with error handling
	config.Region, err = utils.GetRegion(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// The hcp CLI's AWS SDK (v2) needs a region in its environment to resolve the
	// STS AssumeRole endpoint when using --sts-creds; the --region flag only sets
	// the cluster infra region, not the SDK's STS client. Without this the CLI
	// fails with "AssumeRole ... Invalid Configuration: Missing Region". Every
	// hcp exec.Command below inherits this process env (nil cmd.Env), so set it once.
	// Prefer HCP_REGION (exported by the CI create stage); fall back to the region
	// resolved above from options.yaml.
	awsRegion := os.Getenv("HCP_REGION")
	if awsRegion == "" {
		awsRegion = config.Region
	}
	if awsRegion != "" {
		os.Setenv("AWS_REGION", awsRegion)
		os.Setenv("AWS_DEFAULT_REGION", awsRegion)
	}

	// GetNodePoolReplicas with error handling
	config.NodePoolReplicas, err = utils.GetNodePoolReplicas(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetReleaseImage with error handling
	// TODO allow empty and default to latest release image
	config.ReleaseImage, err = utils.GetReleaseImage(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetNamespace with error handling
	// TODO allow empty or default clusters ns
	config.Namespace, err = utils.GetNamespace(TYPE_AWS)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetPullSecret with error handling
	config.PullSecret, err = utils.GetPullSecret()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetAWSCreds with error handling
	config.AWSCreds, err = utils.GetAWSCreds()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// GetSecretCreds
	config.SecretCredsName, err = utils.GetAWSSecretCreds()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	curatorEnabled, err = utils.GetCuratorEnabled()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	fipsEnabled, err = utils.GetFIPSEnabled()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return nil
}, func(data []byte) {
	if len(data) > 0 {
		initializeHCPProxySuite()
	}
})

var _ = ginkgo.ReportAfterSuite("HyperShift E2E Report", func(report ginkgo.Report) {
	junit_report_file := os.Getenv("JUNIT_REPORT_FILE")
	if junit_report_file != "" {
		err := utils.GenerateJUnitReport(report, junit_report_file)
		if err != nil {
			fmt.Printf("Failed to generate the report due to: %v", err)
		}
	}
})
