package hypershift_test

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	o "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = g.Describe("Hosted Control Plane CLI KubeVirt Destroy Tests:", g.Label(TYPE_KUBEVIRT), func() {

	g.It("Destroy all KubeVirt hosted clusters on the hub", g.Label("destroy"), func() {
		startTime := time.Now()

		// get list of kubevirt hosted clusters
		hostedClusterList, err := utils.GetHostedClustersList(dynamicClient, TYPE_KUBEVIRT, "")
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// if hostedClusterList is empty, skip the test
		if len(hostedClusterList) == 0 {
			g.Skip("No KubeVirt hosted clusters found on the hub")
		}

		// Run destroy command on all kubevirt hosted clusters without waiting to verify at first
		for _, hostedCluster := range hostedClusterList {
			fmt.Printf("KubeVirt Hosted Cluster found: %s\n", hostedCluster.GetName())

			commandArgs := []string{
				"destroy", "cluster", strings.ToLower(TYPE_KUBEVIRT),
				"--name", hostedCluster.GetName(),
				"--namespace", hostedCluster.GetNamespace(),
				"--destroy-cloud-resources",
			}

			cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
			session, err := gexec.Start(cmd, g.GinkgoWriter, g.GinkgoWriter)
			defer gexec.KillAndWait()
			o.Expect(err).ShouldNot(o.HaveOccurred())
			o.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
			utils.PrintOutput(session) // prints command, args and output
		}

		// Now we can verify each hosted cluster has sucecssfully been cleaned up
		for _, hostedCluster := range hostedClusterList {
			g.By(fmt.Sprintf("Waiting for hosted cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForHostedClusterDestroyed(dynamicClient, hostedCluster.GetName())
			})

			g.By(fmt.Sprintf("Waiting for managed cluster %s to be removed", hostedCluster.GetName()), func() {
				utils.WaitForClusterDetached(dynamicClient, hostedCluster.GetName())
			})
		}

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Destroy Hosted Clusters ===============================")
	})

	g.It("Destroy a KubeVirt hosted cluster on the hub", g.Label("destroy-one"), func() {
		startTime := time.Now()

		config.ClusterName, err = utils.GetClusterName("")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		if config.ClusterName == "" {
			g.Skip("HCP_CLUSTER_NAME is not defined. Please supply the name of the cluster to destroy before running.")
		}

		if curatorEnabled == "true" {
			fmt.Println("CURATOR ENABLED, INITILIZE DESTROY VIA CURATOR")
			o.Expect(utils.SetDesiredCuration(dynamicClient, config.ClusterName, config.Namespace, "destroy")).Should(o.BeNil())

			g.By(fmt.Sprintf("Waiting for prehook-ansiblejob to complete with status True and reason job_has_finished for the cluster %s", config.ClusterName), func() {
				o.Eventually(func() error {
					return utils.CheckCuratorCondition(dynamicClient, config.ClusterName, config.Namespace, "prehook-ansiblejob", "True", "Completed executing init container", "Job_has_finished")
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeNil())
				fmt.Printf("Prehook ansiblejob completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the prehook-ansiblejob to complete: %s\n", time.Since(startTime).String())
			})
		} else {
			commandArgs := []string{
				"destroy", "cluster", strings.ToLower(TYPE_KUBEVIRT),
				"--name", config.ClusterName,
				"--namespace", config.Namespace,
				"--destroy-cloud-resources",
			}

			cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
			session, err := gexec.Start(cmd, g.GinkgoWriter, g.GinkgoWriter)
			defer gexec.KillAndWait()
			o.Expect(err).ShouldNot(o.HaveOccurred())
			o.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
			utils.PrintOutput(session) // prints command, args and output
		}

		// Now we can verify the hosted cluster has sucecssfully been cleaned up
		g.By(fmt.Sprintf("Waiting for HostedCluster %s to be removed", config.ClusterName), func() {
			utils.WaitForHostedClusterDestroyed(dynamicClient, config.ClusterName)
		})

		if curatorEnabled == "true" {
			g.By(fmt.Sprintf("Waiting for Job_has_finished to True for hypershift-uninstalling-job for the cluster curator  %s", config.ClusterName), func() {
				o.Eventually(func() error {
					return utils.CheckCuratorCondition(dynamicClient, config.ClusterName, config.Namespace, "hypershift-uninstalling-job", "True", "-uninstall", "Job_has_finished")
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeNil())
				fmt.Printf("hypershift-uninstalling-job completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the hypershift-uninstalling-job to complete: %s\n", time.Since(startTime).String())
			})

			g.By(fmt.Sprintf("Waiting AnsibleJob for posthook-ansiblejob to complete for the cluster %s", config.ClusterName), func() {
				o.Eventually(func() bool {
					ansibleJob, err := utils.GetCurrentAnsibleJob(dynamicClient, config.ClusterName, config.Namespace)
					if ansibleJob == nil || err != nil {
						return false
					}
					isFinished := ansibleJob.Object["status"].(map[string]interface{})["isFinished"]
					hookVar := ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})["hook"]

					fmt.Printf("AnsibleJob isFinished field for the cluster %s: %#v\n", config.ClusterName, isFinished)
					return isFinished != nil && isFinished.(bool) == true &&
						hookVar != nil && hookVar.(string) == "post"
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeTrue())
				fmt.Printf("Posthook ansiblejob completed successfully for the cluster %s\n", config.ClusterName)
			})
		}

		g.By(fmt.Sprintf("Waiting for ManagedCluster %s to be removed", config.ClusterName), func() {
			utils.WaitForClusterDetached(dynamicClient, config.ClusterName)
		})

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Destroy Hosted Cluster ===============================")
	})
})
