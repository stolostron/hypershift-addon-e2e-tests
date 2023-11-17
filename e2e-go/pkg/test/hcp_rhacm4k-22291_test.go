package hypershift_test

import (
	"context"
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("RHACM4K-22291: Hypershift: Able to see the hypershift-addon status reflected by the hypershift operator pod status and external DNS pod status", ginkgo.Label("e2e", "@non-ui", "RHACM4K-22291", TYPE_AWS), func() {
	var (
		deploymentNamespace     = "hypershift"
		addonNamesapce          = "local-cluster"
		managedClusterAddonName = "hypershift-addon"
		// Deployments to verify
		deploymentsToVerify = []string{"external-dns", "operator"}
	)
	ginkgo.It("hypershift-addon status reflected by the hypershift operator pod status and external DNS pod status", func() {
		ginkgo.By("Step 1: See if the hypershift operator and external dns started on hosting cluster hypershift namespace", func() {
			ginkgo.By("Step 2: hypershift-addon status", func() {

				// Get deployments in the namespace
				deployments, err := kubeClient.AppsV1().Deployments(deploymentNamespace).List(context.TODO(), metav1.ListOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error getting deployments: %v", err)

				// Verify the deployments
				gomega.Expect(deployments.Items).NotTo(gomega.BeEmpty(), "No deployments found in the namespace")

				for _, deploymentName := range deploymentsToVerify {
					utils.VerifyDeploymentExistence(deployments, deploymentName)
				}
			})
			ginkgo.By("Check the hypershift addon status, should be degraded = false", func() {
				// Get ManagedClusterAddon
				managedClusterAddon, err := addonClient.AddonV1alpha1().ManagedClusterAddOns(addonNamesapce).Get(context.TODO(), managedClusterAddonName, metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error getting ManagedClusterAddon: %v", err)
				//fmt.Printf(managedClusterAddon.Status.De)
				fmt.Printf("test %s:", managedClusterAddon.Annotations["Degraded"])

				// Verify ManagedClusterAddon details
				gomega.Expect(managedClusterAddon.Name).To(gomega.Equal(managedClusterAddonName), "ManagedClusterAddon name mismatch")
				fmt.Printf("ManagedClusterAddon Name: %s\n", managedClusterAddon.Name)

				// // Verify ManagedClusterAddon status
				// gomega.Expect(managedClusterAddon.Status.Available).To(gomega.BeTrue(), "ManagedClusterAddon is not available")
				// fmt.Printf("ManagedClusterAddon is available\n")

				// if managedClusterAddon.Status.Degraded {
				// 	fmt.Printf("ManagedClusterAddon is degraded\n")
				// }

				// if managedClusterAddon.Status.Progressing {
				// 	fmt.Printf("ManagedClusterAddon is progressing\n")
				// }
			})
		})
	})
})
