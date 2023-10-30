package hypershift_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

// const (
// 	secretName = "hypershift-operator-oidc-provider-s3-credentials"
// 	namespace  = "open-cluster-management-agent-addon"
// )

var _ = ginkgo.Describe("RHACM4K-21843: Hypershift: Hypershift Addon should detect changes in S3 secret and re-install the hypershift operator", ginkgo.Label("e2e", "@non-ui", "RHACM4K-21843", TYPE_AWS), func() {
	var (
		secretName       = "hypershift-operator-oidc-provider-s3-credentials"
		hcpInstallPrefix = "hypershift-install-job"
		namespace        = "local-cluster"
		namespace2       = "open-cluster-management-agent-addon"
		keyToFind        = "region"
		newKey           = "test"
		newValue         = "12312132123===="
		podNameBefore    string
		podNameAfter     string
	)

	ginkgo.It("Get, modify, and verify the s3 secret", func() {
		ginkgo.By("Step 1: Get the latest hypershift install Pod BEFORE updating the secret", func() {
			podBefore, err := utils.GetLastCreatedPod(kubeClient, namespace2)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			podNameBefore = podBefore.ObjectMeta.Name
			podCreationTime := podBefore.ObjectMeta.CreationTimestamp.Time
			fmt.Printf("BEFORE --> Pod %s found in namespace %s created at %s \n", podNameBefore, namespace2, podCreationTime)
		})
		ginkgo.By("Step 2: Update the s3 secret by injecting a new key to it", func() {
			utils.UpdateSecret(context.TODO(), kubeClient, namespace, secretName, keyToFind, newKey, newValue)
		})
		ginkgo.By("Step 3: Get the latest hypershift isntall Pod AFTER updating the secret", func() {
			// Set a timeout of 5 minutes
			timeout := 5 * time.Minute

			startTime := time.Now()
			// Continuously check for a new hypershift-install-pod for 5 minutes
			for {
				// Check if the 5 minutes have passed
				if time.Since(startTime) >= timeout {
					ginkgo.Fail(fmt.Sprintf("Timeout reached while waiting for the operation to succeed : % \nv", err))
					break
				}
				podAfter, err := utils.GetLastCreatedPod(kubeClient, namespace2)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				podNameAfter = podAfter.ObjectMeta.Name
				podCreationTime := podAfter.ObjectMeta.CreationTimestamp.Time
				fmt.Printf("AFTER --> Pod %s found in namespace %s created at %s \n", podNameAfter, namespace2, podCreationTime)
				if podNameAfter != podNameBefore {
					if strings.HasPrefix(podNameAfter, hcpInstallPrefix) {
						break
					}
				}

				// Sleep for a short duration before checking again
				time.Sleep(2 * time.Second)
			}
		})
		ginkgo.By("Step 4: Verify that a new hypershift install job is running (podNameAfter should be different podNameBefore)", func() {
			gomega.Ω(podNameAfter).ShouldNot(gomega.Equal(podNameBefore))
		})

		// ginkgo.By("Step 5: Verify that all pods are running, and timeout after 5 minutes", func() {
		// 	utils.VerifiesAllPodsAreRunning(kubeClient, namespace2, 5)
		// })
	})
})
