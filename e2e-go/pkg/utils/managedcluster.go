package utils

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	libgounstructuredv1 "github.com/stolostron/library-go/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var ManagedClustersGVR = schema.GroupVersionResource{
	Group:    "cluster.open-cluster-management.io",
	Version:  "v1",
	Resource: "managedclusters",
}

var ManagedClusterAddonGVR = schema.GroupVersionResource{
	Group:    "addon.open-cluster-management.io",
	Version:  "v1alpha1",
	Resource: "managedclusteraddons",
}

var AcmManagedClusterAddOns = []string{
	"application-manager",
	"cert-policy-controller",
	"cluster-proxy",
	"iam-policy-controller",
	"cert-policy-controller",
	"governance-policy-framework",
	"search-collector",
	"work-manager",
}

var MceManagedClusterAddOns = []string{
	"cluster-proxy",
	"work-manager",
}

func CheckClusterImported(hubClientDynamic dynamic.Interface, clusterName string) error {
	fmt.Printf("Cluster %s: Check %s is imported...\n", clusterName, clusterName)
	managedCluster, err := hubClientDynamic.Resource(ManagedClustersGVR).Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var condition map[string]interface{}
	condition, err = libgounstructuredv1.GetConditionByType(managedCluster, "ManagedClusterConditionAvailable")
	if err != nil {
		return err
	}
	fmt.Printf("Cluster %s: Condition %#v\n", clusterName, condition)

	// Ensure the rest of the conditions have status set to true
	if v, ok := condition["status"]; ok && v == string(metav1.ConditionTrue) {
		return nil
	} else {
		fmt.Printf("Cluster %s: Current is not equal to \"%s\" but \"%v\"\n", clusterName, metav1.ConditionTrue, v)
		return generateErrorMsg(UnknownError, UnknownErrorLink, "Import cluster fail, Cluster status in unknown", "Import cluster fail, Cluster status in unknown")
	}
}

func WaitForClusterImported(hubClientDynamic dynamic.Interface, clusterName string) {
	gomega.Eventually(func() error {
		return CheckClusterImported(hubClientDynamic, clusterName)
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
	fmt.Printf("Cluster %s: successfully auto-imported!\n\n", clusterName)
}

func WaitForClusterDetached(hubClientDynamic dynamic.Interface, clusterName string) {
	gomega.Eventually(func() bool {
		hasManagedCluster, err := HasResource(hubClientDynamic, ManagedClustersGVR, "", clusterName)
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		return hasManagedCluster
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeFalse())
	fmt.Printf("Cluster %s: successfully detached!\n\n", clusterName)
}

// WaitForClusterAddonsAvailable waits for all cluster addons to be available, checking against specific add-ons depending on MCE or ACM
func WaitForClusterAddonsAvailable(hubClientDynamic dynamic.Interface, clusterName string) error {
	addonsToCheck := MceManagedClusterAddOns

	if hasACM, err := IsACMInstalled(hubClientDynamic); err != nil {
		return err
	} else if hasACM {
		addonsToCheck = AcmManagedClusterAddOns
	}

	for _, addonName := range addonsToCheck {
		gomega.Eventually(func() error {
			fmt.Printf("Cluster %s: Checking Add-On %s is available...\n", clusterName, addonName)
			return ValidateClusterAddOnAvailable(hubClientDynamic, clusterName, addonName)
		}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
	}

	fmt.Printf("Cluster %s: all add-ons are available!\n\n", clusterName)
	return nil
}

// WaitForAllClusterAddonsAvailable waits for all cluster addons to be available, without checking against specific add-ons
func WaitForAllClusterAddonsAvailable(hubClientDynamic dynamic.Interface, clusterName string) {
	managedClusterAddons, err := hubClientDynamic.Resource(ManagedClusterAddonGVR).Namespace(clusterName).List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(len(managedClusterAddons.Items) > 0).Should(gomega.BeTrue())

	for _, addon := range managedClusterAddons.Items {
		if _, ok := addon.Object["metadata"]; ok {
			addonName := addon.Object["metadata"].(map[string]interface{})["name"].(string)
			gomega.Eventually(func() error {
				fmt.Printf("Cluster %s: Checking Add-On %s is available...\n", clusterName, addonName)
				return ValidateClusterAddOnAvailable(hubClientDynamic, clusterName, addonName)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
		}
	}
	fmt.Printf("Cluster %s: all add-ons are available!\n\n", clusterName)
}

func ValidateClusterAddOnAvailable(dynamicClient dynamic.Interface, clusterName string, addOnName string) error {
	managedClusterAddon, err := dynamicClient.Resource(ManagedClusterAddonGVR).Namespace(clusterName).Get(context.TODO(), addOnName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var condition map[string]interface{}
	condition, err = libgounstructuredv1.GetConditionByType(managedClusterAddon, "Available")
	if err != nil {
		return err
	}
	if v, ok := condition["status"]; ok && v == string(metav1.ConditionTrue) {
		fmt.Printf("Cluster %s: Add-On %s is available! \n", clusterName, addOnName)
		return nil
	}
	err = fmt.Errorf("cluster %s - Add-On %s: status not found or not true", clusterName, addOnName)
	return err
}
