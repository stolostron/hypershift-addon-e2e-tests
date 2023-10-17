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

var _ = ginkgo.Describe("Hosted Control Plane CLI AWS Destroy Tests:", ginkgo.Label(TYPE_AWS), func() {
	var config Config

	ginkgo.BeforeEach(func() {
		// GetNamespace with error handling
		namespace, err := utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Namespace = namespace // TODO allow empty or default clusters ns

		config.SecretCredsName, err = utils.GetAWSSecretCreds()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("Destroy all AWS hosted clusters on the hub", ginkgo.Label("destroy-all", "all"), func() {
		startTime := time.Now()

		hostedClusterList, err := utils.GetHostedClustersList(dynamicClient, TYPE_AWS, "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		// if hostedClusterList is empty, skip the test
		if len(hostedClusterList) == 0 {
			ginkgo.Skip("No hosted clusters found on the hub")
		}

		// Run destroy command on all AWS hosted clusters without waiting to verify at first
		for _, hostedCluster := range hostedClusterList {
			commandArgs := []string{
				"destroy", "cluster", TYPE_AWS,
				"--name", hostedCluster.GetName(),
				"--secret-creds", config.SecretCredsName,
				"--namespace", hostedCluster.GetNamespace(),
				"--destroy-cloud-resources",
			}

			cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
			session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
			defer gexec.KillAndWait()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
			utils.PrintOutput(session) // prints command, args and output
		}

		// Now we can verify each hosted cluster has sucecssfully been cleaned up
		for _, hostedCluster := range hostedClusterList {
			ginkgo.By(fmt.Sprintf("Waiting for hosted cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForHostedClusterDestroyed(dynamicClient, hostedCluster.GetName())
			})

			ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForClusterDetached(dynamicClient, hostedCluster.GetName())
			})
		}

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Destroy Hosted Clusters ===============================")
	})

	ginkgo.It("Destroy a AWS hosted cluster on the hub", ginkgo.Label("destroy", "single"), func() {
		startTime := time.Now()

		config.ClusterName, err = utils.GetClusterName("")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		if config.ClusterName == "" {
			ginkgo.Skip("HCP_CLUSTER_NAME is not defined. Please supply the name of the cluster to destroy before running.")
		}

		commandArgs := []string{
			"destroy", "cluster", TYPE_AWS,
			"--name", config.ClusterName,
			"--secret-creds", config.SecretCredsName,
			"--namespace", config.Namespace,
			"--destroy-cloud-resources",
		}

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		defer gexec.KillAndWait()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		utils.PrintOutput(session) // prints command, args and output

		// Now we can verify the hosted cluster has sucecssfully been cleaned up
		ginkgo.By(fmt.Sprintf("Waiting for HostedCluster %s to be removed", config.ClusterName), func() {
			utils.WaitForHostedClusterDestroyed(dynamicClient, config.ClusterName)
		})

		ginkgo.By(fmt.Sprintf("Waiting for ManagedCluster %s to be removed", config.ClusterName), func() {
			utils.WaitForClusterDetached(dynamicClient, config.ClusterName)
		})

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Destroy Hosted Cluster ===============================")
	})
})

// TODO FOR ALL
// -> Check if HC is destroyed, if not, dump HC to file?

// TODO destroyHostedCluster(hostedClusterName string)
// -> Given hosted cluster name, destroy it
// -> fail test if any errors

// TODO destroyHostedClustersLabel(label string) -> make sharable function out of it?
// -> Destroy all hosted clusters on hub with given label string
// -> use labelSelector when getting the list of hosted clusters
