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

	ginkgo.BeforeEach(func() {
		// Before each test, generate a unique cluster name to create the hosted cluster with
		config.ClusterName, err = utils.GenerateClusterName("acmqe-hc")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("Creates an AWS Hosted Cluster", ginkgo.Label("create", TYPE_AWS), func() {
		startTime := time.Now()
		// TODO ensure auto-import is enabled
		// oc get addondeploymentconfig hypershift-addon-deploy-config -n mce -ojson | jq '.spec.ports | map(.name == "autoImportDisabled") | index(true)'
		// TODO check disable auto-import, MC not auto created even after HCP is ready

		commandArgs := []string{
			"create", "cluster", TYPE_AWS,
			"--name", config.ClusterName,
			"--secret-creds", config.SecretCredsName,
			"--region", config.Region,
			"--node-pool-replicas", config.NodePoolReplicas,
			"--namespace", config.Namespace,
			"--instance-type", config.InstanceType,
			"--release-image", config.ReleaseImage,
			// "--pull-secret", config.PullSecret,
			"--external-dns-domain", config.ExternalDNS,
			"--generate-ssh",
		}

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		defer gexec.KillAndWait()
		utils.PrintOutput(session) // prints command, args and output
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

		// TODO check if cluster has external-dns applied by checking HC conditions, api url, etc.

		ginkgo.By(fmt.Sprintf("Checking if managed cluster %s has the correct labels", config.ClusterName), func() {
			gomega.Eventually(func() bool {
				managedClusterLabels, err := utils.GetResourceLabels(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("managedClusterLabels: %v\n", managedClusterLabels)
				return managedClusterLabels["name"] == config.ClusterName &&
					managedClusterLabels["cloud"] == "Amazon" &&
					managedClusterLabels["cluster.open-cluster-management.io/clusterset"] == "default" &&
					managedClusterLabels["vendor"] == "OpenShift"
				// TODO check ocp version e.g. openshiftVersion: 4.14.0-ec.4
			}, eventuallyTimeoutShort).Should(gomega.BeTrue())
		})

		ginkgo.By(fmt.Sprintf("Checking if managed cluster %s has the correct annotations", config.ClusterName), func() {
			gomega.Eventually(func() bool {
				managedClusterAnnotations, err := utils.GetResourceAnnotations(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("managedClusterAnnotations: %v\n", managedClusterAnnotations)
				return managedClusterAnnotations["import.open-cluster-management.io/klusterlet-deploy-mode"] == "Hosted" &&
					managedClusterAnnotations["import.open-cluster-management.io/hosting-cluster-name"] == utils.LocalClusterName &&
					managedClusterAnnotations["open-cluster-management/created-via"] == "hypershift"
			}, eventuallyTimeoutShort).Should(gomega.BeTrue())
		})

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Printf("========================= End Test create hosted cluster %s ===============================", config.ClusterName)
	})
})
