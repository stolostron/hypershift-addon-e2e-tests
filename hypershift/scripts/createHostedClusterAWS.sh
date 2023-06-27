#!/bin/bash

# Create hosted cluster using ${LOCAL_CLUSTER} as hosted cluster
# Pre-requisites: 
# 0. Logged into hub cluster
# 1. OIDC and DNS secret should exist already
# 2. Hypershift preview must be enabled on hub
# 3. Hypershift-addon should be in good state on ${LOCAL_CLUSTER}
# 4. Hypershift cli is installed

TIMEOUT=300s # default: 5 minute timeout for waiting commands
LOCAL_CLUSTER=local-cluster

# Verify required variables are present
if [ -z ${CLUSTER_NAME_PREFIX+x} ]; then
  echo "CLUSTER_NAME_PREFIX is not defined"
  exit 1
fi

if [ -z ${REGION+x} ]; then
  echo "REGION is not defined"
  exit 1
fi

if [ -z ${BASE_DOMAIN+x} ]; then
  echo "BASE_DOMAIN is not defined"
  exit 1
fi
if [ -z ${AWS_CREDS_FILE+x} ]; then
  echo "AWS_CREDS_FILE is not defined"
  exit 1
fi

if [ -z ${PULL_SECRET_FILE+x} ]; then
  echo "PULL_SECRET_FILE is not defined"
  exit 1
fi

if [ -z ${ACM_HC_NODE_POOL_REPLICAS+x} ]; then
  echo "ACM_HC_NODE_POOL_REPLICAS is not defined"
  exit 1
fi

if [ -z ${ACM_HC_OCP_RELEASE_IMAGE+x} ]; then
  echo "ACM_HC_OCP_RELEASE_IMAGE is not defined"
  exit 1
fi

if [ -z ${ACM_HC_INSTANCE_TYPE+x} ]; then
  echo "ACM_HC_INSTANCE_TYPE is not defined"
  exit 1
fi


echo "Waiting up to ${TIMEOUT} to verify the hosting service cluster is configured with the s3 bucket..."
oc wait configmap/oidc-storage-provider-s3-config -n kube-public --for=jsonpath='{.data.name}'=${BUCKET_NAME} --timeout=${TIMEOUT}
echo "S3 Bucket secret created and hosting cluster configured!"
echo
#TODO: verify DNS secret exist
#TODO: verify hypershift preview is enabled 
#TODO: verify hypershift-addon state
echo "Verify the HyperShift operator deployment and pods are running as expected ..."
oc rollout status deployment operator -n hypershift --timeout=300s
oc wait --for=jsonpath='{.status.phase}'=Running pod -l name=operator -n hypershift --timeout=$TIMEOUT
if [ $? -ne 0 ]; then
  echo "ERROR: Timeout waiting for the HyperShift operator to be ready."
  exit 1
else
  echo "Hypershift addon successfully installed and online. You can now provision a hosted control plane cluster."
fi
echo

echo "Waiting up to ${TIMEOUT} for the HyperShift addon on ${LOCAL_CLUSTER} to be available ..."
oc wait --for=condition=Available=True managedclusteraddon/hypershift-addon -n ${LOCAL_CLUSTER} --timeout=$TIMEOUT
if [ $? -ne 0 ]; then
  echo "ERROR: Timeout waiting for the HyperShift addon to become Available"
  exit 1
else
  echo "Hypershift addon installed successfully!"
fi
echo

ACM_HC_CLUSTER_NAME=${CLUSTER_NAME_PREFIX}$(cat /dev/urandom | env LC_ALL=C tr -dc 'a-z0-9' | fold -w 6 | head -n 1)
ACM_HC_INFRA_ID=${ACM_HC_CLUSTER_NAME}

echo "$(date) ==== Creating hosted cluster using hypershift version installed ===="
hypershift version
echo

### create hosted cluster using ${LOCAL_CLUSTER}
hypershiftCreateCmdAWS="hypershift create cluster aws \
    --name ${ACM_HC_CLUSTER_NAME} \
    --infra-id ${ACM_HC_INFRA_ID} \
    --base-domain ${BASE_DOMAIN} \
    --aws-creds ${AWS_CREDS_FILE} \
    --pull-secret ${PULL_SECRET_FILE} \
    --region ${REGION} \
    --instance-type ${ACM_HC_INSTANCE_TYPE} \
    --release-image ${ACM_HC_OCP_RELEASE_IMAGE} \
    --node-pool-replicas ${ACM_HC_NODE_POOL_REPLICAS} \ 
    --namespace ${LOCAL_CLUSTER} \
    --generate-ssh"
hypershiftDestroyCmdAWS="hypershift destroy cluster aws --infra-id ${ACM_HC_INFRA_ID} --aws-creds ${AWS_CREDS_FILE} --name ${ACM_HC_CLUSTER_NAME} --base-domain ${BASE_DOMAIN} --namespace ${LOCAL_CLUSTER} --destroy-cloud-resources"

echo "$(date) ==== Creating hosted cluster: ${hypershiftCreateCmdAWS} ===="
${hypershiftCreateCmdAWS}
if [ $? -ne 0 ]; then
    echo "$(date) ERROR: Failed to create hosted cluster ${ACM_HC_CLUSTER_NAME}!"
    echo "$(date) Destroying the AWS infrastructure for hosted cluster ${ACM_HC_CLUSTER_NAME}"
    hypershift destroy cluster aws --infra-id ${ACM_HC_INFRA_ID} --aws-creds ${AWS_CREDS_FILE} --name ${ACM_HC_CLUSTER_NAME} --base-domain ${BASE_DOMAIN} --namespace ${LOCAL_CLUSTER} --destroy-cloud-resources
    exit 1
fi

# annotate the hostedcluster
oc annotate hostedclusters ${ACM_HC_INFRA_ID} cluster.open-cluster-management.io/hypershiftdeployment=default -n ${LOCAL_CLUSTER}

## import the managed cluster
echo "$(date) ==== Creating managed cluster for hosted cluster ${ACM_HC_CLUSTER_NAME} ===="
oc apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  annotations:
    import.open-cluster-management.io/hosting-cluster-name: ${LOCAL_CLUSTER}
    import.open-cluster-management.io/klusterlet-deploy-mode: Hosted
    open-cluster-management/created-via: other
  labels:
    cloud: auto-detect
    cluster.open-cluster-management.io/clusterset: default
    name: ${ACM_HC_INFRA_ID}
    vendor: OpenShift
  name: ${ACM_HC_INFRA_ID}
spec:
  hubAcceptsClient: true
  leaseDurationSeconds: 60
EOF
if [ $? -ne 0 ]; then
    echo "$(date) ERROR: Failed to create managed cluster for ${ACM_HC_INFRA_ID}!"
    exit 1
fi

echo "$(date) Installing additional-addons for the managed cluster ${ACM_HC_INFRA_ID}"
oc apply -f - <<EOF
apiVersion: agent.open-cluster-management.io/v1
kind: KlusterletAddonConfig
metadata:
    name: ${ACM_HC_INFRA_ID}
    namespace: ${ACM_HC_INFRA_ID}
spec:
    clusterName: ${ACM_HC_INFRA_ID}
    clusterNamespace: ${ACM_HC_INFRA_ID}
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
if [ $? -ne 0 ]; then
    echo "$(date) ERROR: Failed to create apply add-ons for managed cluster ${ACM_HC_INFRA_ID}!"
    exit 1
fi

echo "For destroy, first delete managed cluster:"
echo "oc delete managedcluster ${ACM_HC_CLUSTER_NAME}"
echo "Then run hypershift hosted cluster destroy command: "
echo ${hypershiftDestroyCmdAWS}

echo "$(date) ==== Running destroy ===="
oc delete managedcluster ${ACM_HC_CLUSTER_NAME} --wait=true --timeout=1200s
if [ $? -ne 0 ]; then
    echo "ERROR: Ran into issue deleting managedcluster ${ACM_HC_CLUSTER_NAME}"
    exit 1
fi
${hypershiftDestroyCmdAWS}
if [ $? -ne 0 ]; then
    echo "ERROR: Ran into issue deleting hosted cluster ${ACM_HC_CLUSTER_NAME}"
    exit 1
fi

#TODO: create secret/config with 
    # infra-d
    # cluster-name
    # aws-creds
    # base-domain

# oc get nodepools -A; echo; echo; oc get hostedclusters -A; echo; echo; oc get managedclusters
## TODO: verify the hostedcluster is ready
## TODO: verify nodepools are ready on the HC
## TODO: verify addons are ready on the HC
## TODO: verify managedcluster cr is good for the HC
## TODO: verify all addons are ready / Available

## TODO: create 1 OCP 4.11, 4.12, 4.13? (2.7.1+ only?)