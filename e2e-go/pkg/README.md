# e2e-go/pkg – Test package layout

This package contains the Ginkgo e2e test suite and shared utilities for the Hypershift addon.

## Layout

- **`test/`** – Ginkgo test specs. Suite bootstrap is in `hcp_suite_test.go`; other `*_test.go` files are feature-specific.
- **`utils/`** – Shared helpers (Kube/dynamic clients, ClusterCurator, HostedCluster, MCE/ACM, options).
- **`resources/`** – YAML fixtures and templates (ClusterCurator, options template).

## Running tests

From `e2e-go`:

```bash
# All e2e tests
ginkgo -v --label-filter='e2e' pkg/test

# PR 511 / ACM-26476 – ClusterCurator HostedCluster channel update only
ginkgo -v --label-filter='channel-upgrade' pkg/test

# Control-plane-only upgrade
ginkgo -v --label-filter='control-plane-upgrade' pkg/test

# Nodepool-only upgrade (requires ~30 min; use --timeout=30m)
ginkgo -v --timeout=30m --label-filter='nodepool-upgrade' pkg/test

# Create / destroy (see repo README for env and options)
ginkgo -v --label-filter='create' pkg/test
ginkgo -v --label-filter='destroy' pkg/test
```

## PR 511 (cluster-curator-controller) – Channel upgrade tests

**Label:** `channel-upgrade` (and `PR511`, `ACM-26476`)

Tests for [PR 511](https://github.com/open-cluster-management/cluster-curator-controller/pull/511) (ACM-26476): HostedCluster channel setting without a version upgrade.

- **`hcp_channel_upgrade_test.go`**
  - **Channel-only update:** Set `spec.upgrade.channel` and `desiredCuration: upgrade`, then assert HostedCluster `spec.channel` and ClusterCurator `hypershift-upgrade-job` condition.
  - **Available channels:** Reads `status.version.desired.channels` (used by the controller for channel validation).

**Requirements:**

- An existing HostedCluster (e.g. `HCP_CLUSTER_NAME` or options)
- Ansible Tower secret for upgrade hooks (`CURATOR_TOWER_SECRET` or default `acmqe-hypershift/ansible-tower-secret`)
- Optional: `HCP_UPGRADE_CHANNEL` (default `fast-4.14`) for the channel to set

**Utils added for PR 511:**

- `utils.SetClusterCuratorUpgradeChannel()` – patch `spec.upgrade.channel`
- `utils.GetHostedClusterChannel()` – read HostedCluster `spec.channel`
- `utils.GetHostedClusterAvailableChannels()` – read `status.version.desired.channels`

## Control-plane-only upgrade tests

**Label:** `control-plane-upgrade`

ClusterCurator upgrade of HostedCluster control plane only (or NodePools only), without upgrading both.

**Inputs (same as channel-upgrade plus upgrade type and desiredUpdate):**

- Existing HostedCluster: `HCP_CLUSTER_NAME` or `options.clusters.aws.clusterName`
- `HCP_NAMESPACE` or default `clusters`
- Target channel: `HCP_UPGRADE_CHANNEL` or `options.clustercurator.channel`
- **Desired update (required):** `HCP_UPGRADE_DESIRED_UPDATE` or `options.clustercurator.desiredUpdate` — target OCP version (e.g. `4.19.22`); maps to `spec.upgrade.desiredUpdate`. The controller requires this for control-plane upgrade and will panic if it is empty.
- **Upgrade type:** `HCP_UPGRADE_TYPE` or `options.clustercurator.upgradeType` — `ControlPlane` (control plane only), `NodePools` (node pools only), or empty for both

**Utils:**

- `utils.GetClusterCuratorUpgradeType()` – returns upgrade type from env or options
- `utils.GetClusterCuratorDesiredUpdate()` – returns desired update version from env or options

## Nodepool-only upgrade tests

**Label:** `nodepool-upgrade`

ClusterCurator upgrade of NodePools (worker nodes) only, without changing the HostedCluster control plane. Channel is ignored for NodePools upgrades.

**Inputs:**

- Existing HostedCluster with at least one NodePool: `HCP_CLUSTER_NAME` or `options.clusters.aws.clusterName`
- `HCP_NAMESPACE` or default `clusters`
- **Desired update (required):** `HCP_UPGRADE_DESIRED_UPDATE` or `options.clustercurator.desiredUpdate` — target OCP version (e.g. `4.19.22`)
- **Upgrade type:** `HCP_UPGRADE_TYPE` or `options.clustercurator.upgradeType` — must be `NodePools` for this test

**Note:** NodePools version cannot exceed the HostedCluster control plane version. Upgrade the control plane first (`control-plane-upgrade`) if needed.

**Duration:** The nodepool upgrade test requires approximately 30 minutes. Use `--timeout=30m` (ginkgo) or `-timeout=30m` (go test).

**Utils:**

- `utils.ListNodePoolsForHostedCluster()` – list NodePools belonging to a HostedCluster
- `utils.GetNodePoolSpecRelease()` – read NodePool `spec.release.image`
