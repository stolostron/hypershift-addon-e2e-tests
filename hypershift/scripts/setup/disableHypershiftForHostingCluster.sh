#!/bin/bash

if [ -z ${HOSTING_CLUSTER_NAME+x} ]; then
  echo "ERROR: HOSTING_CLUSTER_NAME is not defined"
  exit 1
fi

oc delete --ignore-not-found -f - <<EOF
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: hypershift-addon
  namespace: ${HOSTING_CLUSTER_NAME}
spec:
  installNamespace: open-cluster-management-agent-addon
EOF