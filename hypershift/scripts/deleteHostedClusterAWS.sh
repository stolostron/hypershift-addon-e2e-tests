#!/bin/bash

ACM_HC_CLUSTER_NAME=$1
ACM_HC_INFRA_ID=$2
BASE_DOMAIN=$3

# todo: check if managedcluster for the hostedcluster still exists, delete if yes
deleteMcCmd="oc delete managedcluster ${ACM_HC_CLUSTER_NAME}"
echo ${deleteMcCmd}
${deleteMcCmd}
# verify gone

# run destroy command
destroyCMD="hypershift destroy cluster aws \
  --infra-id ${ACM_HC_INFRA_ID} \
  --aws-creds ${AWS_CREDS_FILE} \
  --name ${ACM_HC_CLUSTER_NAME} \
  --base-domain ${BASE_DOMAIN} \
  --namespace local-cluster \
  --destroy-cloud-resources"
echo ${destroyCMD}
${destroyCMD}

## ensure no more HC on hub
getHostedClustersCmd="oc get hostedclusters -n local-cluster -o jsonpath='{.items[*].metadata.name}'"