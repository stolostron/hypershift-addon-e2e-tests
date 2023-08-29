package hypershift_test

import (
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = ginkgo.Describe("Hosted Control Plane CLI Destroy Tests", ginkgo.Label("destroy", TYPE_AWS), func() {
	var config Config

	ginkgo.BeforeEach(func() {
		// GetNamespace with error handling
		namespace, err := utils.GetNamespace(TYPE_AWS)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.Namespace = namespace // TODO allow empty or default clusters ns

		// GetAWSCreds with error handling
		awsCreds, err := utils.GetAWSCreds()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		config.AWSCreds = awsCreds
	})

	ginkgo.It("Destroy all AWS hosted clusters on the hub", ginkgo.Label("destroy", "all"), func() {
		startTime := time.Now()

		// get list of hosted clusters
		// fail the test if no hosted clusters on the hub
		// TODO ensure we get only AWS ones
		hostedClusterList, err := utils.ListResource(dynamicClient, utils.HostedClustersGVR, "", "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		fmt.Fprintf(ginkgo.GinkgoWriter, "hostedClusterList: %v\n", hostedClusterList)

		// Run destroy command on all AWS hosted clusters without waiting to verify at first
		// for _, hostedCluster := range hostedClusterList {
		// 	commandArgs := []string{
		// 		"destroy", "cluster", TYPE_AWS,
		// 		"--name", hostedCluster.GetName(),
		// 		"--aws-creds", config.AWSCreds,
		// 		"--namespace", config.Namespace,
		// 		"--destroy-cloud-resources",
		// 	}

		// 	cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		// 	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		// 	defer gexec.KillAndWait()
		// 	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		// 	gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		// 	utils.PrintOutput(session) // prints command, args and output
		// }

		// Now we can verify each hosted cluster has sucecssfully been cleaned up
		for _, hostedCluster := range hostedClusterList {
			ginkgo.By(fmt.Sprintf("Waiting for hosted cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForHostedClusterDestroyed(dynamicClient, hostedCluster.GetName())
			})

			ginkgo.By(fmt.Sprintf("Waiting for managed cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForClusterDetached(dynamicClient, "acmqe-hc-062e17efdd1640a69bb229493c67ffdb")
			})
		}

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Destroy Hosted Clusters ===============================")
	})
})

// TODO FOR ALL
// -> Check if MC is destroyed
// -> Check if HC is destroyed, if not, dump HC?

// TODO destroyHostedCluster(hostedClusterName string)
// -> Given hosted cluster name, destroy it
// -> fail test if any errors

// TODO destroyHostedClustersLabel(label string) -> make sharable function out of it?
// -> Destroy all hosted clusters on hub with given label string
// -> use labelSelector when getting the list of hosted clusters

// example output
// 2023-08-25T00:47:46-07:00       INFO    Found hosted cluster    {"namespace": "local-cluster", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b"}
// 2023-08-25T00:47:47-07:00       INFO    Updated finalizer for hosted cluster    {"namespace": "local-cluster", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b"}
// 2023-08-25T00:47:47-07:00       INFO    Deleting hosted cluster {"namespace": "local-cluster", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b"}
// 2023-08-25T00:52:19-07:00       INFO    Destroying infrastructure       {"infraID": "acmqe-hc-fee9e689aea3-nmpzc"}
// 2023-08-25T00:52:25-07:00       INFO    Deleted VPC endpoints   {"IDs": "vpce-017de6462ef7b902c"}
// 2023-08-25T00:52:26-07:00       INFO    Deleted private hosted zone     {"id": "Z05881081RG8K6YWMCYD", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b.dev09.red-chesterfield.com."}
// 2023-08-25T00:52:27-07:00       INFO    Deleted private hosted zone     {"id": "Z02072111BWQ5PBFJUMNU", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b.hypershift.local."}
// 2023-08-25T00:52:27-07:00       INFO    Deleted route table     {"table": "rtb-07584713ddfabc714"}
// 2023-08-25T00:52:28-07:00       INFO    Deleted route from route table  {"table": "rtb-0767dc4e0b3086c2f", "destination": "0.0.0.0/0"}
// 2023-08-25T00:52:28-07:00       INFO    Removed route table association {"table": "rtb-0767dc4e0b3086c2f", "association": "rtb-0767dc4e0b3086c2f"}
// 2023-08-25T00:52:28-07:00       INFO    Deleted route from route table  {"table": "rtb-0700c48e3f0c14d07", "destination": "0.0.0.0/0"}
// 2023-08-25T00:52:28-07:00       INFO    Removed route table association {"table": "rtb-0700c48e3f0c14d07", "association": "rtb-0700c48e3f0c14d07"}
// 2023-08-25T00:52:28-07:00       INFO    Deleted route table     {"table": "rtb-0700c48e3f0c14d07"}
// 2023-08-25T00:52:29-07:00       INFO    WARNING: error during destroy, will retry       {"error": "deleting NAT gateway nat-0651545db685579c2"}
// 2023-08-25T00:52:39-07:00       INFO    WARNING: error during destroy, will retry       {"error": "NAT gateway nat-0651545db685579c2 still deleting"}
// 2023-08-25T00:52:49-07:00       INFO    WARNING: error during destroy, will retry       {"error": "NAT gateway nat-0651545db685579c2 still deleting"}
// 2023-08-25T00:52:59-07:00       INFO    WARNING: error during destroy, will retry       {"error": "NAT gateway nat-0651545db685579c2 still deleting"}
// 2023-08-25T00:53:10-07:00       INFO    Revoked security group ingress permissions      {"group": "sg-0a36b8290b002991f"}
// 2023-08-25T00:53:10-07:00       INFO    Revoked security group egress permissions       {"group": "sg-0a36b8290b002991f"}
// 2023-08-25T00:53:10-07:00       INFO    Deleted security group  {"group": "sg-0a36b8290b002991f"}
// 2023-08-25T00:53:10-07:00       INFO    Revoked security group ingress permissions      {"group": "sg-0336e2957d976064d"}
// 2023-08-25T00:53:10-07:00       INFO    Revoked security group egress permissions       {"group": "sg-0336e2957d976064d"}
// 2023-08-25T00:53:11-07:00       INFO    Revoked security group ingress permissions      {"group": "sg-02fa7a8bbcde74d37"}
// 2023-08-25T00:53:11-07:00       INFO    Revoked security group egress permissions       {"group": "sg-02fa7a8bbcde74d37"}
// 2023-08-25T00:53:11-07:00       INFO    Deleted security group  {"group": "sg-02fa7a8bbcde74d37"}
// 2023-08-25T00:53:12-07:00       INFO    Deleted subnet  {"id": "subnet-02dec402dec83e82f"}
// 2023-08-25T00:53:12-07:00       INFO    Deleted subnet  {"id": "subnet-06dcd5c55a052532d"}
// 2023-08-25T00:53:12-07:00       INFO    WARNING: error during destroy, will retry       {"error": "failed to delete vpc with id vpc-09e7b3ba137052742: DependencyViolation: The vpc 'vpc-09e7b3ba137052742' has dependencies and cannot be deleted.\n\tstatus code: 400, request id: 7a5dafaa-29a4-48a9-ac20-2aefaafda2c1"}
// 2023-08-25T00:53:15-07:00       INFO    Detached internet gateway from VPC      {"gateway id": "igw-07c6a0c8f6e4e46b2", "vpc": "vpc-09e7b3ba137052742"}
// 2023-08-25T00:53:15-07:00       INFO    Deleted internet gateway        {"id": "igw-07c6a0c8f6e4e46b2"}
// 2023-08-25T00:53:20-07:00       INFO    Deleted VPC     {"id": "vpc-09e7b3ba137052742"}
// 2023-08-25T00:53:21-07:00       INFO    Deleted EIP     {"id": "eipalloc-04959d5da153ea06f"}
// 2023-08-25T00:53:21-07:00       INFO    Deleted DHCP options    {"id": "dopt-0a3abea687d912b72"}
// 2023-08-25T00:53:21-07:00       INFO    Destroying IAM  {"infraID": "acmqe-hc-fee9e689aea3-nmpzc"}
// 2023-08-25T00:53:22-07:00       INFO    Deleted OIDC provider   {"providerARN": "arn:aws:iam::902449478968:oidc-provider/acmqe-hypershift.s3.us-east-1.amazonaws.com/acmqe-hc-fee9e689aea3-nmpzc"}
// 2023-08-25T00:53:22-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-openshift-ingress"}
// 2023-08-25T00:53:22-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-openshift-ingress"}
// 2023-08-25T00:53:22-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-openshift-image-registry"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-openshift-image-registry"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-aws-ebs-csi-driver-controller"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-aws-ebs-csi-driver-controller"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-cloud-controller"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-cloud-controller"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-node-pool"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-node-pool"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-control-plane-operator"}
// 2023-08-25T00:53:23-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-control-plane-operator"}
// 2023-08-25T00:53:24-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-cloud-network-config-controller"}
// 2023-08-25T00:53:24-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-cloud-network-config-controller"}
// 2023-08-25T00:53:24-07:00       INFO    Removed role from instance profile      {"profile": "acmqe-hc-fee9e689aea3-nmpzc-worker", "role": "acmqe-hc-fee9e689aea3-nmpzc-worker-role"}
// 2023-08-25T00:53:24-07:00       INFO    Deleted instance profile        {"profile": "acmqe-hc-fee9e689aea3-nmpzc-worker"}
// 2023-08-25T00:53:25-07:00       INFO    Deleted role policy     {"role": "acmqe-hc-fee9e689aea3-nmpzc-worker-role", "policy": "acmqe-hc-fee9e689aea3-nmpzc-worker-policy"}
// 2023-08-25T00:53:25-07:00       INFO    Deleted role    {"role": "acmqe-hc-fee9e689aea3-nmpzc-worker-role"}
// 2023-08-25T00:53:25-07:00       INFO    Deleting Secrets        {"namespace": "local-cluster"}
// 2023-08-25T00:53:25-07:00       INFO    Deleted CLI generated secrets
// 2023-08-25T00:53:25-07:00       INFO    Finalized hosted cluster        {"namespace": "local-cluster", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b"}
// 2023-08-25T00:53:25-07:00       INFO    Successfully destroyed cluster and infrastructure       {"namespace": "local-cluster", "name": "acmqe-hc-fee9e689aea34955ad76ab2814c7b55b", "infraID": "acmqe-hc-fee9e689aea3-nmpzc"}
