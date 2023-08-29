# acm-hypershift-e2e-ginkgo

Ginkgo e2e test framework for self-managed hosted control plane using MCE / ACM. Currently supports AWS only.

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
