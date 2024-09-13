package hypershift_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
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
	eventuallyTimeout      = 60 * time.Minute
	eventuallyTimeoutShort = 10 * time.Minute
	eventuallyInterval     = 5 * time.Second
	TYPE_AWS               = "AWS"
	TYPE_KUBEVIRT          = "KubeVirt"
)

var (
	dynamicClient             dynamic.Interface
	kubeClient                kubernetes.Interface
	routeClient               routeclient.Interface
	httpc                     *http.Client
	clientClient              client.Client
	addonClient               addonv1alpha1client.Interface
	apiExtensionsClient       apiextensionsclient.Interface
	defaultManagedCluster     string
	defaultInstallNamespace   string
	mceNamespace              string
	config                    Config
	err                       error
	hcpCliConsoleDownloadSpec map[string]interface{}
	curatorEnabled            string
	fipsEnabled               string
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Hypershift E2e Suite")
}

// This suite is sensitive to the following environment variables:
//
// - KUBECONFIG is the location of the kubeconfig file to use
var _ = ginkgo.SynchronizedBeforeSuite(func() {
	var err error

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
	err = utils.CreateOIDCProviderSecret(context.TODO(), kubeClient, "qe-hcp-clc", oidcProviderCredential, "us-east-1", defaultManagedCluster)
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
}, func() {})

var _ = ginkgo.ReportAfterSuite("HyperShift E2E Report", func(report ginkgo.Report) {
	junit_report_file := os.Getenv("JUNIT_REPORT_FILE")
	if junit_report_file != "" {
		err := utils.GenerateJUnitReport(report, junit_report_file)
		if err != nil {
			fmt.Printf("Failed to generate the report due to: %v", err)
		}
	}
})
