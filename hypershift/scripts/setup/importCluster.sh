#!/bin/bash
TIMEOUT=300s
oc version

IMPORT_CLUSTER_API_URL="https://api.clc-acm-hs-import01.dev09.red-chesterfield.com:6443"
IMPORT_CLUSTER_USER="kubeadmin"
IMPORT_CLUSTER_PASSWORD="jcyFv-nYoxk-VID2d-Bz5hc"
ACM_NS=ocm

echo "Creating new project for cluster ${HOSTING_CLUSTER_NAME} being imported..."
oc new-project ${HOSTING_CLUSTER_NAME}

echo "Creating ManagedCluster CR..."
oc apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  labels:
    cloud: auto-detect
    vendor: auto-detect
  name: ${HOSTING_CLUSTER_NAME}
spec:
  hubAcceptsClient: true
EOF

sleep 30s

echo "Generating klusterlet-crd.yaml..."
oc get secret ${HOSTING_CLUSTER_NAME}-import -n ${HOSTING_CLUSTER_NAME} -o jsonpath={.data.crds\\.yaml} | base64 --decode > ${WORKSPACE}/klusterlet-crd.yaml
echo "Generating import.yaml..."
oc get secret ${HOSTING_CLUSTER_NAME}-import -n ${HOSTING_CLUSTER_NAME} -o jsonpath={.data.import\\.yaml} | base64 --decode > ${WORKSPACE}/import.yaml

echo "Logging into remote cluster..."
oc login ${IMPORT_CLUSTER_API_URL} -u ${IMPORT_CLUSTER_USER} -p ${IMPORT_CLUSTER_PASSWORD} --insecure-skip-tls-verify
                            
echo "Applying klusterlet-crd.yaml..."
oc apply -f ${WORKSPACE}/klusterlet-crd.yaml
echo

echo "Applying import.yaml..."
oc apply -f ${WORKSPACE}/import.yaml
echo

echo "Waiting for klusterlet to be in Running state..."
oc rollout status deployment klusterlet -n open-cluster-management-agent --timeout=${TIMEOUT}
oc get pod -l app=klusterlet -n open-cluster-management-agent
# oc wait --for=jsonpath="{.status.phase}"=Running pod -l app=klusterlet -n open-cluster-management-agent --timeout=${TIMEOUT}
# if [ $? -ne 0 ]; then
#     echo "ERROR: Timeout waiting for the klusterlet to be ready."
#     exit 1
# else
#     echo "Klusterlet is Running!"
# fi
# echo

echo "Returning context to hub cluster"
oc login ${HUB_API_URL} -u ${HUB_USER} -p ${HUB_PASSWORD} --insecure-skip-tls-verify
echo
                            
echo "Waiting for ManagedCluster to be Available ..."
oc wait --for=condition=ManagedClusterConditionAvailable managedcluster ${HOSTING_CLUSTER_NAME} --timeout=${TIMEOUT}
if [ $? -ne 0 ]; then
  printf "ERROR: Timeout waiting ${HOSTING_CLUSTER_NAME} to be available"
  exit 1
else
  echo "-- Cluster imported and available!"
fi
echo

echo "Checking if ACM is installed..."
oc get mch -n ${ACM_NS} multiclusterhub >> /dev/null
if [ $? -eq 0 ]; then
  echo "multiclusterhub (ACM) detected. Importing klusterlet add-ons..."
  oc apply -f - <<EOF
  apiVersion: agent.open-cluster-management.io/v1
  kind: KlusterletAddonConfig
  metadata:
    name: ${HOSTING_CLUSTER_NAME}
    namespace: ${HOSTING_CLUSTER_NAME}
  spec:
    applicationManager:
      enabled: true
    certPolicyController:
      enabled: true
    iamPolicyController:
      enabled: true
    policyController:
      enabled: true
    searchCollector:
      enabled: true
EOF
else
  echo "--multiclusterhub (ACM) not installed, skipping installing addons on cluster..."
fi