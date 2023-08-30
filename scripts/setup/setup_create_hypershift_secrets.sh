#!/bin/bash
# This script is run on the hub cluster to help create the following secrets for hypershift:
# 1. hypershift-operator-oidc-provider-s3-credentials
# 2. hypershift-operator-external-dns-credentials

#########################################
#   POPULATE THESE WITH ENV VARS        #
#   ie: export OCP_RELEASE_IMAG=foobar  #
#########################################
# This public hosted zone needs to exist in AWS Route53. Replace with your own
# The hypershift-addon must be enabled with external DNS option
#export EXT_DNS_DOMAIN=
#export S3_BUCKET_NAME=
#export S3_REGION=

# HOSTING_CLUSTER_NAME is the target managed cluster where the hosted cluster is created. 
# The hypershift-addon must be enabled in this managed cluster.
# defaults to local-cluster if not specified
# export HOSTING_CLUSTER_NAME=local-cluster

# The AWS creds
#export AWS_ACCESS_KEY_ID=
#export AWS_SECRET_ACCESS_KEY=

# The name of the secret that will be created on the hosting cluster
# if the secret doesn't yet exist, then it will be created and will require base domain and ssh keys variables set
#export SECRET_AWS_CRED_NAME=
#export BASE_DOMAIN=
#export SSH_PUBLIC_KEY=
#export SSH_PRIVATE_KEY=
#########################################

TIMEOUT=300s # default: 5 minute timeout for oc wait commands

cleanup() {
  rm -rf ./.aws/credentials
}
# Delete all aws credentials in the current directory on any exit
trap cleanup EXIT

if [ -z ${HOSTING_CLUSTER_NAME+x} ]; then
  echo "WARN: HOSTING_CLUSTER_NAME is not defined, defaulting to local-cluster"
  HOSTING_CLUSTER_NAME="local-cluster"
fi

if [ -z ${SECRET_AWS_CRED_NAME+x} ]; then
  echo "ERROR: SECRET_AWS_CRED_NAME is not defined"
  exit 1
fi

if [ -z ${S3_BUCKET_NAME+x} ]; then
  echo "ERROR: S3_BUCKET_NAME is not defined"
  exit 1
fi

if [ -z ${S3_REGION+x} ]; then
  echo "ERROR: S3_REGION is not defined"
  exit 1
fi

if [ -z ${EXT_DNS_DOMAIN+x} ]; then
  echo "ERROR: EXT_DNS_DOMAIN is not defined"
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

mkdir ./.aws/
cat <<EOF >./.aws/credentials
[default]
aws_access_key_id=${AWS_ACCESS_KEY_ID}
aws_secret_access_key=${AWS_SECRET_ACCESS_KEY}
EOF

AWS_CREDS_FILE=./.aws/credentials

# grab the pull secret from the hub and use it for the hosted cluster
PULL_SECRET=$(oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d)

#######################################################
# Create secrets for hypershift operator installation
#######################################################
echo "$(date) creating secret hypershift-operator-oidc-provider-s3-credentials..."
oc delete secret hypershift-operator-oidc-provider-s3-credentials --ignore-not-found -n ${HOSTING_CLUSTER_NAME}
oc create secret generic hypershift-operator-oidc-provider-s3-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=bucket=${S3_BUCKET_NAME} --from-literal=region=${S3_REGION} -n ${HOSTING_CLUSTER_NAME}
oc label secret hypershift-operator-oidc-provider-s3-credentials -n ${HOSTING_CLUSTER_NAME} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
    echo "$(date) failed to create secret hypershift-operator-oidc-provider-s3-credentials"
    exit 1
fi
echo

echo "$(date) creating secret hypershift-operator-external-dns-credentials..."
oc delete secret hypershift-operator-external-dns-credentials --ignore-not-found -n ${HOSTING_CLUSTER_NAME}
oc create secret generic hypershift-operator-external-dns-credentials --from-file=credentials=${AWS_CREDS_FILE} --from-literal=provider=aws --from-literal=domain-filter=${EXT_DNS_DOMAIN} -n ${HOSTING_CLUSTER_NAME}
oc label secret hypershift-operator-external-dns-credentials -n ${HOSTING_CLUSTER_NAME} cluster.open-cluster-management.io/backup=true --overwrite
if [ $? -ne 0 ]; then
    echo "$(date) failed to create secret hypershift-operator-external-dns-credentials"
    exit 1
fi
echo
#######################################################

#######################################################
# Create mce aws secret on the hosting cluster 
#######################################################
if oc get secret "$SECRET_AWS_CRED_NAME" -n "$HOSTING_CLUSTER_NAME"; then
    echo "Secret $SECRET_AWS_CRED_NAME already exists in $HOSTING_CLUSTER_NAME namespace"
else
    echo "Secret $SECRET_AWS_CRED_NAME does not exist yet in $HOSTING_CLUSTER_NAME namespace"
    echo "Creating new secret $SECRET_AWS_CRED_NAME in $HOSTING_CLUSTER_NAME namespace..."

    if [ -z ${BASE_DOMAIN+x} ]; then
      echo "ERROR: BASE_DOMAIN is not defined"
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

    oc apply -f - << EOF
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: $SECRET_AWS_CRED_NAME
  namespace: $HOSTING_CLUSTER_NAME
  labels:
    cluster.open-cluster-management.io/type: aws
    cluster.open-cluster-management.io/credentials: ""
stringData:
  aws_access_key_id: $AWS_ACCESS_KEY_ID
  aws_secret_access_key: $AWS_SECRET_ACCESS_KEY
  baseDomain: $BASE_DOMAIN
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
fi
if [ $? -ne 0 ]; then
  echo "$(date) failed to create secret aws mce secret"
  exit 1
fi

#######################################################

echo "$(date) Waiting up to ${TIMEOUT} to verify the hosting service cluster is configured with the s3 bucket..."
oc wait configmap/oidc-storage-provider-s3-config -n kube-public --for=jsonpath='{.data.name}'=${S3_BUCKET_NAME} --timeout=${TIMEOUT}
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
        oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o yaml
        exit 1
    fi

    addonAvailable=$(oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
    addonDegraded=$(oc get managedclusteraddon hypershift-addon -n ${HOSTING_CLUSTER_NAME} -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}')

    if [[ ("$addonAvailable" == "True") && ("$addonDegraded" == "False") ]]; then 
        echo "Hypershift addon is available"
        break
    fi
    sleep 10
    (( SECONDS = SECONDS + 10 ))
done