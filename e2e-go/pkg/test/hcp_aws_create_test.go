package hypershift_test

import (
	"fmt"
	"os/exec"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = ginkgo.Describe("Create AWS hosted cluster", ginkgo.Label("e2e", "create", TYPE_AWS), func() {
	var config Config
	ginkgo.BeforeEach(func() {
		// get the variables from our options file
		// TODO update all util methods to try to pull from env vars first if they exist
		// TODO check s3 exists, if not overwrite

		// GetClusterName with error handling
		clusterName, err := utils.GenerateClusterName("acmqe-hc")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.ClusterName = clusterName

		// GetInstanceType with error handling
		instanceType, err := utils.GetInstanceType(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.InstanceType = instanceType

		// GetBaseDomain with error handling
		baseDomain, err := utils.GetBaseDomain(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.BaseDomain = baseDomain

		// GetRegion with error handling
		region, err := utils.GetRegion(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Region = region

		// GetNodePoolReplicas with error handling
		nodePoolReplicas, err := utils.GetNodePoolReplicas(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.NodePoolReplicas = nodePoolReplicas

		// GetReleaseImage with error handling
		releaseImage, err := utils.GetReleaseImage(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.ReleaseImage = releaseImage // TODO allow empty and default to latest release image

		// GetNamespace with error handling
		namespace, err := utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Namespace = namespace // TODO allow empty or default clusters ns

		// GetPullSecret with error handling
		pullSecret, err := utils.GetPullSecret()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.PullSecret = pullSecret

		// GetAWSCreds with error handling
		awsCreds, err := utils.GetAWSCreds()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.AWSCreds = awsCreds
	})

	ginkgo.It("Creates an AWS Hosted Cluster", ginkgo.Label("create", TYPE_AWS), func() {
		startTime := time.Now()
		// TODO ensure auto-import is enabled
		// TODO check disable auto-import, MC not auto created even after HCP is ready

		commandArgs := []string{
			"create", "cluster", TYPE_AWS,
			"--name", config.ClusterName,
			"--aws-creds", config.AWSCreds,
			"--region", config.Region,
			"--base-domain", config.BaseDomain,
			"--pull-secret", config.PullSecret,
			"--node-pool-replicas", config.NodePoolReplicas,
			"--namespace", config.Namespace,
			"--instance-type", config.InstanceType,
			"--release-image", config.ReleaseImage,
			// --external-dns-domain=${HYPERSHIFT_EXTERNAL_DNS_DOMAIN} \
			"--generate-ssh",
		}

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		defer gexec.KillAndWait()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		utils.PrintOutput(session) // prints command, args and output

		ginkgo.By(fmt.Sprintf("Waiting for hosted cluster plane for cluster %s to be available", config.ClusterName), func() {
			utils.WaitForHCPAvailable(dynamicClient, config.ClusterName, config.Namespace)
			fmt.Printf("Time taken for the hosted control plane to be available: %s\n", time.Since(startTime).String())
		})

		// Checks to see if ManagedCluster is created and the HC is auto-imported...
		ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s to be Available", config.ClusterName), func() {
			utils.WaitForClusterImported(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster to be imported: %s\n", time.Since(startTime).String())
		})

		// Checks to see if add-ons are installed and available for the HC managed cluster...
		ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s addons are enabled and Available", config.ClusterName), func() {
			utils.WaitForClusterAddonsAvailable(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster be imported and addons ready: %s\n", time.Since(startTime).String())
		})

		ginkgo.By(fmt.Sprintf("Checking if managed cluster %s has the correct labels", config.ClusterName), func() {
			gomega.Eventually(func() bool {
				managedClusterLabels, err := utils.GetResourceLabels(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("managedClusterLabels: %v\n", managedClusterLabels)
				return managedClusterLabels["name"] == config.ClusterName &&
					managedClusterLabels["cloud"] == "Amazon" &&
					managedClusterLabels["cluster.open-cluster-management.io/clusterset"] == "default" &&
					managedClusterLabels["vendor"] == "OpenShift"
				// UNCOMMENT BELOW ONCE https://issues.redhat.com/browse/ACM-6547 IS DONE
				//&& managedClusterLabels["open-cluster-management/created-via"] == "hypershift"
			}, eventuallyTimeoutShort).Should(gomega.BeTrue())
		})
		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Printf("========================= End Test create hosted cluster %s ===============================", config.ClusterName)

		// TODO
		// next:
		// - jenkins
		// - check annotations on MC
		// - check clusterClaims on MC
		// - check admin kubeconfig / pw is on hub
		// - chceck api url shows up on hub

		// - check NP (Check nodepools are healthy, correct replcas)
		// --- check replicas correct
		// --- check instance type correct

		// HCP CONDITIONS
		// Check if the hosted cluster is created, hcp is ready
		// Available
		// HCP is ready
		// verison deployed it correct
		// OIDC configuration is valid
		// HostedCluster is at expected version
		// Release image is valid
		// message: Payload loaded version="4.13.9" image="quay.io/openshift-release-dev/ocp-release:4.13.9-multi"
		// architecture="Multi"

		// check external dns is correct
		// check operator pods are good, for dns
		// save kubeconfig as secret on the hub? -> already saved. -> export to assets folder?
		// destroy cluster

		// TODO test if s3 bucket exists? can we create HC first and add it later? invalid s3?
		// TODO test auto-import disabled, MC not created, creating MC manually will import addons
		// e2e tests for hs
		// check route for cli download?
		// check grafana
		// check labels
		// create arm cluster

		// TODO stitch results together ginkgo
		// TODO junit result
		// TODO jenkinsfile
		// TODO docker
		// TODO destroy tests -> check for strings in destroy output -> check aws resources?
		// TODO interops
		// TODO CYPRESS
		// TODO test cluster curator

		// MC
		// clusterClaims:
		// - name: id.k8s.io
		//   value: 98ee7380-73b0-4e57-a9a6-6756c175859b
		// - name: kubeversion.open-cluster-management.io
		//   value: v1.27.3+4aaeaec
		// - name: platform.open-cluster-management.io
		//   value: AWS
		// - name: product.open-cluster-management.io
		//   value: OpenShift
		// - name: consoleurl.cluster.open-cluster-management.io
		//   value: https://console-openshift-console.apps.acmqe-hc-fee9e689aea34955ad76ab2814c7b55b.dev09.red-chesterfield.com
		// - name: controlplanetopology.openshift.io
		//   value: External
		// - name: hostedcluster.hypershift.openshift.io
		//   value: "true"
		// - name: id.openshift.io
		//   value: 98ee7380-73b0-4e57-a9a6-6756c175859b
		// - name: infrastructure.openshift.io
		//   value: '{"infraName":"acmqe-hc-fee9e689aea3-nmpzc"}'
		// - name: oauthredirecturis.openshift.io
		//   value: https://oauth-local-cluster-acmqe-hc-fee9e689aea34955ad76ab2814c7b55b.apps.clc-qe-hs-01.dev09.red-chesterfield.com:443/oauth/token/implicit
		// - name: region.open-cluster-management.io
		//   value: us-east-1
		// - name: version.openshift.io
		//   value: 4.14.0-ec.4

		// MC CONDITIONS
		// conditions:
		// - lastTransitionTime: "2023-08-25T07:24:26Z"
		//   message: Import succeeded
		//   reason: ManagedClusterImported
		//   status: "True"
		//   type: ManagedClusterImportSucceeded
		// - lastTransitionTime: "2023-08-25T07:24:26Z"
		//   message: Accepted by hub cluster admin
		//   reason: HubClusterAdminAccepted
		//   status: "True"
		//   type: HubAcceptedManagedCluster
		// - lastTransitionTime: "2023-08-25T07:24:32Z"
		//   message: Managed cluster joined
		//   reason: ManagedClusterJoined
		//   status: "True"
		//   type: ManagedClusterJoined
		// - lastTransitionTime: "2023-08-25T07:24:32Z"
		//   message: Managed cluster is available
		//   reason: ManagedClusterAvailable
		//   status: "True"
		//   type: ManagedClusterConditionAvailable

	})
})
