package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stolostron/applier/pkg/applier"           // old (V1.0.1) version
	"github.com/stolostron/applier/pkg/templateprocessor" // old (V1.0.1) version
	libgounstructuredv1 "github.com/stolostron/library-go/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const CLUSTER_CURATOR_TEST_FIXTURE_DIR = "../resources/clustercurator"

var ClusterCuratorGVR = schema.GroupVersionResource{
	Group:    "cluster.open-cluster-management.io",
	Version:  "v1beta1",
	Resource: "clustercurators",
}

var AnsibleJobGVR = schema.GroupVersionResource{
	Group:    "tower.ansible.com",
	Version:  "v1alpha1",
	Resource: "ansiblejobs",
}

func CreateOrUpdateAnsibleTowerSecret(clientClient client.Client, ansSecretName, ansSecretNs, ansHost, ansToken string) error {
	var err error
	if ansSecretName == "" || ansSecretNs == "" {
		return fmt.Errorf("ERROR: ansible tower secret name and namespace must be provided")
	}

	if ansHost == "" || ansToken == "" {
		// If host/token are not provided, then default to getting
		// the ansible tower host and token from env vars/options file, if not set/empty then return error
		ansHost, err = GetTowerHost()
		if err != nil {
			return fmt.Errorf("ERROR: ansible tower host must be provided or set via environment variables")
		}

		ansToken, err = GetTowerToken()
		if err != nil {
			return fmt.Errorf("ERROR: ansible tower token must be provided or set via environment variables")
		}
	}

	createYamlReader := templateprocessor.NewYamlFileReader(filepath.Join(CLUSTER_CURATOR_TEST_FIXTURE_DIR, "ansible_tower_secret.yaml"))
	values := struct {
		AnsibleSecretName      string
		AnsibleSecretNamespace string
		AnsibleTowerHost       string
		AnsibleTowerToken      string
	}{
		AnsibleSecretName:      ansSecretName,
		AnsibleSecretNamespace: ansSecretNs,
		AnsibleTowerHost:       ansHost,
		AnsibleTowerToken:      ansToken,
	}

	testApplier, err := applier.NewApplier(createYamlReader, &templateprocessor.Options{}, clientClient, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to be created: %v", err)
	}

	err = testApplier.CreateOrUpdateInPath("", nil, true, values)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to create or update cluster curator CR: %v", err)
	}

	return nil
}

func CreateOrUpdateClusterCurator(clientClient client.Client, hcName, hcNamespace, desiredCuration, hcPlatform, ansTowerSecret string) error {
	if hcName == "" || hcNamespace == "" || hcPlatform == "" || ansTowerSecret == "" {
		return fmt.Errorf("ERROR: cluster curator name, namespace, platform, and tower secret must be provided")
	}
	// desiredCuration may be empty when creating a curator to then patch (e.g. channel-upgrade test)

	createYamlReader := templateprocessor.NewYamlFileReader(filepath.Join(CLUSTER_CURATOR_TEST_FIXTURE_DIR, "cluster_curator.yaml"))
	values := struct {
		ClusterName        string
		ClusterNamespace   string
		DesiredCuration    string
		ClusterPlatform    string
		AnsibleTowerSecret string
	}{
		ClusterName:        hcName,
		ClusterNamespace:   hcNamespace,
		DesiredCuration:    desiredCuration,
		ClusterPlatform:    hcPlatform,
		AnsibleTowerSecret: ansTowerSecret,
	}

	testApplier, err := applier.NewApplier(createYamlReader, &templateprocessor.Options{}, clientClient, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to be created: %v", err)
	}

	err = testApplier.CreateOrUpdateInPath("", nil, true, values)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to create or update cluster curator CR: %v", err)
	}

	return nil
}

// CreateOrUpdateClusterCuratorForChannelUpgrade creates a minimal ClusterCurator for channel-upgrade only (PR 511).
// No Ansible Tower secret or hooks; use with SetClusterCuratorUpgradeChannel and SetDesiredCuration("upgrade").
func CreateOrUpdateClusterCuratorForChannelUpgrade(clientClient client.Client, hcName, hcNamespace string) error {
	if hcName == "" || hcNamespace == "" {
		return fmt.Errorf("ERROR: cluster curator name and namespace must be provided")
	}
	createYamlReader := templateprocessor.NewYamlFileReader(filepath.Join(CLUSTER_CURATOR_TEST_FIXTURE_DIR, "cluster_curator_channel_upgrade.yaml"))
	values := struct {
		ClusterName      string
		ClusterNamespace string
	}{
		ClusterName:      hcName,
		ClusterNamespace: hcNamespace,
	}
	testApplier, err := applier.NewApplier(createYamlReader, &templateprocessor.Options{}, clientClient, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to be created: %v", err)
	}
	err = testApplier.CreateOrUpdateInPath("", nil, true, values)
	if err != nil {
		return fmt.Errorf("ERROR Applier failed to create or update cluster curator CR: %v", err)
	}
	return nil
}

func SetDesiredCuration(hubClientDynamic dynamic.Interface, curatorName, namespace, desiredCuration string) error {
	fmt.Printf("ClusterCurator %s: Patching clustercurator in the namespace %s with spec.desiredCuration: %s\n", curatorName, namespace, desiredCuration)
	payload := fmt.Sprintf(`{"spec": {"desiredCuration": "%s"}}`, desiredCuration)
	_, err := hubClientDynamic.Resource(ClusterCuratorGVR).Namespace(namespace).Patch(context.TODO(), curatorName, types.MergePatchType, []byte(payload), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("ERROR Failed to patch clustercurator: %v", err)
	}

	return nil
}

// SetClusterCuratorUpgradeChannel patches the ClusterCurator with spec.upgrade.channel (PR 511 / ACM-26476).
// Use with desiredCuration "upgrade" to trigger a channel-only update on the HostedCluster.
func SetClusterCuratorUpgradeChannel(hubClientDynamic dynamic.Interface, curatorName, namespace, channel string) error {
	if channel == "" {
		return fmt.Errorf("channel must be non-empty")
	}
	fmt.Printf("ClusterCurator %s: Patching spec.upgrade.channel to %s in namespace %s\n", curatorName, channel, namespace)
	payload := fmt.Sprintf(`{"spec": {"upgrade": {"channel": "%s"}}}`, channel)
	_, err := hubClientDynamic.Resource(ClusterCuratorGVR).Namespace(namespace).Patch(context.TODO(), curatorName, types.MergePatchType, []byte(payload), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("ERROR Failed to patch clustercurator upgrade channel: %v", err)
	}
	return nil
}

func DeleteClusterCurator(hubClientDynamic dynamic.Interface, curatorName, namespace string) error {
	fmt.Printf("ClusterCurator %s: Deleting clustercurator in the namespace %s\n", curatorName, namespace)
	_, err := GetResource(hubClientDynamic, ClusterCuratorGVR, namespace, curatorName)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("ClusterCurator %s: ClusterCurator CR does not exist\n", curatorName)
			return nil
		} else {
			return fmt.Errorf("ERROR failed to get the cluster curator CR: %v", err)
		}
	} else {
		deleteErr := hubClientDynamic.Resource(ClusterCuratorGVR).Namespace(namespace).Delete(context.TODO(), curatorName, metav1.DeleteOptions{})
		if deleteErr != nil {
			return fmt.Errorf("ERROR failed to delete the cluster curator CR: %v", deleteErr)
		}
	}
	return nil
}

func CheckCuratorCondition(hubClientDynamic dynamic.Interface, curatorName, namespace, conType, expectedStatus, expectedMsg, expectedReason string) error {
	fmt.Printf("ClusterCurator %s: Checking clustercurator condition %s for the cluster %s\n", curatorName, conType, curatorName)
	clusterCurator, err := GetResource(hubClientDynamic, ClusterCuratorGVR, namespace, curatorName)
	if err != nil {
		return fmt.Errorf("ERROR failed to get the cluster curator CR: %v", err)
	}

	var condition map[string]interface{}
	condition, err = libgounstructuredv1.GetConditionByType(clusterCurator, conType)
	if err != nil {
		return fmt.Errorf("ERROR failed to get the cluster curator condition by type %s: %v", conType, err)
	}
	fmt.Printf("ClusterCurator %s: Condition %#v\n", curatorName, condition)

	// Ensure the rest of the conditions have status set to expectedStatus
	if v, ok := condition["status"]; ok && v == expectedStatus &&
		(condition["reason"] == expectedReason) &&
		(strings.Contains(condition["message"].(string), expectedMsg)) {
		return nil
	} else {
		fmt.Printf("ClusterCurator %s: Expected \"%s\" but got \"%v\"\n", curatorName, expectedStatus, v)
		return generateErrorMsg(UnknownError, UnknownErrorLink,
			"ClusterCurator "+conType+" condition not reached",
			"ClusterCurator "+conType+" condition not reached")
	}
}

func GetCurrentAnsibleJob(hubClientDynamic dynamic.Interface, curatorName, namespace string) (*unstructured.Unstructured, error) {
	fmt.Printf("ClusterCurator %s: Checking clustercurator condition %s for the cluster %s\n", curatorName, "current-ansiblejob", curatorName)
	clusterCurator, err := GetResource(hubClientDynamic, ClusterCuratorGVR, namespace, curatorName)
	if err != nil {
		return nil, fmt.Errorf("ERROR failed to get the cluster curator CR: %v", err)
	}

	// Gets the current ansiblejob from the ClusterCurator CRD
	// Condition looks something like this:
	// 	type: prehook-ansiblejob
	//   - lastTransitionTime: "2023-10-20T20:25:48Z"
	// 	message: prehookjob-2dkb9
	// 	reason: Job_has_finished
	// 	status: "False"
	// 	type: current-ansiblejob
	var condition map[string]interface{}
	condition, err = libgounstructuredv1.GetConditionByType(clusterCurator, "current-ansiblejob")
	if err != nil {
		return nil, fmt.Errorf("ERROR failed to get the cluster curator condition by type %s: %v", "current-ansiblejob", err)
	}
	fmt.Printf("ClusterCurator %s: Condition %#v\n", curatorName, "current-ansiblejob")

	// get resource AnsibleJobGVR with the name condition["message"]
	var ansibleJob *unstructured.Unstructured
	ansibleJob, err = GetResource(hubClientDynamic, AnsibleJobGVR, namespace, condition["message"].(string))
	if err != nil {
		return nil, fmt.Errorf("ERROR failed to get the ansiblejob: %v", err)
	}

	return ansibleJob, nil
}
