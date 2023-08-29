package hypershift_test

import (
	"fmt"
	"os/exec"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = ginkgo.Describe("Create AWS hosted cluster", ginkgo.Label("e2e", "create", TYPE_AWS), func() {
	var config Config
	ginkgo.BeforeEach(func() {
		// GetClusterName with error handling
		clusterName, err := utils.GenerateClusterName("acmqe-hc")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.ClusterName = clusterName

		// GetInstanceType with error handling
		instanceType, err := utils.GetInstanceType(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.InstanceType = instanceType

		// GetBaseDomain with error handling
		baseDomain, err := utils.GetBaseDomain(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.BaseDomain = baseDomain

		// GetRegion with error handling
		region, err := utils.GetRegion(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Region = region

		// GetNodePoolReplicas with error handling
		nodePoolReplicas, err := utils.GetNodePoolReplicas(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.NodePoolReplicas = nodePoolReplicas

		// GetReleaseImage with error handling
		releaseImage, err := utils.GetReleaseImage(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.ReleaseImage = releaseImage // TODO allow empty and default to latest release image

		// GetNamespace with error handling
		namespace, err := utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Namespace = namespace // TODO allow empty or default clusters ns

		// GetPullSecret with error handling
		pullSecret, err := utils.GetPullSecret()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.PullSecret = pullSecret

		// GetAWSCreds with error handling
		awsCreds, err := utils.GetAWSCreds()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.AWSCreds = awsCreds
	})

	ginkgo.It("Creates an AWS Hosted Cluster", ginkgo.Label("create", TYPE_AWS), func() {
		startTime := time.Now()
		// TODO ensure auto-import is enabled
		// TODO check disable auto-import, MC not auto created even after HCP is ready

		commandArgs := []string{
			"create", "cluster", TYPE_AWS,
			"--name", config.ClusterName,
			"--aws-creds", config.AWSCreds,
			"--region", config.Region,
			"--base-domain", config.BaseDomain,
			"--pull-secret", config.PullSecret,
			"--node-pool-replicas", config.NodePoolReplicas,
			"--namespace", config.Namespace,
			"--instance-type", config.InstanceType,
			"--release-image", config.ReleaseImage,
			// --external-dns-domain=${HYPERSHIFT_EXTERNAL_DNS_DOMAIN} \
			"--generate-ssh",
		}

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		defer gexec.KillAndWait()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		utils.PrintOutput(session) // prints command, args and output

		ginkgo.By(fmt.Sprintf("Waiting for hosted cluster plane for cluster %s to be available", config.ClusterName), func() {
			utils.WaitForHCPAvailable(dynamicClient, config.ClusterName, config.Namespace)
			fmt.Printf("Time taken for the hosted control plane to be available: %s\n", time.Since(startTime).String())
		})

		// Checks to see if ManagedCluster is created and the HC is auto-imported...
		ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s to be Available", config.ClusterName), func() {
			utils.WaitForClusterImported(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster to be imported: %s\n", time.Since(startTime).String())
		})

		// Checks to see if add-ons are installed and available for the HC managed cluster...
		ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s addons are Enabled and Available", config.ClusterName), func() {
			utils.WaitForClusterAddonsAvailable(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster be imported and addons ready: %s\n", time.Since(startTime).String())
		})

		ginkgo.By(fmt.Sprintf("Checking if managed cluster %s has the correct labels", config.ClusterName), func() {
			gomega.Eventually(func() bool {
				managedClusterLabels, err := utils.GetResourceLabels(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("managedClusterLabels: %v\n", managedClusterLabels)
				return managedClusterLabels["name"] == config.ClusterName &&
					managedClusterLabels["cloud"] == "Amazon" &&
					managedClusterLabels["cluster.open-cluster-management.io/clusterset"] == "default" &&
					managedClusterLabels["vendor"] == "OpenShift"
				// UNCOMMENT BELOW ONCE https://issues.redhat.com/browse/ACM-6547 IS DONE
				//&& managedClusterLabels["open-cluster-management/created-via"] == "hypershift"
			}, eventuallyTimeoutShort).Should(gomega.BeTrue())
		})
		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Printf("========================= End Test create hosted cluster %s ===============================", config.ClusterName)
	})
})
