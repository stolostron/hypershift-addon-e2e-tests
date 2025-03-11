#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

ODF_INSTALL_NAMESPACE=openshift-storage
ODF_OPERATOR_CHANNEL="${ODF_OPERATOR_CHANNEL:-'stable-4.17'}"
ODF_SUBSCRIPTION_NAME="${ODF_SUBSCRIPTION_NAME:-'odf-operator'}"
ODF_BACKEND_STORAGE_CLASS="${ODF_BACKEND_STORAGE_CLASS:-'gp3-csi'}"
ODF_VOLUME_SIZE="${ODF_VOLUME_SIZE:-100}Gi"
ODF_SUBSCRIPTION_SOURCE="${ODF_SUBSCRIPTION_SOURCE:-'redhat-operators'}"

# Make the masters schedulable so we have more capacity to run VMs
oc patch scheduler cluster --type=json -p '[{ "op": "replace", "path": "/spec/mastersSchedulable", "value": true }]'

# create the install namespace
oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-storage
EOF


# deploy new operator group
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  annotations:
    olm.providedAPIs: BackingStore.v1alpha1.noobaa.io,BucketClass.v1alpha1.noobaa.io,CSIAddonsNode.v1alpha1.csiaddons.openshift.io,NamespaceStore.v1alpha1.noobaa.io,NetworkFence.v1alpha1.csiaddons.openshift.io,NooBaa.v1alpha1.noobaa.io,NooBaaAccount.v1alpha1.noobaa.io,ObjectBucket.v1alpha1.objectbucket.io,ObjectBucketClaim.v1alpha1.objectbucket.io,ReclaimSpaceCronJob.v1alpha1.csiaddons.openshift.io,ReclaimSpaceJob.v1alpha1.csiaddons.openshift.io,StorageSystem.v1alpha1.odf.openshift.io,VolumeReplication.v1alpha1.replication.storage.openshift.io,VolumeReplicationClass.v1alpha1.replication.storage.openshift.io
  name: openshift-storage-odf-operator
  namespace: openshift-storage
spec:
  targetNamespaces:
  - openshift-storage
  upgradeStrategy: Default
EOF

oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: odf-operator
  namespace: openshift-storage
spec:
  channel: stable-4.17
  installPlanApproval: Automatic
  name: odf-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF


RETRIES=60
echo "Waiting for CSV to be available from operator group"
for ((i=1; i <= $RETRIES; i++)); do
    CSV=$(oc -n "$ODF_INSTALL_NAMESPACE" get subscription.operators.coreos.com odf-operator -o jsonpath='{.status.installedCSV}' || true)
    if [[ -n "$CSV" ]]; then
        if [[ "$(oc -n "$ODF_INSTALL_NAMESPACE" get csv "$CSV" -o jsonpath='{.status.phase}')" == "Succeeded" ]]; then
            echo "ClusterServiceVersion \"$CSV\" ready"
            break
	else
	   oc -n "$ODF_INSTALL_NAMESPACE" get csv "$CSV" -o yaml --ignore-not-found
        fi
    else
      echo "Try ${i}/${RETRIES}: ODF is not deployed yet. Checking again in 10 seconds"
      oc -n "$ODF_INSTALL_NAMESPACE" get subscription.operators.coreos.com odf-operator -o yaml --ignore-not-found
    fi
    sleep 10
done

echo "Waiting for noobaa-operator"
for ((i=1; i <= $RETRIES; i++)); do
    NOOBAA=$(oc -n "$ODF_INSTALL_NAMESPACE" get deployment noobaa-operator --ignore-not-found)
    if [[ -n "$NOOBAA" ]]; then
       echo "Found noobaa operator"
       break
    fi
    sleep 10
done

oc wait deployment noobaa-operator \
--namespace="${ODF_INSTALL_NAMESPACE}" \
--for=condition='Available' \
--timeout='5m'

echo "Preparing nodes"
oc label nodes cluster.ocs.openshift.io/openshift-storage='' \
  --selector='node-role.kubernetes.io/worker' --overwrite

echo "Create StorageCluster"
cat <<EOF | oc apply -f -
kind: StorageCluster
apiVersion: ocs.openshift.io/v1
metadata:
 name: ocs-storagecluster
 namespace: openshift-storage
spec:
  resources:
    mon:
      requests:
        cpu: "0"
        memory: "0"
    mgr:
      requests:
        cpu: "0"
        memory: "0"
  monDataDirHostPath: /var/lib/rook
  managedResources:
    cephFilesystems:
      reconcileStrategy: ignore
    cephObjectStores:
      reconcileStrategy: ignore
  multiCloudGateway:
    reconcileStrategy: ignore
  storageDeviceSets:
    - name: ocs-deviceset
      count: 6
      dataPVCTemplate:
        spec:
          storageClassName: gp3-csi
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 100Gi
          volumeMode: Block
      placement: {}
      portable: false
      replica: 1
      resources:
        requests:
          cpu: "0"
          memory: "0"
EOF

echo "Wait for StorageCluster to be deployed"
oc wait "storagecluster.ocs.openshift.io/ocs-storagecluster"  \
   -n $ODF_INSTALL_NAMESPACE --for=condition='Available' --timeout='10m'

echo "ODF/OCS Operator is deployed successfully"

# Setting ocs-storagecluster-ceph-rbd the default storage class
for item in $(oc get sc --no-headers | awk '{print $1}'); do
	oc annotate --overwrite sc $item storageclass.kubernetes.io/is-default-class='false'
done
oc annotate --overwrite sc ocs-storagecluster-ceph-rbd storageclass.kubernetes.io/is-default-class='true'
echo "ocs-storagecluster-ceph-rbd is set as default storage class"


oc patch ingresscontroller -n openshift-ingress-operator default --type=json -p '[{ "op": "add", "path": "/spec/routeAdmission", "value": {wildcardPolicy: "WildcardsAllowed"}}]'

oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-cnv
EOF

oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: openshift-cnv-group
  namespace: openshift-cnv
spec:
  targetNamespaces:
  - openshift-cnv
EOF

cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  labels:
    operators.coreos.com/kubevirt-hyperconverged.openshift-cnv: ''
  name: kubevirt-hyperconverged
  namespace: openshift-cnv
spec:
  channel: stable
  installPlanApproval: Automatic
  name: kubevirt-hyperconverged
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

sleep 30

RETRIES=30
CSV=
for i in $(seq ${RETRIES}); do
  if [[ -z ${CSV} ]]; then
    CSV=$(oc get subscription.operators.coreos.com -n openshift-cnv kubevirt-hyperconverged -o jsonpath='{.status.installedCSV}')
  fi
  if [[ -z ${CSV} ]]; then
    echo "Try ${i}/${RETRIES}: can't get the CSV yet. Checking again in 30 seconds"
    sleep 30
  fi
  if [[ $(oc get csv -n openshift-cnv ${CSV} -o jsonpath='{.status.phase}') == "Succeeded" ]]; then
    echo "CNV is deployed"
    break
  else
    echo "Try ${i}/${RETRIES}: CNV is not deployed yet. Checking again in 30 seconds"
    sleep 30
  fi
done

if [[ $(oc get csv -n openshift-cnv ${CSV} -o jsonpath='{.status.phase}') != "Succeeded" ]]; then
  echo "Error: Failed to deploy CNV"
  echo "CSV ${CSV} YAML"
  oc get CSV ${CSV} -n openshift-cnv -o yaml
  echo
  echo "CSV ${CSV} Describe"
  oc describe CSV ${CSV} -n openshift-cnv
  exit 1
fi

# Deploy HyperConverged custom resource to complete kubevirt's installation
oc apply -f - <<EOF
apiVersion: hco.kubevirt.io/v1beta1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
  namespace: openshift-cnv
spec:
  featureGates:
    enableCommonBootImageImport: false
  logVerbosityConfig:
    kubevirt:
      virtLauncher: 8
      virtHandler: 8
      virtController: 8
      virtApi: 8
      virtOperator: 8
EOF

oc wait hyperconverged -n openshift-cnv kubevirt-hyperconverged --for=condition=Available --timeout=15m