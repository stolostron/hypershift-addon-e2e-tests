#!/bin/bash
# This script is run on the hub cluster to help create the following secrets for hypershift:
# 1. hypershift-operator-oidc-provider-s3-credentials
# 2. hypershift-operator-external-dns-credentials
# 3. MCE/ACM AWS Secret

#########################################
#   POPULATE THESE WITH ENV VARS        #
#   ie: export OCP_RELEASE_IMAG=foobar  #
#########################################
# This public hosted zone needs to exist in AWS Route53. Replace with your own
# The hypershift-addon must be enabled with external DNS option
#export S3_BUCKET_NAME=

# The AWS creds
#export AWS_ACCESS_KEY_ID=
#export AWS_SECRET_ACCESS_KEY=
#export BASE_DOMAIN=
#export SSH_PUBLIC_KEY=
#export SSH_PRIVATE_KEY=

#########################################
## OPTIONAL FIELDS #####################
#########################################
# S3 bucket region
#export S3_REGION=us-east-1

# Hypershift operator external dns value
# export EXT_DNS_DOMAIN=acmqe-hs.qe.red-chesterfield.com

# The name of the secret that will be created on the hosting cluster
# if the secret doesn't yet exist, then it will be created and will require base domain and ssh keys variables set
#export SECRET_AWS_CRED_NAME=clc-hs-aws-cred

# HOSTED_CLUSTER_NS is the target namespace where the hosted cluster is created
#export HOSTED_CLUSTER_NS=clusters

# HOSTING_CLUSTER is the hosting cluster for the hcp, 
# DEFAULTS to local-cluster for self-managed hcp if not specified
#export HOSTING_CLUSTER=local-cluster

# OCP PULL SECRET for the hosted cluster, defaults to the one that exists on the MCE/ACM hub
#export PULL_SECRET=/Users/dhuynh/dhu-pull-secret.txt
#########################################

TIMEOUT=300s # default: 5 minute timeout for oc wait commands

cleanup() {
  echo "cleaning up tmp files"
  rm -rf ./.aws/
}

# Delete all aws credentials in the current directory on any exit
trap cleanup EXIT

if [ -z ${HOSTING_CLUSTER+x} ]; then
  echo "WARN: HOSTING_CLUSTER is not defined, defaulting to local-cluster"
  HOSTING_CLUSTER="local-cluster"
fi

if [ -z ${HOSTED_CLUSTER_NS+x} ]; then
  echo "WARN: HOSTED_CLUSTER_NS is not defined, defaulting to clusters"
  HOSTED_CLUSTER_NS="clusters"
fi

if [ -z ${HCP_REGION+x} ]; then
  echo "WARN: HCP_REGION is not defined, defaulting to us-east-1"
  HCP_REGION="us-east-1"
fi

if [ -z ${HCP_NODE_POOL_REPLICAS+x} ]; then
  echo "WARN: HCP_NODE_POOL_REPLICAS is not defined, defaulting to 2"
  HCP_REGION="2"
fi

if [ -z ${SECRET_AWS_CRED_NAME+x} ]; then
  echo "WARN: SECRET_AWS_CRED_NAME is not defined, defaulting to clc-hs-aws-cred"
  SECRET_AWS_CRED_NAME="clc-hs-aws-cred"
fi

if [ -z ${S3_REGION+x} ]; then
  echo "WARN: S3_REGION is not defined, defaulting to us-east-1"
  S3_REGION="us-east-1"
fi

if [ -z ${S3_BUCKET_NAME+x} ]; then
  echo "ERROR: S3_BUCKET_NAME is not defined"
  exit 1
fi

if [ -z ${EXT_DNS_DOMAIN+x} ]; then
  echo "WARN: EXT_DNS_DOMAIN is not defined, defaulting external dns name to acmqe-hs.qe.red-chesterfield.com"
  EXT_DNS_DOMAIN="acmqe-hs.qe.red-chesterfield.com"
fi

if [ -z ${AWS_ACCESS_KEY_ID+x} ]; then
  echo "ERROR: AWS_ACCESS_KEY_ID is not defined"
  exit 1
fi

if [ -z ${AWS_SECRET_ACCESS_KEY+x} ]; then
  echo "ERROR: AWS_SECRET_ACCESS_KEY is not defined"
  exit 1
fi

if [ -z ${PULL_SECRET+x} ]; then
  echo "WARN: PULL_SECRET is not defined, defaulting to the one that exists on the MCE/ACM hub via:"
  PULL_SECRET=$(oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d)
fi

if [ -z ${JUNIT_REPORT_FILE+x} ]; then
  echo "WARN: JUNIT_REPORT_FILE is not defined, defaulting to ./results/result.xml"
  JUNIT_REPORT_FILE="./results/result.xml"
fi

cleanup # clean up tmp files first
mkdir ./.aws/
cat <<EOF >./.aws/credentials
[default]
aws_access_key_id=${AWS_ACCESS_KEY_ID}
aws_secret_access_key=${AWS_SECRET_ACCESS_KEY}
EOF

AWS_CREDS_FILE=./.aws/credentials
cat ${AWS_CREDS_FILE}

#######################################################
# Create secrets for hypershift operator installation
#######################################################
echo "$(date) creating secret hypershift-operator-oidc-provider-s3-credentials..."
oc delete secret hypershift-operator-oidc-provider-s3-credentials --ignore-not-found -n ${HOSTING_CLUSTER}
oc create secret generic hypershift-operator-oidc-provider-s3-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=bucket=${S3_BUCKET_NAME} --from-literal=region=${S3_REGION} -n ${HOSTING_CLUSTER}
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/credentials= --overwrite
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/type=awss3 --overwrite
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
  echo "$(date) failed to create secret hypershift-operator-oidc-provider-s3-credentials"
  exit 1
fi
echo

echo "$(date) creating secret hypershift-operator-external-dns-credentials..."
oc delete secret hypershift-operator-external-dns-credentials --ignore-not-found -n ${HOSTING_CLUSTER}
oc create secret generic hypershift-operator-external-dns-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=provider=aws --from-literal=domain-filter=${EXT_DNS_DOMAIN} -n ${HOSTING_CLUSTER}
oc label secret hypershift-operator-external-dns-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
  echo "$(date) failed to create secret hypershift-operator-external-dns-credentials"
  exit 1
fi
echo
#######################################################

#######################################################
# Create mce aws secret on the hosting cluster
#######################################################
if oc get secret "$SECRET_AWS_CRED_NAME" -n "$HOSTED_CLUSTER_NS"; then
  echo "Secret $SECRET_AWS_CRED_NAME already exists in $HOSTED_CLUSTER_NS namespace... re-creating secret"
  # delete secret ignore error
  oc delete secret "$SECRET_AWS_CRED_NAME" -n "$HOSTED_CLUSTER_NS" --ignore-not-found
fi

echo "Creating new secret $SECRET_AWS_CRED_NAME in $HOSTED_CLUSTER_NS namespace..."

if [ -z ${HCP_BASE_DOMAIN_NAME+x} ]; then
  echo "ERROR: HCP_BASE_DOMAIN_NAME is not defined"
  exit 1
fi

if [ -z ${SSH_PUBLIC_KEY+x} ]; then
  echo "ERROR: SSH_PUBLIC_KEY is not defined"
  exit 1
fi

if [ -z ${SSH_PRIVATE_KEY+x} ]; then
  echo "ERROR: SSH_PRIVATE_KEY is not defined"
  exit 1
fi

oc apply -f - <<EOF
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: $SECRET_AWS_CRED_NAME
  namespace: $HOSTED_CLUSTER_NS
  labels:
    cluster.open-cluster-management.io/type: aws
    cluster.open-cluster-management.io/credentials: ""
stringData:
  aws_access_key_id: $AWS_ACCESS_KEY_ID
  aws_secret_access_key: $AWS_SECRET_ACCESS_KEY
  baseDomain: $HCP_BASE_DOMAIN_NAME
  pullSecret: >
    $PULL_SECRET
  ssh-privatekey: |
    $SSH_PRIVATE_KEY
  ssh-publickey: >
    $SSH_PUBLIC_KEY
  httpProxy: ""
  httpsProxy: ""
  noProxy: ""
  additionalTrustBundle: ""
EOF
if [ $? -ne 0 ]; then
  echo "$(date) failed to create secret aws mce secret"
  exit 1
fi

#######################################################
## Create secrets
#######################################################

echo "$(date) Waiting up to ${TIMEOUT} to verify the hosting service cluster is configured with the s3 bucket..."
oc wait configmap/oidc-storage-provider-s3-config -n kube-public --for=jsonpath='{.data.name}'="${S3_BUCKET_NAME}" --timeout=${TIMEOUT}
echo "$(date) S3 Bucket secret created and hosting cluster configured!"
echo

echo "$(date) Waiting up to ${TIMEOUT} to verify the hosting service cluster is configured with the AWS secret creds..."
oc wait secret/"${SECRET_AWS_CRED_NAME}" -n "${HOSTED_CLUSTER_NS}" --for=jsonpath='{.metadata.name}'="${SECRET_AWS_CRED_NAME}" --timeout=${TIMEOUT}
echo "$(date) S3 Bucket secret created and hosting cluster configured!"
echo

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