package hypershift_test

import (
	"context"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

// const (
// 	secretName = "hypershift-operator-oidc-provider-s3-credentials"
// 	namespace  = "open-cluster-management-agent-addon"
// )

var _ = ginkgo.Describe("RHACM4K-21843: Hypershift: Hypershift Addon should detect changes in S3 secret and re-install the hypershift operator", ginkgo.Label("e2e", "@non-ui", "RHACM4K-21843", TYPE_AWS), func() {
	var (
		secretName = "hypershift-operator-oidc-provider-s3-credentials123"
		namespace  = "local-cluster"
		namespace2 = "open-cluster-management-agent-addon"
		keyToFind  = "region"
		newKey     = "test"
		newValue   = "12312132123===="
	)

	ginkgo.It("Get, modify, and verify the s3 secret", func() {

		ginkgo.By("Step 1: Get the list of Pods before updating the secret", func() {
			utils.GetPodsInfoList(kubeClient, namespace2)
		})
		ginkgo.By("Step 2: Update the s3 secret", func() {
			// Update secret
			utils.UpdateSecret(context.TODO(), kubeClient, namespace, secretName, keyToFind, newKey, newValue)
			// Get the list of pods after the update]
			utils.GetPodsInfoList(kubeClient, namespace2)
		})
	})
})
