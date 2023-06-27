#!/bin/bash

# MUST BE LOGGED INTO HUB ALREADY

## TODO: if given arg, then delete only that one, else delete all

# Get a list of all deployments in the namespace
managedclusters=$(oc get managedclusters -o jsonpath='{.items[*].metadata.name}')

if [ -z "$managedclusters" ]; then
  echo "No managed clusters found"
else
  echo "Managed clusters found: $managedclusters"
  # Wait for a maximum of 20 minutes (1200 seconds)
  for managedcluster in $managedclusters; do
    if [ "$managedcluster" == "local-cluster" ]; then
      echo "Skip local-cluster managedcluster deletion..."
      continue
    else
      echo "Deleting $managedcluster then waiting for delete to complete..."
      oc delete managedcluster "$managedcluster" --wait=true --timeout=1200s
      if [ $? -ne 0 ]; then
        echo "Error: time limit reached, managedcluster $managedcluster is not deleted/detached."
        exit 1
      fi
    fi
  done
fi

echo "All managed clusters are deleted, imported clusters should now be detached. oc get managedclusters: "
oc get managedclusters

