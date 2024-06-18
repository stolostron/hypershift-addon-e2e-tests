#!/bin/bash

# Run this script on the ACM hub cluster to import managed MCE clusters into ACM
# as well as auto-importing the HCPs on those managed MCE clusters.

## Requires:
# kubeconfigs for the managed MCE clusters to bring into ACM in the kubeconfigs/ directory
## ensure kubeconfigs are named as <cluster-name>.kubeconfig
# envsubst
# oc or kubectl
# clusteradm

MANIFESTS_DIR="resources/managedMCE"
MCE_CONFIGS_DIR="resources/managedMCE/kubeconfigs"

export ADDON_CONFIG_NS=${ADDON_CONFIG_NS:-"addon-ns-config"}
export AGENT_NS=${AGENT_NS:-"open-cluster-management-agent-addon-discovery"}
export KLUSTERLET_CONFIG=${KLUSTERLET_CONFIG:-"mce-import-klusterlet-config"}
export MCE_NS=${MCE_NS:-"multicluster-engine"}
export TIMEOUT=${TIMEOUT:-"300s"}

envsubst < $MANIFESTS_DIR/00-addondeploymentconfig.yaml | oc apply -f -

# Update ClusterManagementAddOn for work-manager addon to add a reference to the AddOnDeploymentConfig resource created in the previous step.
# Do the same update for managed-serviceaccount addon.
envsubst < $MANIFESTS_DIR/01-clustermanagementaddon-work-mgr.yaml | oc apply -f -
envsubst < $MANIFESTS_DIR/02-clustermanagementaddon-sa.yaml | oc apply -f -

# enable the hypershift addon on the managed mce clusters
oc patch addondeploymentconfig hypershift-addon-deploy-config -n "${MCE_NS}" --type=merge -p "{\"spec\":{\"agentInstallNamespace\":\"${AGENT_NS}\"}}"
oc patch addondeploymentconfig hypershift-addon-deploy-config -n "${MCE_NS}" --type=merge -p '{"spec":{"customizedVariables":[{"name":"disableMetrics","value": "true"}]}}'

# Ensure addons are installed in the specified namespace
oc get deployment -n "${AGENT_NS}"
oc wait --for=condition=Available=True deployment klusterlet-addon-workmgr -n "${AGENT_NS}" --timeout "${TIMEOUT}"
oc wait --for=condition=Available=True deployment managed-serviceaccount-addon-agent -n "${AGENT_NS}" --timeout "${TIMEOUT}"

# Create the KlusterletConfig resource necessary for importing MCE
envsubst < $MANIFESTS_DIR/03-klusterletconfig.yaml  | oc apply -f -

# create the mce managed cluster with the necessary annotations
# DO NOT enable any other ACM addons for the imported MCE.
# do a loop in case we want to import multiple MCE's
# iterate all files in the $MANIFESTS_DIR/kubeconfigs directory
for kubeconfig in "$MCE_CONFIGS_DIR"/*; do
    if [ ! -e "$kubeconfig" ]; then
        echo "🧐 No files found."
        exit 1
    fi
    managedcluster_name=$(basename "$kubeconfig" | cut -d. -f1)
    kubeconfig=$(<"$kubeconfig")

    export MCE_MANAGED_CLUSTER_NAME=$managedcluster_name
    export MCE_KUBECONFIG=$(echo "$kubeconfig" | sed 's/^/    /')

    echo
    echo "Applying the following auto-import-secret: "
    echo
    envsubst < $MANIFESTS_DIR/05-auto-import-secret.yaml
    echo

    envsubst < $MANIFESTS_DIR/04-managedcluster-mce.yaml | oc apply -f -
    envsubst < $MANIFESTS_DIR/05-auto-import-secret.yaml | oc apply -f -
done

# enable the hypershift addon on the managed mce clusters
for kubeconfig in "$MCE_CONFIGS_DIR"/*; do
    if [ ! -e "$kubeconfig" ]; then
        echo "🧐 No files found."
        exit 1
    fi
    managedcluster_name=$(basename "$kubeconfig" | cut -d. -f1)

    echo "Enabling hypershift-addon for the managed MCE cluster $managedcluster_name..."

    clusteradm addon enable --names hypershift-addon --clusters "${managedcluster_name}"

    oc wait --for=condition=Available=True managedclusteraddon cluster-proxy -n "$managedcluster_name" --timeout "${TIMEOUT}"
    oc wait --for=condition=Available=True managedclusteraddon managed-serviceaccount -n "$managedcluster_name" --timeout "${TIMEOUT}"
    oc wait --for=condition=Available=True managedclusteraddon work-manager -n "$managedcluster_name" --timeout "${TIMEOUT}"
    oc wait --for=condition=Available=True managedclusteraddon hypershift-addon -n "$managedcluster_name" --timeout "${TIMEOUT}"
done

# Apply policy for auto importing discovered HCP clusters
oc apply -f $MANIFESTS_DIR/06-policy-mce-hcp-autoimport.yaml

for kubeconfig in "$MCE_CONFIGS_DIR"/*; do
    if [ ! -e "$kubeconfig" ]; then
        echo "🧐 No files found."
        exit 1
    fi
    managedcluster_name=$(basename "$kubeconfig" | cut -d. -f1)
    echo
    echo
    oc --kubeconfig "$kubeconfig" cluster-info
    echo "Checking addons are installed in the correct namespace on the managed MCE clusters..."
    oc --kubeconfig "$kubeconfig" wait --for=condition=Available=True deployment klusterlet-addon-workmgr -n "${AGENT_NS}" --timeout "${TIMEOUT}"
    oc --kubeconfig "$kubeconfig" wait --for=condition=Available=True deployment managed-serviceaccount-addon-agent -n "${AGENT_NS}" --timeout "${TIMEOUT}"
    oc --kubeconfig "$kubeconfig" wait --for=condition=Available=True deployment hypershift-addon-agent -n "${AGENT_NS}" --timeout "${TIMEOUT}"

    ## get list of HCPs on the MCEs and then
    ## check on the hub that the HCPs appear as DiscoveredClusters with name <managed_mce_name>-<hcp_name>
    echo "Hosted clusters on managed MCE hub $managedcluster_name:"
    hostedclusters=$(oc --kubeconfig "$kubeconfig" get hostedclusters -A -o jsonpath='{.items[*].metadata.name}')
    echo "$hostedclusters"

    for hostedcluster in $hostedclusters; do
        echo "Checking if the hostedcluster $hostedcluster is correctly reflected as a DiscoveredCluster"
        discoveredcluster=$(oc get discoveredcluster -A -o jsonpath="{.items[?(@.metadata.labels.hypershift\.open-cluster-management\.io/hc-name==\"$hostedcluster\")].metadata.name}")
        oc wait discoveredcluster "$discoveredcluster" -n "$managedcluster_name" --for=jsonpath='{.spec.type}'=MultiClusterEngineHCP
        oc wait discoveredcluster "$discoveredcluster" -n "$managedcluster_name" --for=jsonpath='{.spec.status}'=Active
        oc wait discoveredcluster "$discoveredcluster" -n "$managedcluster_name" --for=jsonpath='{.spec.importAsManagedCluster}'=true
        oc wait discoveredcluster "$discoveredcluster" -n "$managedcluster_name" --for=jsonpath='{.spec.isManagedCluster}'=true
        echo

        HCP_MC=$managedcluster_name-$hostedcluster
        echo "Checking the managedcluster $HCP_MC is imported and healthy..."
        oc wait --for=condition=ManagedClusterImportSucceeded=True managedclusters "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=ManagedClusterAvailable=True managedclusters "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=ManagedClusterJoined=True managedclusters "$HCP_MC" --timeout ${TIMEOUT}
        echo

        echo "Checking the managedcluster addons for $HCP_MC are healthy..."
        oc wait --for=condition=Available=True managedclusteraddon application-manager -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon cluster-proxy -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon config-policy-controller  -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon cert-policy-controller -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon governance-policy-framework -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon work-manager -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon managed-serviceaccount -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon search-collector -n "$HCP_MC" --timeout ${TIMEOUT}
        oc wait --for=condition=Available=True managedclusteraddon observability-controller -n "$HCP_MC" --timeout ${TIMEOUT}
    done
done