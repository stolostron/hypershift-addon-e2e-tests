#!/bin/bash
# Requires to be connected to the hub already before running
# Requires all variables already set as required by setup scripts

if [ -z ${PULL_SECRET+x} ]; then
  echo "WARN: PULL_SECRET is not defined, defaulting to the one that exists on the MCE/ACM hub via:"
  echo "oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d &> hub_pull_secret"
  oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d &> hub_pull_secret
  export PULL_SECRET="./hub_pull_secret"
fi

if [ -z ${HOSTING_CLUSTER+x} ]; then
  echo "WARN: HOSTING_CLUSTER is not defined, defaulting to local-cluster"
  export HOSTING_CLUSTER="local-cluster"
fi

if [ -z ${HOSTED_CLUSTER_NS+x} ]; then
  echo "WARN: HOSTED_CLUSTER_NS is not defined, defaulting to clusters"
  export HOSTED_CLUSTER_NS="clusters"
fi

if [ -z ${HCP_NODE_POOL_REPLICAS+x} ]; then
  echo "WARN: HCP_NODE_POOL_REPLICAS is not defined, defaulting to 2"
  export HCP_NODE_POOL_REPLICAS="2"
fi

if [ -z ${EXT_DNS_DOMAIN+x} ]; then
  echo "WARN: EXT_DNS_DOMAIN is not defined, defaulting external dns name to acmqe-hs.qe.red-chesterfield.com"
  export EXT_DNS_DOMAIN="acmqe-hs.qe.red-chesterfield.com"
fi

## HCP_RELEASE_IMAGE=quay.io/openshift-release-dev/ocp-release:4.16.0-ec.5-multi

if [ -z ${JUNIT_REPORT_FILE+x} ]; then
  echo "WARN: JUNIT_REPORT_FILE is not defined, defaulting to ./results/result.xml"
  export JUNIT_REPORT_FILE="./results/result.xml"
fi

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: "$HOSTED_CLUSTER_NS"
EOF

./setup_installHypershiftBinary.sh
./setup_create_aws_hcp_secrets.sh

# Wait for hypershift-addon to be available
echo "$(date) Waiting for hypershift-addon..."
FOUND=1
SECONDS=0
while [ ${FOUND} -eq 1 ]; do
  # Wait up to 10min
  if [ ${SECONDS} -gt 600 ]; then
    echo "Timeout waiting for hypershift-addon to be available."
    echo "List of current pods:"
    oc get managedclusteraddon hypershift-addon -n "${HOSTING_CLUSTER}" -o yaml
    exit 1
  fi

  addonAvailable=$(oc get managedclusteraddon hypershift-addon -n "${HOSTING_CLUSTER}" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
  addonDegraded=$(oc get managedclusteraddon hypershift-addon -n "${HOSTING_CLUSTER}" -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}')

  if [[ ("$addonAvailable" == "True") && ("$addonDegraded" == "False") ]]; then
    echo "Hypershift addon is available"
    break
  fi
  sleep 10
  ((SECONDS = SECONDS + 10))
done