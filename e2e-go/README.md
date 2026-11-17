# acm-hypershift-e2e-ginkgo

Ginkgo e2e test framework for self-managed hosted control plane using MCE / ACM. Currently supports AWS only.

## HCP proxy integration (ACM-38016)

The `hcp-proxy` label runs hub APIService requests through cluster-proxy and
checks the resulting resources on the hosting cluster. These specs target MCE/ACM
5.1 or newer because the finalizers, version, and validate subresources were
introduced in 5.1. Run this label only on a hub with the HCP proxy APIService,
an Available `cluster-proxy` ManagedClusterAddOn, and Hypershift HostedCluster
and NodePool CRDs on the hosting cluster. The hub kubeconfig identity must have
`managedcluster:admin` access to the hosting cluster.

```bash
export KUBECONFIG=/path/to/hub-kubeconfig
export HCP_PROXY_HOSTING_CLUSTER=local-cluster
# Required when the hosting cluster is separate from the hub:
export HCP_PROXY_SPOKE_KUBECONFIG=/path/to/hosting-cluster-kubeconfig
ginkgo -v --label-filter='hcp-proxy' pkg/test
```

Individual stories can be run with their Jira labels (`ACM-37268`,
`ACM-37269`, `ACM-39216`, `ACM-39226`, `ACM-39227`, `ACM-39228`, or
`ACM-42875`), for example:

```bash
ginkgo -v --label-filter='ACM-39216' pkg/test
```

When the label filter selects only `hcp-proxy` specs, the suite uses a minimal
proxy-only bootstrap (kubeconfig, API clients, cluster-proxy availability, and an
optional options-file release image). It skips cleanly if no hub kubeconfig is
available. If the filter also selects any non-proxy spec, the normal full-suite
AWS/options bootstrap still runs.

The built-in request uses a `None` platform HostedCluster, managed etcd with an
8Gi PersistentVolume, and a zero-replica NodePool, so it tests proxy resource
handling without provisioning cloud infrastructure. Pre-create validation is a
caller flow: the suite calls the read-only `GET .../validate` endpoint before a
successful POST, and stops before POST when validation reports a duplicate name
or incompatible architecture. Create specs require a release image from
`HCP_PROXY_RELEASE_IMAGE`, the existing options file, or a full request template;
otherwise they skip with an actionable reason. Set `HCP_PROXY_RELEASE_IMAGE` to
override the suite release image. For environments that require a different valid
create payload, set `HCP_PROXY_CREATE_REQUEST_TEMPLATE` to a JSON CreateRequest
file. The test replaces `{{NAMESPACE}}`, `{{NAME}}`, and `{{RELEASE_IMAGE}}` and
supplies its own `extraObjects`. The template must name its HostedCluster
`{{NAME}}` and include one NodePool named `{{NAME}}-workers` so round-trip and
cleanup checks can find it.

The missing `supported-versions` ConfigMap failure test is destructive to a shared
hosting-cluster resource. It is skipped unless
`HCP_PROXY_TEST_MISSING_VERSION_CONFIGMAP=true` is explicitly set; run it only on a
dedicated hosting cluster.

**What is tested when you run the suite:** see [docs/E2E_TEST_COVERAGE.md](docs/E2E_TEST_COVERAGE.md) for a full list of tests, labels, and how to run subsets.

1. Ensure go is installed
2. Set env variables (if not set, will default to options.yaml)

    ```bash
    export KUBECONFIG=~/.kube/config
    export HCP_CLUSTER_NAME=
    export HCP_NAMESPACE=local-cluster
    export HCP_REGION=us-east-1
    export HCP_NODE_POOL_REPLICAS=2
    export HCP_BASE_DOMAIN_NAME=dev09.red-chesterfield.com
    export HCP_RELEASE_IMAGE=quay.io/openshift-release-dev/ocp-release:4.14.0-ec.4-multi
    export HCP_INSTANCE_TYPE=m6a.xlarge
    export AWS_CREDS=~/aws/aws
    export PULL_SECRET_FILE=~/pull-secret.txt
    export JUNIT_REPORT_FILE=./results/result.xml
    ```

    - `KUBECONFIG`: must be set, or else default ~/.kube/config
    - `HCP_CLUSTER_NAME` (optional): used to destroy or do e2e on a specific cluster, will generate random name for creation
    - `HCP_NAMESPACE`(optional): used to create HCP
    - `HCP_REGION`(optional): used to create HCP
    - `HCP_NODE_POOL_REPLICAS`(optional): used to create HCP
    - `HCP_BASE_DOMAIN_NAME`(optional): used to create HCP
    - `HCP_RELEASE_IMAGE`(optional): used to create HCP
    - `HCP_INSTANCE_TYPE`(optional): used to create HCP
    - `AWS_CREDS`(required): path to AWS credentials to create HCP
    - `PULL_SECRET_FILE`(required): path to pull secret to create HCP
    - `JUNIT_REPORT_FILE`(optional): path to file where you want to save the junit report

3. (Optional) Fill in options.yaml (if options.yaml missing, will fail)
    - Copy resources/options_template.yaml to resources/options.yaml
    - Fill in options.yaml as you desire
4. to create hcp:

    ```bash
    ginkgo -v --label-filter='create' pkg/test  
    ```

5. to destroy hcp:

    ```bash
    ginkgo -v --label-filter='destroy' pkg/test  
    ```

6. to run e2e with parallelism enabled:

    ```bash
    ginkgo -v -p -nodes=2--label-filter='e2e' pkg/test  
    ```

    where:
    `-nodes` is set to number of parallel threads to run

7. to run PR 511 / ACM-26476 channel-upgrade tests (ClusterCurator HostedCluster channel update):

    ```bash
    ginkgo -v --label-filter='channel-upgrade' pkg/test
    ```

    See `pkg/README.md` for layout and PR 511 test requirements.

8. to run control-plane-upgrade tests (ClusterCurator HostedCluster control plane upgrade only):

    **Required / optional inputs** (same pattern as channel-upgrade, plus upgrade type and desiredUpdate):

    - **Options file** (`resources/options.yaml` or path passed via `--options`):
      - `options.clustercurator.channel`: target channel for the upgrade (e.g. `fast-4.19`).
      - `options.clustercurator.upgradeType`: `ControlPlane` (control plane only), `NodePools` (node pools only), or omit/empty for both.
      - `options.clustercurator.desiredUpdate`: target OCP version for the upgrade (e.g. `4.19.22`); maps to ClusterCurator `spec.upgrade.desiredUpdate`.
    - **Environment variables** (override options when set):
      - `KUBECONFIG`: hub kubeconfig (required).
      - `HCP_CLUSTER_NAME`: existing HostedCluster name to upgrade (or set `options.clusters.aws.clusterName`).
      - `HCP_NAMESPACE`: namespace of the HostedCluster (default `clusters`).
      - `HCP_UPGRADE_CHANNEL`: target channel (overrides `options.clustercurator.channel`).
      - `HCP_UPGRADE_TYPE`: `ControlPlane`, `NodePools`, or unset for both (overrides `options.clustercurator.upgradeType`).
      - `HCP_UPGRADE_DESIRED_UPDATE`: target OCP version (e.g. `4.19.22`) (overrides `options.clustercurator.desiredUpdate`). **Required** for control-plane upgrade—the controller panics if this is empty.

    Example (control-plane-only upgrade to a specific version):

    ```bash
    export HCP_CLUSTER_NAME=my-hosted-cluster
    export HCP_UPGRADE_CHANNEL=fast-4.19
    export HCP_UPGRADE_TYPE=ControlPlane
    export HCP_UPGRADE_DESIRED_UPDATE=4.19.22
    ginkgo -v --label-filter='control-plane-upgrade' pkg/test
    ```

    Or use `options.yaml` under `options.clustercurator`: set `channel`, `upgradeType: 'ControlPlane'`, and `desiredUpdate: '4.19.22'` and omit the env vars.

9. to run nodepool-upgrade tests (ClusterCurator NodePools-only upgrade):

    **Required inputs** (desiredUpdate and upgradeType; channel is ignored for NodePools):

    - **Options file** (`resources/options.yaml` or path passed via `--options`):
      - `options.clustercurator.upgradeType`: `NodePools` (node pools only).
      - `options.clustercurator.desiredUpdate`: target OCP version for the upgrade (e.g. `4.19.22`); maps to ClusterCurator `spec.upgrade.desiredUpdate`.
    - **Environment variables** (override options when set):
      - `KUBECONFIG`: hub kubeconfig (required).
      - `HCP_CLUSTER_NAME`: existing HostedCluster name to upgrade (or set `options.clusters.aws.clusterName`).
      - `HCP_NAMESPACE`: namespace of the HostedCluster (default `clusters`).
      - `HCP_UPGRADE_TYPE`: `NodePools` (overrides `options.clustercurator.upgradeType`).
      - `HCP_UPGRADE_DESIRED_UPDATE`: target OCP version (e.g. `4.19.22`) (overrides `options.clustercurator.desiredUpdate`). **Required** for nodepool upgrade.

    **Note:** NodePools version cannot exceed the HostedCluster control plane version. Upgrade the control plane first (`control-plane-upgrade`) if needed.

    **Duration:** The nodepool upgrade test requires approximately 30 minutes. Use `--timeout=30m` (ginkgo) or `-timeout=30m` (go test) to avoid suite timeout.

    Example (nodepool-only upgrade):

    ```bash
    export HCP_CLUSTER_NAME=my-hosted-cluster
    export HCP_UPGRADE_TYPE=NodePools
    export HCP_UPGRADE_DESIRED_UPDATE=4.19.22
    ginkgo -v --timeout=30m --label-filter='nodepool-upgrade' pkg/test
    ```

    Or use `options.yaml` under `options.clustercurator`: set `upgradeType: 'NodePools'` and `desiredUpdate: '4.19.22'` and omit the env vars.
