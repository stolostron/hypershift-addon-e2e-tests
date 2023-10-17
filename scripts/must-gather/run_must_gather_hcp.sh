#!/bin/bash
#
# Triggers must-gather on a hosted cluster on the hub. If no hosted cluster is provided, then a random one will be chosen on the hub.
#
# PRE-REQUISITES
# 1. oc cli is installed on the system
# 2. oc is logged into the hub cluster
# 3. MUST_GATHER_IMAGE is set prior to running

#########################################
#   POPULATE THESE WITH ENV VARS        #
#   ie: export OCP_RELEASE_IMAG=foobar  #
#########################################
#export MUST_GATHER_HCP=
#export MUST_GATHER_HCP_NS=
#export MUST_GATHER_IMAGE=quay.io/stolostron/must-gather:2.9.0-SNAPSHOT-2023-10-10-18-05-35

MUST_GATHER_DIR=../../results/must-gather/

if [ -z ${MUST_GATHER_IMAGE+x} ]; then
  echo "ERROR: MUST_GATHER_IMAGE is not defined"
  exit 1
fi

if [ -z ${MUST_GATHER_HCP_NS+x} ]; then
  echo "WARN: MUST_GATHER_HCP_NS is not defined, defaulting to clusters"
  MUST_GATHER_HCP_NS=clusters
fi

# TODO: if no hc is found, then exit
# TODO: if hc is not provided, then choose a random one on the hub. also set ns

oc adm must-gather --image="${MUST_GATHER_IMAGE}" /usr/bin/gather hosted-cluster-namespace=:"${MUST_GATHER_HCP_NS}" hosted-cluster-name="${MUST_GATHER_HCP}" --dest-dir=${MUST_GATHER_DIR}
cd ${MUST_GATHER_DIR} || exit 1

MUST_GATHER_LOGS_DIR=$(ls -d */)
cd "${MUST_GATHER_LOGS_DIR}" || exit 1

## check if hosted cluster related files exist
# check if directory for the hosted cluster exists
HS_DIR=hostedcluster-"${MUST_GATHER_HCP}"
if [ ! -d "$HS_DIR" ]; then
  echo "Directory not found!"
  exit 1
else
  echo "Directory found"
fi

# check if the hypershift-dump.tar.gz exists
HS_FILE=hypershift-dump.tar.gz
if [ ! -f "$HS_FILE" ]; then
  echo "File not found!"
  exit 1
else
  echo "File found"
fi

