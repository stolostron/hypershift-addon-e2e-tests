package hypershift_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"

	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	libgocmd "github.com/stolostron/library-e2e-go/pkg/cmd"
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
}

const (
	eventuallyTimeout      = 30 * time.Minute
	eventuallyTimeoutShort = 10 * time.Minute
	eventuallyInterval     = 5 * time.Second
	TYPE_AWS               = "aws"
	TYPE_KUBEVIRT          = "kubevirt"
)

var (
	dynamicClient           dynamic.Interface
	kubeClient              kubernetes.Interface
	addonClient             addonv1alpha1client.Interface
	defaultManagedCluster   string
	defaultInstallNamespace string
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

	kubeClient, err = utils.NewKubeClient()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cfg, err := utils.NewKubeConfig()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	addonClient, err = addonv1alpha1client.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	libgocmd.InitFlags(nil)
	err = utils.InitVars()
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("The init options failed due to : %v", err))
	}

	ginkgo.By("Check & Print the hypershift cli version running version on the system")
	// use gomega gexec function to run the command hypershift version and print it out
	command := exec.Command(utils.HypershiftCLIName, "version")
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	defer gexec.KillAndWait()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).Should(gexec.Exit(0))

	// TODO check if hypershift addon is enabled on the hub
	// If ACM 2.8 or below, then enable it if not enabled

	ginkgo.By("Check if the hypershift operator is healthy")
	gomega.Eventually(func() error {
		deployment, err :=
			kubeClient.AppsV1().Deployments(utils.HypershiftOperatorNamespace).Get(context.TODO(), utils.HypershiftOperatorName, metav1.GetOptions{})

		if err != nil {
			ginkgo.GinkgoWriter.Println(err)
			return err
		}

		if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			return errors.New("ERROR: Hypershift operator is not healthy. Available replicas is not equal to the number of replicas")
		} else {
			fmt.Printf("Hypershift operator is healthy with %d Available replicas!\n", deployment.Status.AvailableReplicas)
		}

		return err
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	ginkgo.By("Check the addon manager on the hub was installed")
	gomega.Eventually(func() error {
		_, err = kubeClient.AppsV1().Deployments("multicluster-engine").Get(context.TODO(), utils.HypershiftAddonMgrName, metav1.GetOptions{})
		ginkgo.GinkgoWriter.Println(err)
		return err
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	ginkgo.By("Check the hypershift-addon on the hub is in Available status")
	gomega.Eventually(func() error {
		// check addon pods are running
		fmt.Println("Checking if hypershift-addon is available on the hub...")
		err = utils.ValidateClusterAddOnAvailable(dynamicClient, utils.LocalClusterName, utils.HypershiftAddonName)
		ginkgo.GinkgoWriter.Println(err)
		return err
	}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

	// TODO: check dns deployment is good, hypershift-addon is still good

	// TODO: check if s3 secret exists and is on the hub
	// AWS only
	ginkgo.By("Checking if the oidc aws s3 secret exists on the hub (Required only for AWS)")
	oidcProviderCredential, err := utils.GetS3Creds()
	err = utils.CreateOIDCProviderSecret(context.TODO(), kubeClient, "acmqe-hypershift", oidcProviderCredential, "us-east-1", defaultManagedCluster)
	if err != nil {
		gomega.Expect(apierrors.IsAlreadyExists(err)).Should(gomega.BeTrue())
		fmt.Printf("Secret hypershift-operator-oidc-provider-s3-credentials already exists in namespace %s\n", defaultManagedCluster)
	} else {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		ginkgo.GinkgoWriter.Println(err)
	}
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
