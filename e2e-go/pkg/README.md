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

- `CURATOR_ENABLED=true`
- An existing HostedCluster (e.g. `HCP_CLUSTER_NAME` or options)
- Ansible Tower secret for upgrade hooks (`CURATOR_TOWER_SECRET` or default `acmqe-hypershift/ansible-tower-secret`)
- Optional: `HCP_UPGRADE_CHANNEL` (default `fast-4.14`) for the channel to set

**Utils added for PR 511:**

- `utils.SetClusterCuratorUpgradeChannel()` – patch `spec.upgrade.channel`
- `utils.GetHostedClusterChannel()` – read HostedCluster `spec.channel`
- `utils.GetHostedClusterAvailableChannels()` – read `status.version.desired.channels`
