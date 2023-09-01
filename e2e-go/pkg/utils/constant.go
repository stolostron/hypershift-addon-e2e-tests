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
	eventuallyTimeout           = 15 * time.Minute
	eventuallyInterval          = 5 * time.Second
)
