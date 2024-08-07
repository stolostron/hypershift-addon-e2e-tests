package hypershift_test

import (
	"fmt"
	"os/exec"
	"strings"
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

		config.ClusterArch, err = utils.GetArch()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		config.AWSStsCreds, err = utils.GetAWSStsCreds()
		o.Expect(err).ShouldNot(o.HaveOccurred())

		config.AWSRoleArn, err = utils.GetAWSRoleArn()
		o.Expect(err).ShouldNot(o.HaveOccurred())
	})

	g.It("Creates a FIPS AWS Hosted Cluster using STS Creds", g.Label("create"), func() {
		startTime := time.Now()
		// TODO ensure auto-import is enabled
		// check if it exists:
		// oc get addondeploymentconfig hypershift-addon-deploy-config -n mce -ojson | jq '.spec.ports | map(.name == "autoImportDisabled") | index(true)'

		commandArgs := []string{
			"create", "cluster", strings.ToLower(TYPE_AWS),
			"--name", config.ClusterName,
			"--sts-creds", config.AWSStsCreds,
			"--role-arn", config.AWSRoleArn,
			"--pull-secret", config.PullSecret,
			"--base-domain", config.BaseDomain,
			"--region", config.Region,
			"--node-pool-replicas", config.NodePoolReplicas,
			"--namespace", config.Namespace,
			"--instance-type", config.InstanceType,
			"--release-image", config.ReleaseImage,
			"--arch", config.ClusterArch,
		}
		// remove secret-creds
		// regular aws creds for s3 bucket

		// pre-setup the bucket via policy
		// pre-setup the role via policy

		if fipsEnabled == "true" {
			commandArgs = append(commandArgs, "--fips")
		}

		commandArgs = append(commandArgs, "--generate-ssh")

		if curatorEnabled == "true" {
			fmt.Println("CURATOR ENABLED, SETTING PAUSEDUNTIL TO TRUE")
			commandArgs = append(commandArgs, "--pausedUntil", "true")
		}

		fmt.Println(commandArgs)

		cmd := exec.Command(utils.HypershiftCLIName, commandArgs...)
		session, err := gexec.Start(cmd, g.GinkgoWriter, g.GinkgoWriter)
		o.Expect(err).ShouldNot(o.HaveOccurred())

		defer gexec.KillAndWait()

		o.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
		utils.PrintOutput(session) // prints command, args and output

		if curatorEnabled == "true" {
			// TODO: FAIL test if operator is not in good state or not installed -> suite level?
			// TODO: customize ansible template to choose which playbooks to target. maybe later...
			// TODO: awx: remove & upload expected templates to tower
			// Create/Update the aap tower secret -> suite level?
			fmt.Println("Creating Ansible Tower secret...")
			o.Expect(utils.CreateOrUpdateAnsibleTowerSecret(clientClient, "aap-tower-cred", config.Namespace, "", "")).Should(o.BeNil())

			// destroy any existing clustercurator first if it exists in the same ns with same name and then re-create it.
			o.Expect(utils.DeleteClusterCurator(dynamicClient, config.ClusterName, config.Namespace)).Should(o.BeNil())
			o.Expect(utils.CreateOrUpdateClusterCurator(
				clientClient, config.ClusterName, config.Namespace, "install", "hc-"+TYPE_AWS, "aap-tower-cred")).Should(o.BeNil())
		}

		if curatorEnabled == "true" {
			// TODO - Check all curator pods are not in error in the HC namespace
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
		// TODO Set fips=true label on the ManagedCluster, or just check the HC later
		// TODO check hostedcluster has fips: true

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
