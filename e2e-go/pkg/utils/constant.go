package utils

import "time"

const (
	KubeConfigFileEnv           = "KUBECONFIG"
	HypershiftOperatorNamespace = "hypershift"
	HypershiftOperatorName      = "operator"
	LocalClusterName            = "local-cluster"
	HypershiftAddonName         = "hypershift-addon"
	HypershiftAddonMgrName      = "hypershift-addon-manager"
	HypershiftCLIName           = "hypershift"
	HypershiftS3OIDCSecretName  = "hypershift-operator-oidc-provider-s3-credentials"
	UnknownError                = "[unknown error]"
	UnknownErrorLink            = "https://github.com/stolostron/cluster-lifecycle-e2e/blob/main/doc/e2eFailedAnalysis.md#unknown-error"
	eventuallyTimeout           = 15 * time.Minute
	eventuallyInterval          = 5 * time.Second
)
