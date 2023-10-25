package hypershift_test

import (
	"context"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

const (
	secretName = "hypershift-operator-oidc-provider-s3-credentials"
	namespace  = "open-cluster-management-agent-addon"
)

var _ = ginkgo.Describe("RHACM4K-21843: Hypershift: Hypershift Addon should detect changes in S3 secret and re-install the hypershift operator", ginkgo.Label("e2e", "@non-ui", "RHACM4K-21843", TYPE_AWS), func() {
	var (
		secretName = "my-secret"
		namespace  = "default"
		keyToFind  = "region"
		newKey     = "test"
		newValue   = "dXMtZWFzdC0x----"
	)

	ginkgo.It("Get, modify, and verify the s3 secret", func() {

		ginkgo.By("Step 1: Get the secret with oc command", func() {
			// Step 1: Get the secret with oc command
			// getSecretCmd := exec.Command("oc", "get", "secret", secretName, "-n", namespace, "-o", "yaml")
			// output, err := getSecretCmd.CombinedOutput()
			// secretOutput := string(output)
			// gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// secretOutput := secret.Data
			// // Step 3: Find the position of the key and get its value
			// keyPosition := strings.Index(secretOutput, `"`+keyToFind+`":`) + len(`"`+keyToFind+`":`)
			// valueStart := keyPosition + 1 // Start of the value after the colon
			// valueEnd := strings.Index(secretOutput[valueStart:], "\"") + valueStart
			// oldValue := secretOutput[valueStart:valueEnd]

			// // Step 4: Check if the key-value pair exists in the secret output
			// gomega.Expect(strings.Contains(secretOutput, `"`+keyToFind+`":"`+oldValue+`"`)).To(gomega.BeTrue())

			// // Step 5: Append a new key-value pair
			// modifiedOutput := secretOutput[:valueEnd+1] + `"` + newKey + `":"` + newValue + `",` + secretOutput[valueEnd+1:]

			// // Step 6: Apply the modified secret
			// applySecretCmd := exec.Command("oc", "apply", "-n", namespace, "-f", "-")
			// applySecretCmd.Stdin = strings.NewReader(modifiedOutput)
			// err = applySecretCmd.Run()
			// gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// // Step 7: Verify the secret was updated
			// getUpdatedSecretCmd := exec.Command("oc", "get", "secret", secretName, "-n", namespace, "-o", "yaml")
			// updatedOutput, err := getUpdatedSecretCmd.CombinedOutput()
			// gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// // Verify if the modified data is present in the updated secret
			// gomega.Expect(strings.Contains(string(updatedOutput), `"`+newKey+`":"`+newValue+`"`)).To(gomega.BeTrue())

			// Step 2: Get the secret
			// secret, err := utils.GetSecretInNamespace(kubeClient, namespace, secretName)
			// Update secret
			utils.UpdateSecret(context.TODO(), kubeClient, namespace, secretName, keyToFind, newKey, newValue)
		})
	})
})
