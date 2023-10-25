package utils

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gexec"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var InfrastructuresGVR = schema.GroupVersionResource{
	Group:    "config.openshift.io",
	Version:  "v1",
	Resource: "infrastructures",
}

func getKubeConfigFile() (string, error) {
	kubeConfigFile := os.Getenv(KubeConfigFileEnv)
	if kubeConfigFile == "" {
		fmt.Printf("Environment variable %s is not set, use default kubeconfig file\n", KubeConfigFileEnv)
		user, err := user.Current()
		if err != nil {
			return "", err
		}
		kubeConfigFile = path.Join(user.HomeDir, ".kube", "config")
	}

	return kubeConfigFile, nil
}

func NewKubeClient() (kubernetes.Interface, error) {
	kubeConfigFile, err := getKubeConfigFile()
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cfg)
}

func NewDynamicClient() (dynamic.Interface, error) {
	kubeConfigFile, err := getKubeConfigFile()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Use kubeconfig file: %s\n", kubeConfigFile)

	clusterCfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(clusterCfg)
	if err != nil {
		return nil, err
	}

	return dynamicClient, nil
}

func NewKubeConfig() (*rest.Config, error) {
	kubeConfigFile, err := getKubeConfigFile()
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func NewRouteV1Client() (routeclient.Interface, error) {
	kubeConfigFile, err := getKubeConfigFile()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Use kubeconfig file: %s\n", kubeConfigFile)

	clusterCfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, err
	}

	routeClient, err := routeclient.NewForConfig(clusterCfg)
	if err != nil {
		return nil, err
	}

	return routeClient, nil
}

func HasResource(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (bool, error) {
	var err error
	if namespace == "" {
		_, err = dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	}
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetResource returns the resource instance for the given GroupVersionResource, namespace, and name
func GetResource(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (
	*unstructured.Unstructured, error) {
	var obj *unstructured.Unstructured
	var err error
	if namespace == "" {
		obj, err = dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		obj, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// GetDynamicResource returns the first resource instance found for the given GroupVersionResource
// Does not require knowing the name or namespace of the resource
func GetDynamicResource(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	resourceList, err := dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Check if any resource instances were found
	if len(resourceList.Items) == 0 {
		return nil, errors.NewNotFound(gvr.GroupResource(), "")
	}

	// Return the single resource instance
	return &resourceList.Items[0], nil
}

func ListResource(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, labelSelector string) ([]*unstructured.Unstructured, error) {
	listOptions := metav1.ListOptions{}
	if labelSelector != "" {
		listOptions.LabelSelector = labelSelector
	}

	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = dynamicClient.Resource(gvr).List(context.TODO(), listOptions)
	} else {
		list, err = dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), listOptions)
	}

	if err != nil {
		return nil, err
	}

	resources := make([]*unstructured.Unstructured, 0)
	for _, item := range list.Items {
		resources = append(resources, item.DeepCopy())
	}

	return resources, nil
}

func GetResourceLabels(client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (map[string]string, error) {

	resource, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	fmt.Printf("Resource labels map for cluster %s: %v\n", name, resource.GetLabels())
	return resource.GetLabels(), nil
}

func GetResourceAnnotations(client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (map[string]string, error) {
	resource, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	fmt.Printf("Resource annotations map for cluster %s: %v\n", name, resource.GetLabels())
	return resource.GetAnnotations(), nil
}

/**
 * This is a helper function to print the output of a command
 * @param sess: the gexec.Session object
 */
func PrintOutput(sess *gexec.Session) {
	r := bufio.NewReader(sess.Buffer())
	for {
		text, err := r.ReadString('\n')
		fmt.Printf("Running cmd %s \n\n%s Output:\n%s", sess.Command.String(), sess.Command.String(), text)
		if err == io.EOF {
			break
		}
	}
}

func CreateOIDCProviderSecret(ctx context.Context, client kubernetes.Interface, bucketName string, awsKey AWSAPIKey, region string, namespace string) error {
	ginkgo.By(fmt.Sprintf("Creating hosted control plane OIDC AWS S3 Bucket provider secret for %s", namespace))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: HypershiftS3OIDCSecretName,
		},
		Data: map[string][]byte{
			"bucket": []byte(bucketName),
			"credentials": []byte(`[default]
			aws_access_key_id     = ` + awsKey.AWSAccessKeyID + `
			aws_secret_access_key = ` + awsKey.AWSAccessSecret),
			"region": []byte(region),
		},
	}

	_, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

/*
- This functions return a specific secret in a namesapce and verfies it exists
*/
func GetSecretInNamespace(client kubernetes.Interface, namespace string, secretName string) (*corev1.Secret, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error getting secret: %v", err)
	}
	fmt.Printf("Secret %s found in namespace %s\n", secretName, namespace)
	return secret, err
}

func UpdateSecret(ctx context.Context, client kubernetes.Interface, namespace string, secretName string, key string, newKey string, newKeyValue string) error {
	secret, err := GetSecretInNamespace(client, namespace, secretName)
	if err != nil {
		return err
	}
	// Check if the key exists in the secret
	if _, exists := secret.Data[key]; !exists {
		return fmt.Errorf("Key '%s' does not exist in the secret", key)
	}
	// Add a new key-value pair right after the existing key
	updatedData := make(map[string][]byte)
	for k, v := range secret.Data {
		updatedData[k] = v
		if k == key {
			updatedData[newKey] = []byte(newKeyValue)
		}
	}
	// Update the secret data
	secret.Data = updatedData

	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Error updating secret: %v", err)
	}

	return nil
}

func GetResourceDecodedSecretValue(client kubernetes.Interface, namespace, secretName, secretKey string, base64Decode bool) (string, error) {
	// secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	// if err != nil {
	// 	fmt.Printf("Error fetching secret: %v\n", err)
	// 	return "", err
	// }
	// fmt.Printf("Secret %s found in namespace %s\n", secretName, namespace)
	secret, err := GetSecretInNamespace(client, namespace, secretName)

	encodedValue, exists := secret.Data[secretKey]
	if !exists {
		fmt.Printf("Key %s not found in secret\n", secretKey)
		return "", err
	}
	fmt.Printf("Key %s found in secret\n", secretKey)
	fmt.Printf("Encoded value: %s\n", string(encodedValue))

	if base64Decode {
		decodedValue, err := base64.StdEncoding.DecodeString(string(encodedValue))
		if err != nil {
			fmt.Printf("Error decoding value: %v\n", err)
			return "", err
		}
		return string(decodedValue), nil
	}

	return string(encodedValue), nil
}

func DeleteOIDCProviderSecret(ctx context.Context, client kubernetes.Interface, namespace string) error {
	ginkgo.By(fmt.Sprintf("Deleting the hypershift OIDC provider secret for %s", namespace))
	return client.CoreV1().Secrets(namespace).Delete(ctx, HypershiftS3OIDCSecretName, metav1.DeleteOptions{})
}

// GenerateClusterName generates a unique cluster name given a prefix string and adds a uuid to the end
func GenerateClusterName(prefix string) (string, error) {
	if prefix == "" {
		prefix = "acmqe-hc"
	}
	uuidObj := uuid.New()
	uuidString := uuidObj.String()
	lowerCaseUUID := strings.ReplaceAll(uuidString, "-", "")

	uniqueID := fmt.Sprintf("%s-%s", prefix, strings.ToLower(lowerCaseUUID))
	// requires to be shorter, as adding an external dns name could cause it to be over the 63 character limit for creating endpoint
	return uniqueID[:25], nil
}

func generateErrorMsg(tag, solution, reason, errmsg string) error {
	return fmt.Errorf("tag: %v, "+
		"Possible Solution: %v, "+
		"Reason: %v, "+
		"Error message: %v,",
		tag, solution, reason, errmsg)
}

/*
- This function returns the list of pods and their info info (Name, Ready, Status, Restarts, Age) in a specific namespace

	type PodInfo struct {
	Name     string
	Ready    bool
	Status   string
	Age      string
	Restarts int32
	}
*/
type PodInfo struct {
	Name     string
	Ready    bool
	Status   string
	Age      string
	Restarts int32
}

func getPodsInfoList(client kubernetes.Interface, namespace string) ([]PodInfo, error) {
	// Get pod list
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error getting pod list: %v", err)
	}
	// Extract pod details
	var podInfoList []PodInfo
	for _, pod := range pods.Items {
		podInfo := PodInfo{
			Name:     pod.Name,
			Ready:    pod.Status.ContainerStatuses[0].Ready,
			Status:   string(pod.Status.Phase),
			Restarts: pod.Status.ContainerStatuses[0].RestartCount,
			Age:      time.Since(pod.ObjectMeta.CreationTimestamp.Time).String(),
		}
		podInfoList = append(podInfoList, podInfo)
	}
	return podInfoList, nil
}
