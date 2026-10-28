package utils

import "time"

const (
	KubeConfigFileEnv           = "KUBECONFIG"
	HypershiftOperatorNamespace = "hypershift"
	HyperShiftDNSOperatorName   = "external-dns"
	HypershiftOperatorName      = "operator"
	LocalClusterName            = "local-cluster"
	HypershiftAddonName         = "hypershift-addon"
	HypershiftAddonMgrName      = "hypershift-addon-manager"
	HypershiftCLIName           = "hcp"
	HypershiftS3OIDCSecretName  = "hypershift-operator-oidc-provider-s3-credentials"
	ExternalDNSSecretName       = "hypershift-operator-external-dns-credentials"
	HCPCliDownloadName          = "hcp-cli-download"
	UnknownError                = "[unknown error]"
	UnknownErrorLink            = "https://github.com/stolostron/cluster-lifecycle-e2e/blob/main/doc/e2eFailedAnalysis.md#unknown-error"
	eventuallyTimeout           = 45 * time.Minute
	eventuallyInterval          = 15 * time.Second
	TYPE_AWS                    = "AWS"
	TYPE_KUBEVIRT               = "KubeVirt"
	TYPE_AGENT                  = "Agent"

	// AddonNsConfigName is the name of the AddOnDeploymentConfig created by the hub discovery
	// controller to isolate the discovery-related add-ons (work-manager, managed-serviceaccount,
	// cluster-proxy) into their own agent install namespace when ACM manages an imported MCE
	// cluster. See ACM-48312 / ACM-48634.
	AddonNsConfigName = "addon-ns-config"
	// DiscoveryAgentInstallNamespace is the agent install namespace set on addon-ns-config so that
	// discovery add-ons do not collide with the standard ACM managed-cluster add-on namespace.
	DiscoveryAgentInstallNamespace = "open-cluster-management-agent-addon-discovery"
	// EnableSyncLabelsToClusterClaimsVar is the customizedVariables name used by work-manager to
	// decide whether it should sync ManagedCluster labels onto ClusterClaims. The hub discovery
	// controller sets this to "false" on addon-ns-config to avoid dual work-manager contention
	// overwriting ClusterClaim names/labels (ACM-48312).
	EnableSyncLabelsToClusterClaimsVar = "enableSyncLabelsToClusterClaims"
	// WorkManagerAddonName is the ClusterManagementAddOn name for the work-manager add-on.
	WorkManagerAddonName = "work-manager"
	// ManagedServiceAccountAddonName is the ClusterManagementAddOn name for the managed-serviceaccount add-on.
	ManagedServiceAccountAddonName = "managed-serviceaccount"
	// ClusterProxyAddonName is the ClusterManagementAddOn name for the cluster-proxy add-on.
	ClusterProxyAddonName = "cluster-proxy"
	// HypershiftAddonDeployConfigName is the AddOnDeploymentConfig that carries the
	// configureMceImport customizedVariable used to enable the MCE import/discovery flow.
	HypershiftAddonDeployConfigName = "hypershift-addon-deploy-config"
	// ConfigureMceImportVar is the customizedVariables name on hypershift-addon-deploy-config that
	// enables MCE cluster import/discovery.
	ConfigureMceImportVar = "configureMceImport"
)
