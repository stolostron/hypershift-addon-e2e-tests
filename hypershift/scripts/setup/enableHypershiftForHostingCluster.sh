#!/bin/bash

if [ -z ${HOSTING_CLUSTER_NAME+x} ]; then
  echo "ERROR: HOSTING_CLUSTER_NAME is not defined"
  exit 1
fi

# Create secrets for hypershift operator installation
echo "create secret hypershift-operator-oidc-provider-s3-credentials"
oc delete secret hypershift-operator-oidc-provider-s3-credentials --ignore-not-found -n ${HOSTING_CLUSTER_NAME}
oc create secret generic hypershift-operator-oidc-provider-s3-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=bucket=${S3_BUCKET_NAME} --from-literal=region=${REGION} -n ${HOSTING_CLUSTER_NAME}
if [ $? -ne 0 ]; then
    echo "$(date) failed to create secret hypershift-operator-oidc-provider-s3-credentials"
    exit 1
fi
echo

echo "create secret hypershift-operator-external-dns-credentials"
oc delete secret hypershift-operator-external-dns-credentials --ignore-not-found -n ${HOSTING_CLUSTER_NAME}
oc create secret generic hypershift-operator-external-dns-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=provider=aws --from-literal=domain-filter=${EXT_DNS_DOMAIN} -n ${HOSTING_CLUSTER_NAME}
if [ $? -ne 0 ]; then
    echo "$(date) failed to create secret hypershift-operator-external-dns-credentials"
    exit 1
fi
echo

echo "Applying hypershift addon..."
oc apply -f - <<EOF
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: hypershift-addon
  namespace: $HOSTING_CLUSTER_NAME
spec:
  installNamespace: open-cluster-management-agent-addon
EOF
if [ $? -ne 0 ]; then
    echo "$(date) failed to apply hypershift-addon"
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
        oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o yaml
        exit 1
    fi

    addonAvailable=`oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o jsonpath='{.status.conditions[?(@.type=="Available")].status}'`
    addonDegraded=`oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}'`

    if [[ ("$addonAvailable" == "True") && ("$addonDegraded" == "False") ]]; then 
        echo "Hypershift addon is available"
        break
    fi
    sleep 10
    (( SECONDS = SECONDS + 10 ))
done