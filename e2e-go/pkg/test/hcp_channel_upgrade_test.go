// Package hypershift_test contains e2e tests for the Hypershift addon.
// This file tests PR 511 (cluster-curator-controller): ACM-26476 HostedCluster channel setting.
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
	// Label for PR 511 / ACM-26476 channel upgrade tests (run with: --label-filter='channel-upgrade')
	labelChannelUpgrade = "channel-upgrade"
)

// PR 511 (cluster-curator-controller): ACM-26476 HostedCluster channel setting.
// Tests that ClusterCurator can update a HostedCluster's channel without a version upgrade,
// and that the controller validates channel against status.version.desired.channels.
var _ = ginkgo.Describe("PR 511 / ACM-26476: ClusterCurator HostedCluster channel update", ginkgo.Label("e2e", labelChannelUpgrade, "PR511", "ACM-26476", TYPE_AWS), func() {
	var (
		clusterName string
		namespace   string
		testChannel string
	)

	ginkgo.BeforeEach(func() {
		var err error
		clusterName, err = utils.GetClusterName("aws")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(clusterName).NotTo(gomega.BeEmpty(), "HCP_CLUSTER_NAME or options.clusters.aws.clusterName must be set")

		namespace, err = utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Channel from options.yaml (options.clustercurator.channel) or HCP_UPGRADE_CHANNEL env, else default
		testChannel = utils.GetClusterCuratorChannel()
	})

	ginkgo.It("Channel-only update: set spec.upgrade.channel and desiredCuration upgrade, then verify HostedCluster spec.channel and curator condition", func() {
		ginkgo.By("Ensuring HostedCluster exists")
		_, err := utils.GetResource(dynamicClient, utils.HostedClustersGVR, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "HostedCluster %s/%s must exist for this test", namespace, clusterName)

		ginkgo.By("Creating or updating ClusterCurator (channel-upgrade only, no Ansible Tower)")
		err = utils.CreateOrUpdateClusterCuratorForChannelUpgrade(clientClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = utils.SetClusterCuratorUpgradeChannel(dynamicClient, clusterName, namespace, testChannel)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = utils.SetDesiredCuration(dynamicClient, clusterName, namespace, "upgrade")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Waiting for ClusterCurator clustercurator-job condition to become True (upgrade completed)")
		timeout := 15 * time.Minute
		interval := 15 * time.Second
		// Controller sets clustercurator-job when the curator job finishes (message e.g. "curator-job-xxx DesiredCuration: upgrade Version (;channel;;;)")
		gomega.Eventually(func() error {
			return utils.CheckCuratorCondition(dynamicClient, clusterName, namespace,
				"clustercurator-job", string(metav1.ConditionTrue), "", "Job_has_finished")
		}, timeout, interval).ShouldNot(gomega.HaveOccurred())

		ginkgo.By("Verifying HostedCluster spec.channel was updated")
		channel, err := utils.GetHostedClusterChannel(dynamicClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(channel).To(gomega.Equal(testChannel),
			"HostedCluster spec.channel should be %q, got %q", testChannel, channel)
		fmt.Printf("HostedCluster %s channel is %s\n", clusterName, channel)
	})

	ginkgo.It("Available channels: HostedCluster status.version.desired.channels can be read for validation", func() {
		ginkgo.By("Ensuring HostedCluster exists")
		_, err := utils.GetResource(dynamicClient, utils.HostedClustersGVR, namespace, clusterName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "HostedCluster %s/%s must exist", namespace, clusterName)

		ginkgo.By("Reading HostedCluster available channels (used by PR 511 for validation)")
		channels, err := utils.GetHostedClusterAvailableChannels(dynamicClient, clusterName, namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		// Channels may be empty if cluster is still provisioning or channel not set yet
		fmt.Printf("HostedCluster %s available channels: %v\n", clusterName, channels)
	})
})
