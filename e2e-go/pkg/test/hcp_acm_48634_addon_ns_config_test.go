package hypershift_test

import (
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ACM-48634 verifies the fix for ACM-48312 (ClusterClaim name/custom labels overwritten when a
// cluster is simultaneously self-managed (MCE) and imported into ACM). The hub discovery
// controller must create/update the "addon-ns-config" AddOnDeploymentConfig in the MCE namespace
// so that:
//   - discovery add-ons (work-manager, managed-serviceaccount, cluster-proxy) are installed in
//     their own isolated agent install namespace (open-cluster-management-agent-addon-discovery)
//   - enableSyncLabelsToClusterClaims is set to "false" so work-manager does not sync
//     ManagedCluster labels onto ClusterClaims, avoiding dual work-manager contention with the
//     ACM-side work-manager instance.
//
// See https://github.com/stolostron/hypershift-addon-operator/pull/936 and
// https://github.com/stolostron/hypershift-addon-operator/blob/main/docs/management/discovering_hostedclusters.md
var _ = ginkgo.Describe("ACM-48634: addon-ns-config should isolate discovery add-ons and disable ClusterClaim label sync when ACM manages an imported MCE cluster", ginkgo.Label("e2e", "@non-ui", "ACM-48634"), func() {
	var addonNsConfig *unstructured.Unstructured

	ginkgo.BeforeEach(func() {
		hasACM, err := utils.IsACMInstalled(dynamicClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		if !hasACM {
			ginkgo.Skip("ACM is not installed on this hub; addon-ns-config is only created when ACM manages an imported MCE cluster")
		}

		configured, err := isMceImportConfigured(mceNamespace)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		if !configured {
			ginkgo.Skip(fmt.Sprintf("configureMceImport is not enabled on %s/%s; skipping addon-ns-config assertions", mceNamespace, utils.HypershiftAddonDeployConfigName))
		}

		// The hub discovery controller creates/updates addon-ns-config asynchronously after
		// configureMceImport is enabled; poll until spec content converges, not just until GET succeeds.
		gomega.Eventually(func() error {
			cfg, err := utils.GetResource(dynamicClient, utils.AddOnDeploymentConfigGVR, mceNamespace, utils.AddonNsConfigName)
			if err != nil {
				return err
			}
			if err := addonNsConfigReady(cfg); err != nil {
				return err
			}
			addonNsConfig = cfg
			return nil
		}, eventuallyTimeoutShort, eventuallyInterval).ShouldNot(gomega.HaveOccurred(),
			fmt.Sprintf("addon-ns-config AddOnDeploymentConfig must be fully configured in namespace %s when configureMceImport is enabled", mceNamespace))
	})

	ginkgo.It("sets agentInstallNamespace to the discovery namespace", func() {
		ginkgo.By(fmt.Sprintf("Check %s/%s has spec.agentInstallNamespace == %s", mceNamespace, utils.AddonNsConfigName, utils.DiscoveryAgentInstallNamespace), func() {
			ns, found, err := utils.GetAddOnDeploymentConfigAgentInstallNamespace(addonNsConfig)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(found).Should(gomega.BeTrue(), "spec.agentInstallNamespace should be set on addon-ns-config")
			gomega.Expect(ns).Should(gomega.Equal(utils.DiscoveryAgentInstallNamespace))
		})
	})

	ginkgo.It("disables ClusterClaim label sync via enableSyncLabelsToClusterClaims=false", func() {
		ginkgo.By(fmt.Sprintf("Check %s/%s customizedVariables has %s=false", mceNamespace, utils.AddonNsConfigName, utils.EnableSyncLabelsToClusterClaimsVar), func() {
			value, found, err := utils.GetAddOnDeploymentConfigCustomizedVariable(addonNsConfig, utils.EnableSyncLabelsToClusterClaimsVar)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(found).Should(gomega.BeTrue(), "customizedVariables should contain enableSyncLabelsToClusterClaims")
			gomega.Expect(value).Should(gomega.Equal("false"))
		})
	})

	ginkgo.It("is referenced by the work-manager, managed-serviceaccount and cluster-proxy ClusterManagementAddOn install strategies", func() {
		ginkgo.By("Wait for ClusterManagementAddOn install strategies to reference addon-ns-config", func() {
			gomega.Eventually(func() error {
				return clusterManagementAddOnsReferenceAddonNsConfig()
			}, eventuallyTimeoutShort, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})
	})
})

// addonNsConfigReady returns nil when addon-ns-config has the discovery namespace and label-sync disable set.
func addonNsConfigReady(cfg *unstructured.Unstructured) error {
	ns, found, err := utils.GetAddOnDeploymentConfigAgentInstallNamespace(cfg)
	if err != nil {
		return err
	}
	if !found || ns != utils.DiscoveryAgentInstallNamespace {
		return fmt.Errorf("agentInstallNamespace not ready: found=%v value=%q", found, ns)
	}

	value, found, err := utils.GetAddOnDeploymentConfigCustomizedVariable(cfg, utils.EnableSyncLabelsToClusterClaimsVar)
	if err != nil {
		return err
	}
	if !found || value != "false" {
		return fmt.Errorf("%s not ready: found=%v value=%q", utils.EnableSyncLabelsToClusterClaimsVar, found, value)
	}

	return nil
}

func clusterManagementAddOnsReferenceAddonNsConfig() error {
	for _, addonName := range []string{utils.WorkManagerAddonName, utils.ManagedServiceAccountAddonName, utils.ClusterProxyAddonName} {
		cma, err := utils.GetResource(dynamicClient, utils.ClusterManagementAddOnGVR, "", addonName)
		if err != nil {
			return fmt.Errorf("ClusterManagementAddOn %s: %w", addonName, err)
		}
		references, err := utils.ClusterManagementAddOnReferencesConfig(cma, mceNamespace, utils.AddonNsConfigName)
		if err != nil {
			return err
		}
		if !references {
			return fmt.Errorf("ClusterManagementAddOn %s does not reference addon-ns-config in %s yet", addonName, mceNamespace)
		}
	}
	return nil
}

// isMceImportConfigured reports whether configureMceImport=true is set on the
// hypershift-addon-deploy-config AddOnDeploymentConfig in the given MCE namespace.
func isMceImportConfigured(namespace string) (bool, error) {
	deployCfg, err := utils.GetResource(dynamicClient, utils.AddOnDeploymentConfigGVR, namespace, utils.HypershiftAddonDeployConfigName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	value, found, err := utils.GetAddOnDeploymentConfigCustomizedVariable(deployCfg, utils.ConfigureMceImportVar)
	if err != nil || !found {
		return false, err
	}
	return value == "true", nil
}
