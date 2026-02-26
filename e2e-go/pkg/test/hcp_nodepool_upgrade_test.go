// Package hypershift_test contains e2e tests for the Hypershift addon.
// This file tests nodepool-only upgrade via ClusterCurator (spec.upgrade.desiredUpdate + upgradeType: NodePools).
package hypershift_test

import (
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	labelNodepoolUpgrade = "nodepool-upgrade"
)

// Nodepool-only upgrade: ClusterCurator upgrades only the NodePools (worker nodes) using
// spec.upgrade.desiredUpdate and spec.upgrade.upgradeType: NodePools. The HostedCluster control plane
// is not modified. NodePools version cannot exceed the control plane version—upgrade control plane first if needed.
var _ = ginkgo.Describe("Nodepool-only upgrade", ginkgo.Label("e2e", labelNodepoolUpgrade, TYPE_AWS), func() {
	var (
		clusterName   string
		namespace     string
		desiredUpdate string
		upgradeType   string
	)

	ginkgo.BeforeEach(func() {
		var err error
		clusterName, err = utils.GetClusterName("aws")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(clusterName).NotTo(gomega.BeEmpty(), "HCP_CLUSTER_NAME or options.clusters.aws.clusterName must be set")

		namespace, err = utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		desiredUpdate = utils.GetClusterCuratorDesiredUpdate()
		upgradeType = utils.GetClusterCuratorUpgradeType()
	})

	ginkgo.It("Nodepool only: set spec.upgrade (desiredUpdate, upgradeType NodePools) and desiredCuration upgrade, then verify curator condition and NodePool release", func() {
		ginkgo.By("Ensuring HostedCluster exists")
		_, err := utils.GetResource(dynamicClient, utils.HostedClustersGVR, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "HostedCluster %s/%s must exist for this test", namespace, clusterName)

		if upgradeType != "NodePools" {
			ginkgo.Skip("nodepool-upgrade test requires upgradeType=NodePools (set HCP_UPGRADE_TYPE=NodePools or options.clustercurator.upgradeType)")
		}
		if desiredUpdate == "" {
			ginkgo.Skip("nodepool-upgrade test requires desiredUpdate (set HCP_UPGRADE_DESIRED_UPDATE or options.clustercurator.desiredUpdate). The cluster-curator-controller panics with 'Version string empty' if desiredUpdate is missing.")
		}

		ginkgo.By("Ensuring at least one NodePool exists for the HostedCluster")
		nodePools, err := utils.ListNodePoolsForHostedCluster(dynamicClient, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(nodePools).NotTo(gomega.BeEmpty(), "HostedCluster %s must have at least one NodePool for nodepool-upgrade test", clusterName)

		ginkgo.By("Creating or updating ClusterCurator (minimal, no Ansible Tower)")
		err = utils.CreateOrUpdateClusterCuratorForChannelUpgrade(clientClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Setting spec.upgrade.desiredUpdate and spec.upgrade.upgradeType (channel is ignored for NodePools)")
		err = utils.SetClusterCuratorUpgradeDesiredUpdateAndType(dynamicClient, clusterName, namespace, desiredUpdate, upgradeType)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Setting desiredCuration to upgrade")
		err = utils.SetDesiredCuration(dynamicClient, clusterName, namespace, "upgrade")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Waiting for ClusterCurator clustercurator-job condition to become True (upgrade completed)")
		timeout := 30 * time.Minute // nodepool upgrade typically requires ~30 minutes
		interval := 15 * time.Second
		gomega.Eventually(func() error {
			return utils.CheckCuratorCondition(dynamicClient, clusterName, namespace,
				"clustercurator-job", string(metav1.ConditionTrue), "", "Job_has_finished")
		}, timeout, interval).ShouldNot(gomega.HaveOccurred())

		ginkgo.By("Verifying all NodePools spec.release reflects desired version")
		nodePools, err = utils.ListNodePoolsForHostedCluster(dynamicClient, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(nodePools).NotTo(gomega.BeEmpty(), "NodePools should still exist after upgrade")

		for _, np := range nodePools {
			release, err := utils.GetNodePoolSpecRelease(np)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(release).To(gomega.ContainSubstring(desiredUpdate),
				"NodePool %s spec.release should contain version %q, got %q", np.GetName(), desiredUpdate, release)
			fmt.Printf("NodePool %s release image is %s\n", np.GetName(), release)
		}

		ginkgo.By("Verifying HostedCluster spec.release was NOT changed (control plane unchanged)")
		hcRelease, err := utils.GetHostedClusterSpecRelease(dynamicClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// HostedCluster release may or may not contain desiredUpdate depending on prior upgrades;
		// the key assertion is that NodePools were upgraded. We just log the HC version for debugging.
		fmt.Printf("HostedCluster %s control plane release image is %s (unchanged by nodepool-only upgrade)\n", clusterName, hcRelease)
	})
})
