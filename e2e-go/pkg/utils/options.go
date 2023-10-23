package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	libgocmd "github.com/stolostron/library-e2e-go/pkg/cmd"
)

const (
	defaultOptionsFilePath = "../resources/options.yaml"
)

func InitVars() error {

	err := LoadOptions(libgocmd.End2End.OptionsFile)
	if err != nil {
		return fmt.Errorf("--options error: %v", err)
	}

	if TestOptions.Options.Hub.KubeConfig == "" {
		TestOptions.Options.Hub.KubeConfig = os.Getenv("KUBECONFIG")
	}

	return nil
}

type TestOptionsContainer struct {
	Options TestOptionsT `json:"options"`
}

// TestOptions ...
// Define options available for Tests to consume
type TestOptionsT struct {
	Hub             Hub             `json:"hub"`
	HostedCluster   Clusters        `json:"clusters"`
	CloudConnection CloudConnection `json:"credentials,omitempty"`
}

// Hub ...
// Define the shape of clusters
type Hub struct {
	Name         string `json:"clusterName,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	KubeContext  string `json:"kubecontext,omitempty"`
	ApiServerURL string `json:"apiServerURL,omitempty"`
	KubeConfig   string `json:"kubeconfig,omitempty"`
}

// Clusters ...
// Define the shape of clusters
type Clusters struct {
	AWS Cluster `json:"aws"`
}

// Cluster ...
// Define the shape of clusters that may be added under management
type Cluster struct {
	Name               string `json:"clusterName,omitempty"`
	InfraID            string `json:"infraID,omitempty"`
	Namespace          string `json:"namespace,omitempty"`
	ClusterSet         string `json:"clusterSet,omitempty"`
	BaseDomain         string `json:"baseDomain"`
	User               string `json:"user,omitempty"`
	Password           string `json:"password,omitempty"`
	KubeContext        string `json:"kubecontext,omitempty"`
	ApiServerURL       string `json:"apiServerURL,omitempty"`
	KubeConfig         string `json:"kubeconfig,omitempty"`
	Region             string `json:"region,omitempty"`
	ReleaseImage       string `json:"releaseImage,omitempty"`
	MasterInstanceType string `json:"masterInstanceType,omitempty"`
	WorkerInstanceType string `json:"workerInstanceType,omitempty"`
	ExternalNetwork    string `json:"extNetwork,omitempty"`
	APIFloatingIP      string `json:"apiFIP,omitempty"`
	IngressFloatingIP  string `json:"ingressFIP,omitempty"`
	MachineCIDR        string `json:"machineCIDR,omitempty"`
	NodePoolReplicas   string `json:"nodePoolReplicas,omitempty"`
	AWSCreds           string `json:"awsCreds,omitempty"`
	GenerateSSHKey     bool   `json:"generateSSH,omitempty"`
	InstanceType       string `json:"instanceType,omitempty"`
}

// CloudConnection struct for bits having to do with Connections
type CloudConnection struct {
	Secrets Secrets `json:"secrets,omitempty"`
	APIKeys APIKeys `json:"apiKeys,omitempty"`
}

// APIKeys - define the cloud connection information
type Secrets struct {
	PullSecret    string `json:"pullSecret"`
	SSHPrivateKey string `json:"sshPrivatekey"`
	SSHPublicKey  string `json:"sshPublickey"`
}

// APIKeys - define the cloud connection information
type APIKeys struct {
	S3           AWSAPIKey `json:"s3,omitempty"`
	AWS          AWSAPIKey `json:"aws,omitempty"`
	AWSCredsFile string    `json:"awsCredsFile,omitempty"`
	AWSCredsName string    `json:"awsCredName,omitempty"`
}

// AWSAPIKey ...
type AWSAPIKey struct {
	AWSAccessKeyID  string `json:"awsAccessKeyID"`
	AWSAccessSecret string `json:"awsSecretAccessKeyID"`
}

var TestOptions TestOptionsContainer

// LoadOptions load the options in the following priority:
// 1. The provided file path
// 2. The OPTIONS environment variable
// 3. Default "resources/options.yaml"
func LoadOptions(optionsFile string) error {
	if err := unmarshal(optionsFile); err != nil {
		return fmt.Errorf("--options error: %v", err)
	}
	return nil
}

func unmarshal(optionsFile string) error {

	if optionsFile == "" {
		optionsFile = os.Getenv("OPTIONS_FILE")
	}
	if optionsFile == "" {
		optionsFile = defaultOptionsFilePath
	}

	fmt.Printf("Attempting to load the options file %s...\n", optionsFile)

	data, err := os.ReadFile(filepath.Clean(optionsFile))
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal([]byte(data), &TestOptions); err != nil {
		return err
	}

	fmt.Printf("Succesfully loaded options from %s!\n", optionsFile)
	return nil

}

// GetOwner returns the owner in the following priority:
// 1. From command-line
// 2. From options.yaml
// 3. Using the $USER environment variable.
// 4. Default: ginkgo
func GetOwner() string {
	// owner is used to help identify who owns deployed resources
	//    If a value is not supplied, the default is OS environment variable $USER
	owner := libgocmd.End2End.Owner

	// if owner == "" {
	// 	owner = os.Getenv("USER")
	// }
	if owner == "" {
		owner = "ginkgo"
	}
	return owner
}

// GetFileName returns the files founded in the expected location
func GetFileName(path, prefix string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(path, prefix))
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetClusterName returns the ClusterName for the supported providers suppled in the options.yaml file
func GetClusterName(provider string) (string, error) {
	if os.Getenv("HCP_CLUSTER_NAME") != "" {
		return os.Getenv("HCP_CLUSTER_NAME"), nil
	}
	switch provider {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.Name, nil
	default:
		return "", fmt.Errorf("options provider %s is not supported", provider)
	}
}

// GetNamespace returns the namespace set in the env variable HCP_NAMESPACE or defaults to "hcp-ns"
func GetNamespace(provider string) (string, error) {
	if os.Getenv("HCP_NAMESPACE") != "" {
		return os.Getenv("HCP_NAMESPACE"), nil
	}
	return "hcp-ns", nil
}

// GetRegion returns the region for the supported cloud providers
func GetRegion(provider string) (string, error) {
	if os.Getenv("HCP_REGION") != "" {
		return os.Getenv("HCP_REGION"), nil
	}
	switch provider {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.Region, nil
	default:
		return "", fmt.Errorf("options provider %s is not supported", provider)

	}
}

// GetNodePoolReplicas returns the region for the supported cloud providers
func GetNodePoolReplicas(provider string) (string, error) {
	if os.Getenv("HCP_NODE_POOL_REPLICAS") != "" {
		return os.Getenv("HCP_NODE_POOL_REPLICAS"), nil
	}
	switch provider {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.NodePoolReplicas, nil
	default:
		return "", fmt.Errorf("options provider %s is not supported", provider)

	}
}

// GetBaseDomain returns the BaseDomain for the supported cloud providers
func GetBaseDomain(cloud string) (string, error) {
	if os.Getenv("HCP_BASE_DOMAIN_NAME") != "" {
		return os.Getenv("HCP_BASE_DOMAIN_NAME"), nil
	}
	switch cloud {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.BaseDomain, nil
	default:
		return "", fmt.Errorf("can not find the baseDomain as the cloud %s is unsupported", cloud)
	}
}

// GetReleaseImage returns the cluster image used to provision the cluster
func GetReleaseImage(cloud string) (string, error) {
	if os.Getenv("HCP_RELEASE_IMAGE") != "" {
		return os.Getenv("HCP_RELEASE_IMAGE"), nil
	}
	switch cloud {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.ReleaseImage, nil
	default:
		return "", fmt.Errorf("can not find the clusterimageset to provision cluster on %v", cloud)
	}
}

func GetInstanceType(cloud string) (string, error) {
	if os.Getenv("HCP_INSTANCE_TYPE") != "" {
		return os.Getenv("HCP_INSTANCE_TYPE"), nil
	}
	switch cloud {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.InstanceType, nil
	default:
		return "", fmt.Errorf("can not find the instance type to provision cluster on %v", cloud)
	}
}

func GetProviderCreds(cloud string) (string, error) {
	switch cloud {
	case "aws":
		return TestOptions.Options.HostedCluster.AWS.AWSCreds, nil
	default:
		return "", fmt.Errorf("can not find the clusterimageset to provision cluster on %v", cloud)
	}
}

func GetS3Creds() (AWSAPIKey, error) {
	if os.Getenv("S3_ACCESS_KEY_ID") != "" && os.Getenv("S3_ACCESS_SECRET") != "" {
		return AWSAPIKey{
			AWSAccessKeyID:  os.Getenv("S3_ACCESS_KEY_ID"),
			AWSAccessSecret: os.Getenv("S3_ACCESS_SECRET"),
		}, nil
	}
	return AWSAPIKey{
		AWSAccessKeyID:  TestOptions.Options.CloudConnection.APIKeys.S3.AWSAccessKeyID,
		AWSAccessSecret: TestOptions.Options.CloudConnection.APIKeys.S3.AWSAccessSecret,
	}, nil
}

func GetAWSCreds() (string, error) {
	if os.Getenv("AWS_CREDS") != "" {
		return os.Getenv("AWS_CREDS"), nil
	}
	return TestOptions.Options.CloudConnection.APIKeys.AWSCredsFile, nil
}

// GetPullSecret returns the cluster image used to provision the cluster
func GetPullSecret() (string, error) {
	if os.Getenv("PULL_SECRET_FILE") != "" {
		return os.Getenv("PULL_SECRET_FILE"), nil
	}
	return TestOptions.Options.CloudConnection.Secrets.PullSecret, nil
}

// GetAWSSecretCreds returns the cluster image used to provision the cluster
func GetAWSSecretCreds() (string, error) {
	if os.Getenv("SECRET_AWS_CRED_NAME") != "" {
		return os.Getenv("SECRET_AWS_CRED_NAME"), nil
	}
	return TestOptions.Options.CloudConnection.APIKeys.AWSCredsName, nil
}

// GetTowerHost returns the AAP Tower Host used for cluster curator auth
func GetTowerHost() (string, error) {
	if os.Getenv("AAP_HOST") != "" {
		return os.Getenv("AAP_HOST"), nil
	}
	return "", nil
}

// GetTowerToken returns the AAP Tower Token used for cluster curator auth
func GetTowerToken() (string, error) {
	if os.Getenv("AAP_TOKEN") != "" {
		return os.Getenv("AAP_TOKEN"), nil
	}
	return "", nil
}

// GetCuratorEnabled returns if AAP curator hooks tests should be enabled
func GetCuratorEnabled() (string, error) {
	if os.Getenv("CURATOR_ENABLED") != "" {
		return os.Getenv("CURATOR_ENABLED"), nil
	}
	return "true", nil
}
