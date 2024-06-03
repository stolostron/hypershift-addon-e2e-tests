package hypershift_test

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = ginkgo.Describe("Hosted Control Plane CLI AWS Destroy Tests:", ginkgo.Label(TYPE_AWS), func() {
	var config Config

	ginkgo.BeforeEach(func() {
		config.SecretCredsName, err = utils.GetAWSSecretCreds()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("Destroy all AWS hosted clusters on the hub", ginkgo.Label("destroy"), func() {
		startTime := time.Now()

		hostedClusterList, err := utils.GetHostedClustersList(dynamicClient, TYPE_AWS, "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		// if hostedClusterList is empty, skip the test
		if len(hostedClusterList) == 0 {
			ginkgo.Skip("No AWS hosted clusters found on the hub")
		}

		// Run destroy command on all AWS hosted clusters without waiting to verify at first
		for _, hostedCluster := range hostedClusterList {
			fmt.Printf("AWS Hosted Cluster found: %s\n", hostedCluster.GetName())

			commandArgs := []string{
				"destroy", "cluster", strings.ToLower(TYPE_AWS),
				"--name", hostedCluster.GetName(),
				"--namespace", hostedCluster.GetNamespace(),
				"--secret-creds", config.SecretCredsName,
				"--destroy-cloud-resources",
			}

			fmt.Println(commandArgs)

			cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
			session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)

			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
			utils.PrintOutput(session) // prints command, args and output

			defer gexec.KillAndWait()
		}

		// Verify each hosted cluster has sucecssfully been cleaned up
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

	ginkgo.It("Destroy a AWS hosted cluster on the hub", ginkgo.Label("destroy-one"), func() {
		startTime := time.Now()

		// HCP_CLUSTER_NAME should be set for this
		config.ClusterName, err = utils.GetClusterName("")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		// HCP_NAMESPACE should be set for this
		config.Namespace, err = utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		if config.ClusterName == "" {
			ginkgo.Skip("HCP_CLUSTER_NAME is not defined. Please supply the name of the cluster to destroy before running.")
		}

		commandArgs := []string{
			"destroy", "cluster", strings.ToLower(TYPE_AWS), // must be lowercase
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
