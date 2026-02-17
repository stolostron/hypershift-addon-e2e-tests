# E2E Test Coverage

This document describes **what is tested** when you run the Hypershift addon e2e suite (`e2e-go/pkg/test`) and how to run subsets by label.

---

## What “run all e2e tests” means

- **No label filter** (e.g. `ginkgo -v pkg/test`): runs **every test** in the suite. That includes create, destroy, console/CLI, metrics, must-gather, S3 secret, and channel-upgrade tests. Order is not guaranteed; create/destroy can conflict if they share clusters.
- **With label filter** (e.g. `ginkgo -v --label-filter='e2e' pkg/test`): runs only specs that have **at least one** of the given labels. Use this to run a subset (e.g. only “e2e” checks, or only create, or only destroy).

Recommended for CI or full validation:

- Run **create** and **destroy** in separate stages (as in the Jenkinsfile).
- Run **e2e** (and optionally **metrics**) after clusters exist.

---

## Suite bootstrap (`hcp_suite_test.go`)

- **Not a test** itself; it registers the “Hypershift E2e Suite” and runs `SynchronizedBeforeSuite` once.
- **BeforeSuite** checks/does:
  - Kubeconfig, dynamic/kube/route/addon clients, MCE namespace.
  - Hypershift addon manager and addon availability.
  - Hypershift CLI version, OIDC S3 secret (AWS), hypershift operator health.
  - ConsoleCLIDownload for `hcp` CLI.
  - Loads config (instance type, base domain, region, node pool replicas, release image, namespace, pull secret, AWS creds, curator enabled, FIPS enabled).

Environment variables that affect the suite (see also README):

- `KUBECONFIG`, `MANAGED_CLUSTER_NAME`, `HCP_CLUSTER_NAME`, `HCP_NAMESPACE`, `HCP_REGION`, `HCP_NODE_POOL_REPLICAS`, `HCP_BASE_DOMAIN_NAME`, `HCP_RELEASE_IMAGE`, `HCP_INSTANCE_TYPE`, `AWS_CREDS`, `PULL_SECRET_FILE` / `PULL_SECRET`, `JUNIT_REPORT_FILE`, options file, etc.

---

## Tests by file and label

Each block below is one **Describe**; indented items are **It** (individual test cases). Labels on the Describe apply to all Its inside it unless overridden.

---

### 1. `hcp_aws_create_test.go`

**Describe:** Hosted Control Plane CLI AWS Create Tests  
**Labels:** `AWS`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Creates a FIPS AWS Hosted Cluster using STS Creds | `create` | Uses `hcp` CLI to create an AWS hosted cluster (STS, FIPS if enabled, optional `pausedUntil` when curator enabled). Waits for cluster to become available and optionally checks addon. |

**When you run “all” tests:** This runs if no label filter (and will create a cluster).  
**Run only this:** `--label-filter='create && AWS'` (or `create` if only AWS is present).

---

### 2. `hcp_aws_destroy_test.go`

**Describe:** Hosted Control Plane CLI AWS Destroy Tests  
**Labels:** `AWS`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Destroy all AWS hosted clusters on the hub | `destroy` | Destroys all AWS hosted clusters found on the hub (via CLI). |
| Destroy a AWS hosted cluster on the hub | `destroy-one` | Destroys a single AWS hosted cluster (name/namespace from options/env). |

**When you run “all” tests:** Both destroy tests run.  
**Run only destroy:** `--label-filter='destroy'` or `destroy-one`.

---

### 3. `hcp_kubevirt_create_test.go`

**Describe:** Hosted Control Plane CLI KubeVirt Create Tests  
**Labels:** `KubeVirt`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Creates a Kubevirt Hosted Cluster | `create` | Uses `hcp` CLI to create a KubeVirt hosted cluster. |

**When you run “all” tests:** This runs if no label filter.  
**Run only this:** `--label-filter='create && KubeVirt'`.

---

### 4. `hcp_kubevirt_destroy_test.go`

**Describe:** Hosted Control Plane CLI KubeVirt Destroy Tests  
**Labels:** `KubeVirt`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Destroy all KubeVirt hosted clusters on the hub | `destroy` | Destroys all KubeVirt hosted clusters on the hub. |
| Destroy a KubeVirt hosted cluster on the hub | `destroy-one` | Destroys one KubeVirt hosted cluster. |

**When you run “all” tests:** Both run.  
**Run only destroy:** `--label-filter='destroy'` (or combine with `KubeVirt`).

---

### 5. `hcp_common_cli_route_test.go`

**Describe:** Hosted Control Plane CLI Binary Tests  
**Labels:** `@e2e`, `CLI-Links`, `AWS`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| should no longer have the old hypershift console link reference | `e2e`, `label`, `console` | Old hypershift console link is not present. |
| should have the correct ConsoleCLIDownload display name set for hcp | `e2e`, `label`, `console` | ConsoleCLIDownload display name for `hcp` CLI. |
| should have the correct ConsoleCLIDownload description set for hcp | `e2e`, `label`, `console` | ConsoleCLIDownload description for `hcp` CLI. |
| should have the correct link for Linux x86_64 | `e2e`, `label`, `consoleLinks` | Download URL for Linux x86_64. |
| should have the correct link for Linux ARM 64 | `e2e`, `label`, `consoleLinks` | Download URL for Linux ARM64. |
| should have the correct link for Mac x86_64 | `e2e`, `label`, `consoleLinks` | Download URL for Mac x86_64. |
| should have the correct link for Mac ARM 64 | `e2e`, `label`, `consoleLinks` | Download URL for Mac ARM64. |
| should have the correct link for Windows x86_64 | `e2e`, `label`, `consoleLinks` | Download URL for Windows x86_64. |
| should have the correct link for IBM Power (PPC64) | `e2e`, `consoleLinks` | Download URL for PPC64. |
| should have the correct link for IBM Power, little endian (PPC64LE) | `e2e`, `consoleLinks` | Download URL for PPC64LE. |
| should have the correct link for IBM Z (S390X) | `e2e`, `consoleLinks` | Download URL for S390X. |

**When you run “all” tests:** All of these run.  
**Run only CLI/console tests:** `--label-filter='@e2e'` or `--label-filter='console'` (or `consoleLinks`).

---

### 6. `hcp_metrics_test.go`

**Describe:** Hypershift Add-on Prometheus/Metrics Tests  
**Labels:** `metrics`, `@e2e`, `@post-upgrade`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| RHACM4K-39628: ServiceMonitor correctly deployed for Prometheus metrics | `service_monitor` | ServiceMonitor for hypershift addon metrics in the expected namespaces (MCE/ACM). |
| RHACM4K-39627: Retrieve/observe Prometheus metrics for hypershift add-on health | `metrics`, `sanity` | Queries Prometheus for addon health metrics. |
| RHACM4K-39474: Retrieve/observe Prometheus metrics for hosted clusters | `metrics` | Prometheus metrics for hosted clusters. |
| RHACM4K-39627: Retrieve/observe Prometheus metrics for capacity | `metrics`, `capacity`, `sanity` | Capacity-related Prometheus metrics. |

**When you run “all” tests:** All four metrics tests run (Prometheus must be available).  
**Run only metrics:** `--label-filter='metrics'` or `@e2e` (to include this Describe).

---

### 7. `hcp_must_gather_test.go`

**Describe:** Hypershift Add-on Must-Gather Tests  
**Labels:** `@must-gather`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Triggers must-gather on a particular hosted cluster | (none) | Runs the must-gather script `scripts/must-gather/run_must_gather_hcp.sh` for a hosted cluster. |

**When you run “all” tests:** This runs.  
**Run only must-gather:** `--label-filter='@must-gather'`.

---

### 8. `hcp_rhacm4k-21843_test.go`

**Describe:** RHACM4K-21843: Hypershift Addon should detect changes in S3 secret and re-install the hypershift operator  
**Labels:** `e2e`, `@non-ui`, `RHACM4K-21843`, `AWS`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Get, modify, and verify the s3 secret | (none) | Gets current hypershift install pod, updates the OIDC S3 secret, then verifies a new hypershift-install pod is created (addon reacts to S3 secret change). Restores secret in AfterEach. |

**When you run “all” tests:** This runs.  
**Run only this:** `--label-filter='e2e'` (or `RHACM4K-21843`).

---

### 9. `hcp_channel_upgrade_test.go`

**Describe:** PR 511 / ACM-26476: ClusterCurator HostedCluster channel update  
**Labels:** `e2e`, `channel-upgrade`, `PR511`, `ACM-26476`, `AWS`

| Test (It) | Labels | What is tested |
|-----------|--------|----------------|
| Channel-only update: set spec.upgrade.channel and desiredCuration upgrade, then verify HostedCluster spec.channel and curator condition | (none) | Creates/updates ClusterCurator with `spec.upgrade.channel`, sets `desiredCuration: upgrade`, waits for `hypershift-upgrade-job` condition, and asserts HostedCluster `spec.channel` (cluster-curator-controller PR 511). |
| Available channels: HostedCluster status.version.desired.channels can be read for validation | (none) | Reads `status.version.desired.channels` from HostedCluster (validation path used by PR 511). |

**When you run “all” tests:** Both run (skipped if `CURATOR_ENABLED != true`).  
**Run only channel-upgrade:** `--label-filter='channel-upgrade'` or `e2e` to include this Describe.

---

## Summary: what runs when

| Command | What runs |
|---------|-----------|
| `ginkgo -v pkg/test` | **Everything**: suite bootstrap + all Describes above (create, destroy, CLI, metrics, must-gather, S3 secret, channel-upgrade). |
| `ginkgo -v --label-filter='create' pkg/test` | Only create Its (AWS and/or KubeVirt depending on labels). |
| `ginkgo -v --label-filter='destroy' pkg/test` | Only destroy Its (all AWS, all KubeVirt, destroy-one for each). |
| `ginkgo -v --label-filter='e2e' pkg/test` | Specs that have label `e2e`: channel-upgrade, RHACM4K-21843. (Note: `@e2e` is a different label; metrics and CLI use `@e2e`.) |
| `ginkgo -v --label-filter='@e2e' pkg/test` | Specs with `@e2e`: CLI Binary Tests, Metrics Tests. |
| `ginkgo -v --label-filter='channel-upgrade' pkg/test` | Only PR 511 / ACM-26476 channel-upgrade tests. |
| `ginkgo -v --label-filter='metrics' pkg/test` | Only Prometheus/metrics tests. |
| `ginkgo -v --label-filter='@must-gather' pkg/test` | Only must-gather test. |
| `ginkgo -v --label-filter='AWS' pkg/test` | All AWS-related specs: create, destroy, CLI, RHACM4K-21843, channel-upgrade. |
| `ginkgo -v --label-filter='KubeVirt' pkg/test` | All KubeVirt create/destroy specs. |

---

## Labels reference

- **Platform:** `AWS`, `KubeVirt`
- **Lifecycle:** `create`, `destroy`, `destroy-one`
- **E2E / feature:** `e2e`, `@e2e`, `channel-upgrade`, `PR511`, `ACM-26476`, `metrics`, `@must-gather`, `CLI-Links`, `@non-ui`, `@post-upgrade`
- **Sub-feature:** `console`, `consoleLinks`, `service_monitor`, `sanity`, `capacity`

Combining labels (Ginkgo):

- `--label-filter='create && AWS'` — create and AWS
- `--label-filter='destroy || create'` — any destroy or create (OR)

---

## See also

- **Run instructions and env:** [README.md](../README.md)
- **Package layout and PR 511:** [pkg/README.md](../pkg/README.md)
