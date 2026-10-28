package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

var AddOnDeploymentConfigGVR = schema.GroupVersionResource{
	Group:    "addon.open-cluster-management.io",
	Version:  "v1alpha1",
	Resource: "addondeploymentconfigs",
}

var ClusterManagementAddOnGVR = schema.GroupVersionResource{
	Group:    "addon.open-cluster-management.io",
	Version:  "v1alpha1",
	Resource: "clustermanagementaddons",
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

// GetAddOnDeploymentConfigAgentInstallNamespace returns spec.agentInstallNamespace for the given
// AddOnDeploymentConfig object.
func GetAddOnDeploymentConfigAgentInstallNamespace(cfg *unstructured.Unstructured) (string, bool, error) {
	return unstructured.NestedString(cfg.Object, "spec", "agentInstallNamespace")
}

// GetAddOnDeploymentConfigCustomizedVariable returns the value of the named entry in
// spec.customizedVariables for the given AddOnDeploymentConfig object, e.g.
// "enableSyncLabelsToClusterClaims".
func GetAddOnDeploymentConfigCustomizedVariable(cfg *unstructured.Unstructured, varName string) (string, bool, error) {
	vars, found, err := unstructured.NestedSlice(cfg.Object, "spec", "customizedVariables")
	if err != nil || !found {
		return "", found, err
	}

	for _, v := range vars {
		variable, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if name, _, _ := unstructured.NestedString(variable, "name"); name == varName {
			value, valueFound, err := unstructured.NestedString(variable, "value")
			return value, valueFound, err
		}
	}

	return "", false, nil
}

// ClusterManagementAddOnReferencesConfig checks whether the given ClusterManagementAddOn's
// install strategy placements reference the given AddOnDeploymentConfig (by name/namespace) in
// at least one of their configs.
func ClusterManagementAddOnReferencesConfig(cma *unstructured.Unstructured, configNamespace, configName string) (bool, error) {
	placements, found, err := unstructured.NestedSlice(cma.Object, "spec", "installStrategy", "placements")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	for _, p := range placements {
		placement, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		configs, found, err := unstructured.NestedSlice(placement, "configs")
		if err != nil {
			return false, err
		}
		if !found {
			continue
		}
		for _, c := range configs {
			config, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			name, _, _ := unstructured.NestedString(config, "name")
			namespace, _, _ := unstructured.NestedString(config, "namespace")
			resource, _, _ := unstructured.NestedString(config, "resource")
			if name == configName && namespace == configNamespace && resource == "addondeploymentconfigs" {
				return true, nil
			}
		}
	}

	return false, nil
}
