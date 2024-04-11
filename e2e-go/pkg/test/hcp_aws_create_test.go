package hypershift_test

import (
	"fmt"
	"os/exec"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = g.Describe("Hosted Control Plane CLI AWS Create Tests:", g.Label(TYPE_AWS), func() {

	g.BeforeEach(func() {
		// Before each test, generate a unique cluster name to create the hosted cluster with
		config.ClusterName, err = utils.GenerateClusterName("acmqe-hc")
		o.Expect(err).ShouldNot(o.HaveOccurred())
	})

	g.It("Creates a FIPS AWS Hosted Cluster using --secret-creds", g.Label("create"), func() {
		startTime := time.Now()

		commandArgs := []string{
			"create", "cluster", TYPE_AWS,
			"--name", config.ClusterName,
			"--secret-creds", config.SecretCredsName,
			"--region", config.Region,
			"--node-pool-replicas", config.NodePoolReplicas,
			"--namespace", config.Namespace,
			"--instance-type", config.InstanceType,
			"--release-image", config.ReleaseImage,
			"--fips",
			"--generate-ssh",
		}

		if curatorEnabled == "true" {
			fmt.Println("CURATOR ENABLED, SETTING PAUSEDUNTIL TO TRUE")
			commandArgs = append(commandArgs, "--pausedUntil", "true")
		}

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, g.GinkgoWriter, g.GinkgoWriter)
		o.Expect(err).ShouldNot(o.HaveOccurred())

		o.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		utils.PrintOutput(session) // prints command, args and output
		defer gexec.KillAndWait()

		if curatorEnabled == "true" {
			fmt.Println("Creating Ansible Tower secret...")
			o.Expect(utils.CreateOrUpdateAnsibleTowerSecret(clientClient, "aap-tower-cred", config.Namespace, "", "")).Should(o.BeNil())

			// destroy any existing clustercurator first if it exists in the same ns with same name and then re-create it.
			o.Expect(utils.DeleteClusterCurator(dynamicClient, config.ClusterName, config.Namespace)).Should(o.BeNil())
			o.Expect(utils.CreateOrUpdateClusterCurator(
				clientClient, config.ClusterName, config.Namespace, "install", "hc-"+TYPE_AWS, "aap-tower-cred")).Should(o.BeNil())
		}

		if curatorEnabled == "true" {
			g.By(fmt.Sprintf("Waiting AnsibleJob for prehook-ansiblejob to complete for the cluster %s", config.ClusterName), func() {
				o.Eventually(func() bool {
					ansibleJob, err := utils.GetCurrentAnsibleJob(dynamicClient, config.ClusterName, config.Namespace)
					if ansibleJob == nil || err != nil {
						return false
					}
					isFinished := ansibleJob.Object["status"].(map[string]interface{})["isFinished"]
					hookVar := ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})["hook"]

					fmt.Printf("AnsibleJob isFinished field for the cluster %s: %#v\n", config.ClusterName, isFinished)
					fmt.Printf("AnsibleJob spec.extra_vars[hook] field for the cluster %s: %#v\n", config.ClusterName, hookVar)
					return isFinished != nil && isFinished.(bool) == true &&
						hookVar != nil && hookVar.(string) == "pre"
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeTrue())
				fmt.Printf("Prehook ansiblejob completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the prehook-ansiblejob to complete: %s\n", time.Since(startTime).String())
			})

			g.By(fmt.Sprintf("Waiting ClusterCurator for prehook-ansiblejob to complete with status True and reason job_has_finished for the cluster %s", config.ClusterName), func() {
				o.Eventually(func() error {
					return utils.CheckCuratorCondition(dynamicClient, config.ClusterName, config.Namespace, "prehook-ansiblejob", "True", "Completed executing init container", "Job_has_finished")
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeNil())
				fmt.Printf("Prehook ansiblejob completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the prehook-ansiblejob to complete: %s\n", time.Since(startTime).String())
			})
		}

		g.By(fmt.Sprintf("Waiting for hosted cluster plane for cluster %s to be available", config.ClusterName), func() {
			utils.WaitForHCPAvailable(dynamicClient, config.ClusterName, config.Namespace)
			fmt.Printf("Time taken for the hosted control plane to be available: %s\n", time.Since(startTime).String())
		})

		if curatorEnabled == "true" {
			g.By(fmt.Sprintf("Waiting for Job_has_finished to True for hypershift-provisioning-job for the cluster curator  %s", config.ClusterName), func() {
				o.Eventually(func() error {
					return utils.CheckCuratorCondition(dynamicClient, config.ClusterName, config.Namespace, "hypershift-provisioning-job", "True", "-provision", "Job_has_finished")
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeNil())
				fmt.Printf("hypershift-provisioning-job completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the hypershift-provisioning-job to complete: %s\n", time.Since(startTime).String())
			})

			g.By(fmt.Sprintf("Waiting AnsibleJob for posthook-ansiblejob to complete for the cluster %s", config.ClusterName), func() {
				ansibleJob, err := utils.GetCurrentAnsibleJob(dynamicClient, config.ClusterName, config.Namespace)
				o.Eventually(func() bool {
					if ansibleJob == nil || err != nil {
						return false
					}
					isFinished := ansibleJob.Object["status"].(map[string]interface{})["isFinished"]
					hookVar := ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})["hook"]

					fmt.Printf("AnsibleJob isFinished field for the cluster %s: %#v\n", config.ClusterName, isFinished)
					fmt.Printf("AnsibleJob spec.extra_vars[hook] field for the cluster %s: %#v\n", config.ClusterName, hookVar)
					return isFinished != nil && isFinished.(bool) == true &&
						hookVar != nil && hookVar.(string) == "post"
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeTrue())
				fmt.Printf("Posthook ansiblejob completed successfully for the cluster %s\n", config.ClusterName)
			})

			g.By(fmt.Sprintf("Waiting for Job_has_finished to True for clustercurator-job for the cluster curator  %s", config.ClusterName), func() {
				o.Eventually(func() error {
					return utils.CheckCuratorCondition(dynamicClient, config.ClusterName, config.Namespace, "clustercurator-job", "True", "DesiredCuration: install", "Job_has_finished")
				}, eventuallyTimeout, eventuallyInterval).Should(o.BeNil())
				fmt.Printf("clustercurator-job completed successfully for the cluster %s\n", config.ClusterName)
				fmt.Printf("Time taken for the clustercurator-job to complete: %s\n", time.Since(startTime).String())
			})
		}

		// Checks to see if ManagedCluster is created and the HC is auto-imported...
		g.By(fmt.Sprintf("Waiting for managed cluster %s to be Available", config.ClusterName), func() {
			utils.WaitForClusterImported(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster to be imported: %s\n", time.Since(startTime).String())
		})

		// Checks to see if add-ons are installed and available for the HC managed cluster...
		g.By(fmt.Sprintf("Waiting for managed cluster %s addons are Enabled and Available", config.ClusterName), func() {
			utils.WaitForClusterAddonsAvailable(dynamicClient, config.ClusterName)
			fmt.Printf("Time taken for the cluster be imported and addons ready: %s\n", time.Since(startTime).String())
		})

		g.By(fmt.Sprintf("Checking if managed cluster %s has the correct labels", config.ClusterName), func() {
			o.Eventually(func() bool {
				managedClusterLabels, err := utils.GetResourceLabels(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				o.Expect(err).ShouldNot(o.HaveOccurred())

				fmt.Printf("managedClusterLabels: %v\n", managedClusterLabels)
				return managedClusterLabels["name"] == config.ClusterName &&
					managedClusterLabels["cloud"] == "Amazon" &&
					managedClusterLabels["cluster.open-cluster-management.io/clusterset"] == "default" &&
					managedClusterLabels["vendor"] == "OpenShift"
			}, eventuallyTimeoutShort).Should(o.BeTrue())
		})

		g.By(fmt.Sprintf("Checking if managed cluster %s has the correct annotations", config.ClusterName), func() {
			o.Eventually(func() bool {
				managedClusterAnnotations, err := utils.GetResourceAnnotations(dynamicClient, utils.ManagedClustersGVR, "", config.ClusterName)
				o.Expect(err).ShouldNot(o.HaveOccurred())

				fmt.Printf("managedClusterAnnotations: %v\n", managedClusterAnnotations)
				return managedClusterAnnotations["import.open-cluster-management.io/klusterlet-deploy-mode"] == "Hosted" &&
					managedClusterAnnotations["import.open-cluster-management.io/hosting-cluster-name"] == utils.LocalClusterName &&
					managedClusterAnnotations["open-cluster-management/created-via"] == "hypershift"
			}, eventuallyTimeoutShort).Should(o.BeTrue())
		})

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Printf("========================= End Test create hosted cluster %s ===============================", config.ClusterName)
	})
})
