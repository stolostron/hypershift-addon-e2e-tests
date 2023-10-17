package hypershift_test

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metrics "github.com/openshift/library-go/test/library/metrics"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	MCE_HS_SERVICE_MONITOR = "mce-hypershift-addon-agent-metrics"
	ACM_HS_SERVICE_MONITOR = "acm-hypershift-addon-agent-metrics"
)

var (
	prometheus              prometheusv1.API
	totalHostedClusterCount int
	awsHostedClusterCount   int
	kvHostedClusterCount    int
	agentHostedClusterCount int

	serviceMonitorGVR = schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}
)

func promQueryVector(ctx context.Context, query string, ts time.Time, opts ...string) model.Vector {
	result, warnings, err := prometheus.Query(ctx, query, ts)
	o.Expect(err).ToNot(o.HaveOccurred())
	fmt.Println("Query result: ", result)

	// Check for any warnings or errors returned by the query.
	if len(warnings) > 0 {
		for _, warning := range warnings {
			fmt.Println("Warning:", warning)
		}
	}

	vec, ok := result.(model.Vector)
	if !ok {
		o.Expect(fmt.Errorf("expecting Prometheus query to return a vector, got %s instead", vec.Type())).ToNot(o.HaveOccurred())
	}

	fmt.Println("Returning Vector: ", vec)
	return vec
}

var _ = g.Describe("Hosted Control Plane CLI KubeVirt Create Tests:", g.Label("metrics", "@e2e", "@post-upgrade"), func() {

	g.BeforeEach(func() {
		// number of total hosted clusters
		hostedClusterList, err := utils.GetHostedClustersList(dynamicClient, "", "")
		o.Expect(err).ShouldNot(o.HaveOccurred())
		totalHostedClusterCount = len(hostedClusterList)
		fmt.Println("Number of total hosted clusters: ", totalHostedClusterCount)

		// number of aws hosted clusters
		awsHostedClusterList, err := utils.GetAWSHostedClustersList(dynamicClient, "")
		o.Expect(err).ShouldNot(o.HaveOccurred())
		awsHostedClusterCount = len(awsHostedClusterList)
		fmt.Println("Number of AWS hosted clusters: ", awsHostedClusterCount)

		// number of kv hosted clusters
		kvHostedClusterList, err := utils.GetKubevirtHostedClustersList(dynamicClient, "")
		o.Expect(err).ShouldNot(o.HaveOccurred())
		kvHostedClusterCount = len(kvHostedClusterList)
		fmt.Println("Number of Kubevirt hosted clusters: ", kvHostedClusterCount)

		// number of agent hosted clusters
		agentHostedClusterList, err := utils.GetAgentHostedClustersList(dynamicClient, "")
		o.Expect(err).ShouldNot(o.HaveOccurred())
		agentHostedClusterCount = len(agentHostedClusterList)
		fmt.Println("Number of Agent hosted clusters: ", agentHostedClusterCount)

		prometheus, err = metrics.NewPrometheusClient(context.TODO(), kubeClient, routeClient)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("Checks if service monitors are deployed to the correct namespaces", g.Label("service_monitor"), func() {
		// Check service monitors does not exist in the openshift-monitoring namespace
		_, err := utils.GetResource(dynamicClient, serviceMonitorGVR, "openshift-monitoring", MCE_HS_SERVICE_MONITOR)
		o.Expect(errors.IsNotFound(err)).Should(o.BeTrue())

		_, err = utils.GetResource(dynamicClient, serviceMonitorGVR, "openshift-monitoring", ACM_HS_SERVICE_MONITOR)
		o.Expect(errors.IsNotFound(err)).Should(o.BeTrue())

		// Check service monitors does not exist in the hypershift namespace
		_, err = utils.GetResource(dynamicClient, serviceMonitorGVR, "hypershift", MCE_HS_SERVICE_MONITOR)
		o.Expect(errors.IsNotFound(err)).Should(o.BeTrue())

		_, err = utils.GetResource(dynamicClient, serviceMonitorGVR, "hypershift", ACM_HS_SERVICE_MONITOR)
		o.Expect(errors.IsNotFound(err)).Should(o.BeTrue())

		// Check MCE service monitor EXISTS in the open-cluster-management-agent-addon namespace
		_, err = utils.GetResource(dynamicClient, serviceMonitorGVR, "open-cluster-management-agent-addon", MCE_HS_SERVICE_MONITOR)
		o.Expect(err).ShouldNot(o.HaveOccurred())

		// Check ACM service monitor does not exist in the open-cluster-management-agent-addon namespace
		_, err = utils.GetResource(dynamicClient, serviceMonitorGVR, "open-cluster-management-agent-addon", ACM_HS_SERVICE_MONITOR)
		o.Expect(errors.IsNotFound(err)).Should(o.BeTrue())

		// Check if the namespaces have the correct label of openshift.io/cluster-monitoring=true
		hsNS, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), "hypershift", metav1.GetOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred())
		addonNS, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), "open-cluster-management-agent-addon", metav1.GetOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred())

		o.Eventually(func() bool {
			return hsNS.Labels["openshift.io/cluster-monitoring"] == "true" &&
				addonNS.Labels["openshift.io/cluster-monitoring"] == "true"
		}, eventuallyTimeoutShort).Should(o.BeTrue())

	})

	// checks metrics related to addon health:
	//	- mce_hs_addon_install_failure_gauge				should be 0
	// 	- mce_hs_addon_install_failing_gauge_bool			should be 0
	// 	- mce_hs_addon_failed_to_start_bool					should be 0
	//	- mce_hs_addon_hypershift_operator_degraded_bool	should be 0
	g.It("Retrieve promethesus metrics related to hypershift-addon health", g.Label("metrics", "health"), func() {
		startTime := time.Now()
		fmt.Println("========================= Start Test Hypershift add-on health metrics ===============================")

		mce_hs_addon_install_failure_gauge := promQueryVector(
			context.Background(), "mce_hs_addon_install_failure_gauge", time.Now())
		fmt.Println("mce_hs_addon_install_failure_gauge: ", mce_hs_addon_install_failure_gauge[0])
		o.Expect(mce_hs_addon_install_failure_gauge[0].Value).Should(o.BeEquivalentTo(0))

		mce_hs_addon_install_failing_gauge_bool := promQueryVector(
			context.Background(), "mce_hs_addon_install_failing_gauge_bool", time.Now())
		fmt.Println("mce_hs_addon_install_failing_gauge_bool: ", mce_hs_addon_install_failing_gauge_bool[0])
		o.Expect(mce_hs_addon_install_failing_gauge_bool[0].Value).Should(o.BeEquivalentTo(0))

		mce_hs_addon_failed_to_start_bool := promQueryVector(
			context.Background(), "mce_hs_addon_failed_to_start_bool", time.Now())
		fmt.Println("mce_hs_addon_failed_to_start_bool: ", mce_hs_addon_failed_to_start_bool[0])
		o.Expect(mce_hs_addon_failed_to_start_bool[0].Value).Should(o.BeEquivalentTo(0))

		mce_hs_addon_hypershift_operator_degraded_bool := promQueryVector(
			context.Background(), "mce_hs_addon_hypershift_operator_degraded_bool", time.Now())
		fmt.Println("mce_hs_addon_hypershift_operator_degraded_bool: ", mce_hs_addon_hypershift_operator_degraded_bool[0])
		o.Expect(mce_hs_addon_hypershift_operator_degraded_bool[0].Value).Should(o.BeEquivalentTo(0))

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= Start Test Hypershift add-on health metrics ===============================")
	})

	g.It("Retrieve promethesus metrics related to hosted clusters", g.Label("metrics"), func() {
		startTime := time.Now()

		fmt.Println("========================= Start Test Hosted Clusters Metrics ===============================")

		mce_hs_addon_total_hosted_control_planes_gauge := promQueryVector(
			context.Background(), "mce_hs_addon_total_hosted_control_planes_gauge", time.Now())
		fmt.Println("mce_hs_addon_total_hosted_control_planes_gauge: ", mce_hs_addon_total_hosted_control_planes_gauge[0])
		o.Expect(totalHostedClusterCount).Should(o.BeEquivalentTo(mce_hs_addon_total_hosted_control_planes_gauge[0].Value))

		// Check promethesus for number of kubevirt hosted clusters
		hypershift_hostedclusters_kv := promQueryVector(
			context.Background(), `hypershift_hostedclusters{platform="KubeVirt"}`, time.Now())
		fmt.Println(`hypershift_hostedclusters{platform="KubeVirt"} `, hypershift_hostedclusters_kv[0])
		o.Expect(kvHostedClusterCount).Should(o.BeEquivalentTo(hypershift_hostedclusters_kv[0].Value))

		// Check promethesus for number of agent hosted clusters
		hypershift_hostedclusters_agent := promQueryVector(
			context.Background(), `hypershift_hostedclusters{platform="Agent"}`, time.Now())
		fmt.Println(`hypershift_hostedclusters{platform="Agent"} `, hypershift_hostedclusters_agent[0])
		o.Expect(agentHostedClusterCount).Should(o.BeEquivalentTo(hypershift_hostedclusters_agent[0].Value))

		// Check promethesus for number of aws hosted clusters
		hypershift_hostedclusters_aws := promQueryVector(
			context.Background(), `hypershift_hostedclusters{platform="AWS"}`, time.Now())
		fmt.Println(`hypershift_hostedclusters{platform="AWS"} `, hypershift_hostedclusters_aws[0])
		o.Expect(awsHostedClusterCount).Should(o.BeEquivalentTo(hypershift_hostedclusters_aws[0].Value))

		fmt.Printf("Test Duration: %s\n", time.Since(startTime).String())
		fmt.Println("========================= End Test Hosted Clusters Metrics ===============================")
	})
})

// TODO metrics hosted
//	mce_hs_addon_available_hosted_clusters_gauge	should be number of total good hcp

/// TODO hc metrics
//	hypershift_cluster_available_duration_seconds	use in create, print out, should not fail
// 	hypershift_nodepools_available_replicas
// 	hypershift_nodepools_size
//	hypershift_hostedcluster_nodepools

// TODO
// Try to find number of IBM hosted clusters
// Try to find number of Azure hosted clusters (not supported yet, should return 0?)
