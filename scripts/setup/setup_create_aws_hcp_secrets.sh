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

TIMEOUT=600s # default: 10 minute timeout for oc wait commands

cleanup() {
  echo "cleaning up tmp files"
  rm -rf ./.aws/
}

# Delete all aws credentials in the current directory on any exit
trap cleanup EXIT

if [ -z ${HCP_REGION+x} ]; then
  echo "WARN: HCP_REGION is not defined, defaulting to us-east-1"
  HCP_REGION="us-east-1"
fi

if [ -z ${S3_REGION+x} ]; then
  echo "WARN: S3_REGION is not defined, defaulting to us-east-1"
  S3_REGION="us-east-1"
fi

if [ -z ${S3_BUCKET_NAME+x} ]; then
  echo "ERROR: S3_BUCKET_NAME is not defined"
  exit 1
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
  echo "ERROR: PULL_SECRET is not defined"
  exit 1
fi

PULL_SECRET=$(cat "${PULL_SECRET}")

# set default instance type for aws
# HCP_AWS_INSTANCE_TYPE=m6g.large
# HCP_AWS_ARCH=arm64

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
# Create AWS sts-creds.json
#######################################################
aws configure set aws_access_key_id $AWS_ACCESS_KEY_ID
aws configure set aws_secret_access_key $AWS_SECRET_ACCESS_KEY
aws sts get-caller-identity --no-cli-pager --region $HCP_REGION
# TODO: check identity is valid
aws sts get-session-token --no-cli-pager --output json --region $HCP_REGION > sts-creds.json
# TODO: check session token is good, store/retreive from secret?

#######################################################
# TODO: Create AWS role if doesn't exist
#######################################################

#######################################################
# TODO: Create AWS S3 Bucket if doesn't exist
#######################################################

#######################################################
# Create secrets for hypershift operator installation
#######################################################
echo "$(date) creating secret hypershift-operator-oidc-provider-s3-credentials..."
oc delete secret hypershift-operator-oidc-provider-s3-credentials --ignore-not-found -n ${HOSTING_CLUSTER}
oc create secret generic hypershift-operator-oidc-provider-s3-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=bucket=${S3_BUCKET_NAME} --from-literal=region=${S3_REGION} -n ${HOSTING_CLUSTER}
if [ $? -ne 0 ]; then
  echo "$(date) failed to create secret hypershift-operator-oidc-provider-s3-credentials"
  exit 1
fi
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/credentials= --overwrite
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/type=awss3 --overwrite
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER} cluster.open-cluster-management.io/backup=true --overwrite
echo
#######################################################

#######################################################
## Create secrets
#######################################################

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

echo "$(date) Waiting for configmap/oidc-storage-provider-s3-config to be configured..."
FOUND=1
SECONDS=0
while [ ${FOUND} -eq 1 ]; do
  # Wait up to 10min
  if [ ${SECONDS} -gt 600 ]; then
    echo "Timeout waiting for configmap/oidc-storage-provider-s3-config!"
    exit 1
  fi

  configMapName=$(oc get configmap oidc-storage-provider-s3-config -n kube-public -o jsonpath='{.data.name}')

  if [[ ("$configMapName" == "${S3_BUCKET_NAME}") ]]; then
    echo "$(date) S3 Bucket secret created and hosting cluster configured!"
    break
  fi
  sleep 10
  ((SECONDS = SECONDS + 10))
done

echo "$(date) Waiting up to ${TIMEOUT} to verify the hosting service cluster is configured with the s3 bucket..."
oc wait configmap/oidc-storage-provider-s3-config -n kube-public --for=jsonpath='{.data.name}'="${S3_BUCKET_NAME}" --timeout=${TIMEOUT}
if [ $? -ne 0 ]; then
  echo "$(date) failed to get configmap/oidc-storage-provider-s3-config"
  exit 1
fi

# dump the hypershift-addon CR
oc get managedclusteraddon hypershift-addon -n "${HOSTING_CLUSTER}" -o yaml
