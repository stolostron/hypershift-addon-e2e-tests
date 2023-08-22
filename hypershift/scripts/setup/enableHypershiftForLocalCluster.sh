#!/bin/bash

## This script:
## 1. Creates hypershift-operator-oidc-provider-s3-credentials secret with backup enabled
## 2. Creates hypershift-operator-external-dns-credentials with backup enabled
## 3. Enables hypershift feature on hub
## 4. Waits for hypershift-addon to be in good condition

LOCAL_CLUSTER='local-cluster'

# Create secrets for hypershift operator installation
echo "create secret hypershift-operator-oidc-provider-s3-credentials"
oc delete secret hypershift-operator-oidc-provider-s3-credentials --ignore-not-found -n ${LOCAL_CLUSTER}
oc create secret generic hypershift-operator-oidc-provider-s3-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=bucket=${S3_BUCKET_NAME} --from-literal=region=${REGION} -n ${LOCAL_CLUSTER}
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${LOCAL_CLUSTER} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
    echo "$(date) failed to create secret hypershift-operator-oidc-provider-s3-credentials"
    exit 1
fi
echo

echo "create secret hypershift-operator-external-dns-credentials"
oc delete secret hypershift-operator-external-dns-credentials --ignore-not-found -n ${LOCAL_CLUSTER}
oc create secret generic hypershift-operator-external-dns-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=provider=aws --from-literal=domain-filter=${EXT_DNS_DOMAIN} -n ${LOCAL_CLUSTER}
oc label secret hypershift-operator-external-dns-credentials -n ${LOCAL_CLUSTER} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
    echo "$(date) failed tocreate secret hypershift-operator-external-dns-credentials"
    exit 1
fi
echo

echo "$(date) ==== Verify ACM or MCE is installed ===="
${OC_COMMAND} get mch -n ${ACM_NS} multiclusterhub >> /dev/null
if [ $? -eq 0 ]; then
  echo "multiclusterhub (ACM) installed"
  MCE_NAME="multiclusterengine"
fi
echo "$(date) mce name: ${MCE_NAME}"

oc get mce ${MCE_NAME} >> /dev/null
if [ $? -ne 0 ]; then
  echo "$(date) ${MCE_NAME} is not available, please install the multi-cluster engine"
  exit 1
fi
echo

# Enable the hypershift feature. This also installs the hypershift addon for ${LOCAL_CLUSTER}
echo "$(date) ==== Patch hypershift-preview feature ===="
oc patch mce ${MCE_NAME} --type=merge -p '{"spec":{"overrides":{"components":[{"name":"hypershift-preview","enabled": true}]}}}'
if [ $? -ne 0 ]; then
    echo "$(date) failed to enable hypershift-preview in MCE"
    exit 1
fi
echo

# Wait for hypershift-addon to be available
echo "waiting for hypershift-addon..."
FOUND=1
SECONDS=0
running="\([0-9]\+\)\/\1"
while [ ${FOUND} -eq 1 ]; do
    # Wait up to 10min
    if [ $SECONDS -gt 600 ]; then
        echo "Timeout waiting for hypershift-addon to be available."
        echo "List of current pods:"
        oc get managedclusteraddon hypershift-addon -n ${LOCAL_CLUSTER} -o yaml
        exit 1
    fi

    addonAvailable=`oc get managedclusteraddon hypershift-addon -n ${LOCAL_CLUSTER} -o jsonpath='{.status.conditions[?(@.type=="Available")].status}'`
    addonDegraded=`oc get managedclusteraddon hypershift-addon -n ${LOCAL_CLUSTER} -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}'`

    if [[ ("$addonAvailable" == "True") && ("$addonDegraded" == "False") ]]; then 
        echo "Hypershift addon is available"
        break
    fi
    sleep 10
    (( SECONDS = SECONDS + 10 ))
done