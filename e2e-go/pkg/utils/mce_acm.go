package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var MultiClusterHubGVR = schema.GroupVersionResource{
	Group:    "operator.open-cluster-management.io",
	Version:  "v1",
	Resource: "multiclusterhubs",
}

var MultiClusterEngineGVR = schema.GroupVersionResource{
	Group:    "multicluster.openshift.io",
	Version:  "v1",
	Resource: "multiclusterengines",
}

// check if the multiclusterhub (ACM) is installed
// We don't use HasResource since we don't know the ns in this case
func IsACMInstalled(dynamicClient dynamic.Interface) (bool, error) {
	_, err := GetDynamicResource(dynamicClient, MultiClusterHubGVR)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Println("ACM is not installed!")
			return false, nil
		} else {
			fmt.Printf("Error getting resource: %v\n", err)
			return false, err
		}
	}
	return true, nil
}

// check if the multiclusterengine (MCE) is installed
// We don't use HasResource since we don't know the ns in this case
func IsMCEInstalled(dynamicClient dynamic.Interface) (bool, error) {
	_, err := GetDynamicResource(dynamicClient, MultiClusterEngineGVR)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Println("MCE is not installed!")
			return false, nil
		} else {
			fmt.Printf("Error getting resource: %v\n", err)
			return false, err
		}
	}
	return true, nil
}

func GetMCENamespace(dynamicClient dynamic.Interface) (string, error) {
	mce, err := GetDynamicResource(dynamicClient, MultiClusterEngineGVR)
	if err != nil {
		return "", err
	}
	return mce.Object["spec"].(map[string]interface{})["targetNamespace"].(string), nil
}

func GetACMNamespace(dynamicClient dynamic.Interface) (string, error) {
	acm, err := GetDynamicResource(dynamicClient, MultiClusterHubGVR)
	if err != nil {
		return "", err
	}
	return acm.Object["spec"].(map[string]interface{})["targetNamespace"].(string), nil
}
