// Package hypershift_test contains e2e tests for the Hypershift addon.
// This file tests control-plane-only upgrade via ClusterCurator (spec.upgrade.desiredUpdate + upgradeType: ControlPlane).
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
	labelControlPlaneUpgrade = "control-plane-upgrade"
)

// Control-plane-only upgrade: ClusterCurator upgrades only the HostedCluster control plane using
// spec.upgrade.desiredUpdate and spec.upgrade.upgradeType: ControlPlane.
var _ = ginkgo.Describe("Control-plane-only upgrade", ginkgo.Label("e2e", labelControlPlaneUpgrade, TYPE_AWS), func() {
	var (
		clusterName   string
		namespace     string
		testChannel   string
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

		testChannel = utils.GetClusterCuratorChannel()
		desiredUpdate = utils.GetClusterCuratorDesiredUpdate()
		upgradeType = utils.GetClusterCuratorUpgradeType()
	})

	ginkgo.It("Control-plane only: set spec.upgrade (channel, desiredUpdate, upgradeType ControlPlane) and desiredCuration upgrade, then verify curator condition and HostedCluster release", func() {
		ginkgo.By("Ensuring HostedCluster exists")
		_, err := utils.GetResource(dynamicClient, utils.HostedClustersGVR, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "HostedCluster %s/%s must exist for this test", namespace, clusterName)

		if upgradeType != "ControlPlane" {
			ginkgo.Skip("control-plane-upgrade test requires upgradeType=ControlPlane (set HCP_UPGRADE_TYPE=ControlPlane or options.clustercurator.upgradeType)")
		}
		if desiredUpdate == "" {
			ginkgo.Skip("control-plane-upgrade test requires desiredUpdate (set HCP_UPGRADE_DESIRED_UPDATE or options.clustercurator.desiredUpdate). The cluster-curator-controller panics with 'Version string empty' if desiredUpdate is missing.")
		}

		ginkgo.By("Creating or updating ClusterCurator (minimal, no Ansible Tower)")
		err = utils.CreateOrUpdateClusterCuratorForChannelUpgrade(clientClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Setting spec.upgrade.channel")
		err = utils.SetClusterCuratorUpgradeChannel(dynamicClient, clusterName, namespace, testChannel)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Setting spec.upgrade.desiredUpdate and spec.upgrade.upgradeType")
		err = utils.SetClusterCuratorUpgradeDesiredUpdateAndType(dynamicClient, clusterName, namespace, desiredUpdate, upgradeType)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Setting desiredCuration to upgrade")
		err = utils.SetDesiredCuration(dynamicClient, clusterName, namespace, "upgrade")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Waiting for ClusterCurator clustercurator-job condition to become True (upgrade completed)")
		timeout := 20 * time.Minute
		interval := 15 * time.Second
		gomega.Eventually(func() error {
			return utils.CheckCuratorCondition(dynamicClient, clusterName, namespace,
				"clustercurator-job", string(metav1.ConditionTrue), "", "Job_has_finished")
		}, timeout, interval).ShouldNot(gomega.HaveOccurred())

		ginkgo.By("Verifying HostedCluster spec.release reflects desired version")
		release, err := utils.GetHostedClusterSpecRelease(dynamicClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(release).To(gomega.ContainSubstring(desiredUpdate),
			"HostedCluster spec.release should contain version %q, got %q", desiredUpdate, release)
		fmt.Printf("HostedCluster %s release image is %s\n", clusterName, release)
	})
})
