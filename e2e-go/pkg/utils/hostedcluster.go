package utils

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	libgounstructuredv1 "github.com/stolostron/library-go/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var HostedClustersGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "hostedclusters",
}

// GetHostedClustersList gets the list of HostedClusters of some type with some label selector
// If type and label selector are not provided, it returns all HostedClusters
func GetHostedClustersList(hubClientDynamic dynamic.Interface, hostedClusterType string, labelSelector string) ([]*unstructured.Unstructured, error) {
	hostedClusterList, err := ListResource(hubClientDynamic, HostedClustersGVR, "", labelSelector)
	if err != nil {
		return nil, err
	}

	if hostedClusterType != "" {
		finalHCList := []*unstructured.Unstructured{}
		for _, hostedCluster := range hostedClusterList {
			// filter by spec.platform.type
			if hostedCluster.Object["spec"].(map[string]interface{})["platform"].(map[string]interface{})["type"] == hostedClusterType {
				finalHCList = append(finalHCList, hostedCluster)
			}
		}
		return finalHCList, nil
	}

	return hostedClusterList, nil
}

// GetAWSHostedClustersList gets the list of AWS HostedClusters of some type with some label selector
// If label selector are not provided, it returns all AWS HostedClusters
func GetAWSHostedClustersList(hubClientDynamic dynamic.Interface, labelSelector string) ([]*unstructured.Unstructured, error) {
	return GetHostedClustersList(hubClientDynamic, TYPE_AWS, labelSelector)
}

// GetKubevirtHostedClustersList gets the list of Kubevirt HostedClusters of some type with some label selector
// If label selector are not provided, it returns all Kubevirt HostedClusters
func GetKubevirtHostedClustersList(hubClientDynamic dynamic.Interface, labelSelector string) ([]*unstructured.Unstructured, error) {
	return GetHostedClustersList(hubClientDynamic, TYPE_KUBEVIRT, labelSelector)
}

// GetAgentHostedClustersList gets the list of Agent HostedClusters of some type with some label selector
// If label selector are not provided, it returns all Agent HostedClusters
func GetAgentHostedClustersList(hubClientDynamic dynamic.Interface, labelSelector string) ([]*unstructured.Unstructured, error) {
	return GetHostedClustersList(hubClientDynamic, TYPE_AGENT, labelSelector)
}

func CheckHCPAvailable(hubClientDynamic dynamic.Interface, clusterName string, namespace string) error {
	fmt.Printf("Cluster %s: Check %s hosted control plane is available...\n", clusterName, clusterName)
	hostedCluster, err := GetResource(hubClientDynamic, HostedClustersGVR, namespace, clusterName)
	if err != nil {
		return err
	}

	var condition map[string]interface{}
	condition, err = libgounstructuredv1.GetConditionByType(hostedCluster, "Available")
	if err != nil {
		return err
	}
	fmt.Printf("HostedCluster %s: Condition %#v\n", clusterName, condition)

	// Ensure the rest of the conditions have status set to true
	if v, ok := condition["status"]; ok && v == string(metav1.ConditionTrue) &&
		(condition["message"] == "The hosted control plane is available") &&
		(condition["reason"] == "AsExpected") {
		return nil
	} else {
		fmt.Printf("HostedCluster %s: Expected condition \"%s\" but got \"%v\"\n", clusterName, metav1.ConditionTrue, v)
		return generateErrorMsg(UnknownError, UnknownErrorLink,
			"HostedCluster control plane not available!",
			"HostedCluster control plane not available!")
	}
}

func WaitForHCPAvailable(hubClientDynamic dynamic.Interface, clusterName string, namespace string) {
	gomega.Eventually(func() error {
		return CheckHCPAvailable(hubClientDynamic, clusterName, namespace)
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
	fmt.Printf("HostedCluster %s: The hosted control plane is available\n\n", clusterName)
}

func WaitForHostedClusterDestroyed(hubClientDynamic dynamic.Interface, clusterName string) {
	gomega.Eventually(func() bool {
		hasManagedCluster, err := HasResource(hubClientDynamic, HostedClustersGVR, "", clusterName)
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		return hasManagedCluster
	}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeFalse())
	fmt.Printf("Hosted Cluster %s: successfully destroyed!\n\n", clusterName)
}
