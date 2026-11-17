package hypershift_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	proxyCreatedVia             = "hcp.ocm.io/created-via"
	proxyHCLabel                = "hcp.ocm.io/hostedcluster"
	proxyFinalizer              = "openshift.io/destroy-cluster"
	proxySupportedVersionsLabel = "hypershift.openshift.io/supported-versions"
)

type proxyFixture struct {
	hubURL   string
	spoke    string
	ns       string
	name     string
	spokeK8s kubernetes.Interface
	spokeDyn dynamic.Interface
}

var _ = g.Describe("ACM-38016 HCP proxy integration", g.Label("hcp-proxy", "ACM-38016"), func() {
	var f proxyFixture

	g.BeforeEach(func() {
		if hcpProxyEnvironmentMissing {
			g.Skip("hcp-proxy integration requires a hub kubeconfig; set KUBECONFIG")
		}
		cfg, e := utils.NewKubeConfig()
		o.Expect(e).NotTo(o.HaveOccurred())
		f.hubURL = strings.TrimRight(cfg.Host, "/")
		f.spoke = os.Getenv("HCP_PROXY_HOSTING_CLUSTER")
		if f.spoke == "" {
			f.spoke = defaultManagedCluster
		}
		spokeConfig := cfg
		if path := os.Getenv("HCP_PROXY_SPOKE_KUBECONFIG"); path != "" {
			spokeConfig, e = clientcmd.BuildConfigFromFlags("", path)
			o.Expect(e).NotTo(o.HaveOccurred())
		} else if f.spoke != utils.LocalClusterName {
			g.Skip(fmt.Sprintf("set HCP_PROXY_SPOKE_KUBECONFIG to verify resources on hosting cluster %s", f.spoke))
		}
		f.spokeK8s, e = kubernetes.NewForConfig(spokeConfig)
		o.Expect(e).NotTo(o.HaveOccurred())
		f.spokeDyn, e = dynamic.NewForConfig(spokeConfig)
		o.Expect(e).NotTo(o.HaveOccurred())

		// Every request below enters through the hub kube-apiserver/APIService.
		// The add-on condition makes a missing cluster-proxy an actionable failure.
		o.Eventually(func() bool {
			addon, getErr := addonClient.AddonV1alpha1().ManagedClusterAddOns(f.spoke).
				Get(context.Background(), "cluster-proxy", metav1.GetOptions{})
			if getErr != nil {
				return false
			}
			for _, condition := range addon.Status.Conditions {
				if condition.Type == "Available" && condition.Status == metav1.ConditionTrue {
					return true
				}
			}
			return false
		}, 2*time.Minute, 5*time.Second).Should(o.BeTrue(), "cluster-proxy must be Available on %s", f.spoke)
	})

	g.It("creates, reads, updates, and deletes a HostedCluster and NodePool", g.Label("ACM-37268", "ACM-37269"), func() {
		f.newResources()

		// A compliant caller runs pre-create validation before posting the
		// CreateRequest. Exercise that success path before applying resources.
		status, body := f.request(http.MethodGet, f.subresourcePath("validate"), nil)
		expectProxyStatus(status, body, http.StatusOK)

		created := f.create(nil)
		hc := proxyObject(created, "hostedCluster")
		o.Expect(proxyName(hc)).To(o.Equal(f.name))
		o.Expect(proxyLabels(hc)[proxyCreatedVia]).To(o.Equal("hcp-from-hub"))
		o.Expect(proxyObjects(created, "nodePools")).To(o.HaveLen(1))

		spokeHC := f.getSpoke(utils.HostedClustersGVR, f.name)
		o.Expect(spokeHC.GetLabels()[proxyCreatedVia]).To(o.Equal("hcp-from-hub"))
		spokeNP := f.getSpoke(utils.NodePoolsGVR, f.name+"-workers")
		o.Expect(spokeNP.GetLabels()[proxyCreatedVia]).To(o.Equal("hcp-from-hub"))

		path := f.subresourcePath("resources")
		status, body = f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusOK)
		bundle := decodeProxyObject(body)
		o.Expect(proxyName(proxyObject(bundle, "hostedCluster"))).To(o.Equal(f.name))
		o.Expect(proxyObjects(bundle, "nodePools")).To(o.HaveLen(1))

		// PUT is a full replace, so start with the live bundle and preserve
		// metadata.resourceVersion and all operator-owned fields.
		proxyLabels(proxyObject(bundle, "hostedCluster"))["e2e.hcp.ocm.io/roundtrip"] = "updated"
		proxyLabels(proxyObjects(bundle, "nodePools")[0])["e2e.hcp.ocm.io/roundtrip"] = "updated"
		status, body = f.request(http.MethodPut, path, bundle)
		expectProxyStatus(status, body, http.StatusOK)
		o.Expect(f.getSpoke(utils.HostedClustersGVR, f.name).GetLabels()["e2e.hcp.ocm.io/roundtrip"]).To(o.Equal("updated"))
		o.Expect(f.getSpoke(utils.NodePoolsGVR, f.name+"-workers").GetLabels()["e2e.hcp.ocm.io/roundtrip"]).To(o.Equal("updated"))

		status, body = f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusOK)
		got := decodeProxyObject(body)
		o.Expect(proxyLabels(proxyObject(got, "hostedCluster"))["e2e.hcp.ocm.io/roundtrip"]).To(o.Equal("updated"))

		status, body = f.request(http.MethodDelete, f.namedPath(), nil)
		o.Expect(status).To(o.BeNumerically(">=", 200), string(body))
		o.Expect(status).To(o.BeNumerically("<", 300), string(body))
		f.expectSpokeGone(utils.HostedClustersGVR, f.name)
		f.expectSpokeGone(utils.NodePoolsGVR, f.name+"-workers")
	})

	g.It("applies allowlisted Role and ConfigMap extra objects and removes them on delete", g.Label("ACM-39216"), func() {
		f.newResources()
		extra := []interface{}{
			map[string]interface{}{
				"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "Role",
				"metadata": map[string]interface{}{"name": "capi-provider-role"},
				"rules": []interface{}{map[string]interface{}{
					"apiGroups": []string{""}, "resources": []string{"configmaps"}, "verbs": []string{"get"},
				}},
			},
			map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]interface{}{"name": "user-ca-bundle"},
				"data":     map[string]string{"ca-bundle.crt": "e2e-ca"},
			},
		}
		created := f.create(extra)
		o.Expect(proxyObjects(created, "extraObjects")).To(o.HaveLen(2))
		role, e := f.spokeK8s.RbacV1().Roles(f.ns).Get(context.Background(), "capi-provider-role", metav1.GetOptions{})
		o.Expect(e).NotTo(o.HaveOccurred())
		o.Expect(role.Labels).To(o.HaveKeyWithValue(proxyCreatedVia, "hcp-from-hub"))
		o.Expect(role.Labels).To(o.HaveKeyWithValue(proxyHCLabel, f.name))
		cm, e := f.spokeK8s.CoreV1().ConfigMaps(f.ns).Get(context.Background(), "user-ca-bundle", metav1.GetOptions{})
		o.Expect(e).NotTo(o.HaveOccurred())
		o.Expect(cm.Labels).To(o.HaveKeyWithValue(proxyCreatedVia, "hcp-from-hub"))
		o.Expect(cm.Labels).To(o.HaveKeyWithValue(proxyHCLabel, f.name))
		o.Expect(cm.Data["ca-bundle.crt"]).To(o.Equal("e2e-ca"))

		status, body := f.request(http.MethodDelete, f.namedPath(), nil)
		o.Expect(status).To(o.BeNumerically(">=", 200), string(body))
		o.Expect(status).To(o.BeNumerically("<", 300), string(body))
		f.expectSpokeGone(utils.HostedClustersGVR, f.name)
		o.Eventually(func() bool {
			_, roleErr := f.spokeK8s.RbacV1().Roles(f.ns).Get(context.Background(), "capi-provider-role", metav1.GetOptions{})
			_, cmErr := f.spokeK8s.CoreV1().ConfigMaps(f.ns).Get(context.Background(), "user-ca-bundle", metav1.GetOptions{})
			return apierrors.IsNotFound(roleErr) && apierrors.IsNotFound(cmErr)
		}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())
	})

	g.It("reports a spoke apply failure instead of claiming extra object success", g.Label("ACM-39216"), func() {
		f.newResources()
		f.requireCreatePayload()
		badRole := map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "Role",
			"metadata": map[string]interface{}{"name": "invalid-role"},
			"rules": []interface{}{map[string]interface{}{
				"apiGroups": []string{""}, "resources": []string{"configmaps"}, "verbs": []string{},
			}},
		}
		status, body := f.request(http.MethodPost, f.collectionPath(), f.createBody([]interface{}{badRole}))
		o.Expect(status).To(o.BeNumerically(">=", 400), string(body))
		_, e := f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Get(context.Background(), f.name, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(e)).To(o.BeTrue(), "HostedCluster should not be applied: %v", e)
	})

	g.It("holds deletion with the CLI finalizer until it is removed", g.Label("ACM-39226"), func() {
		f.newResources()
		f.create(nil)
		finalizersPath := f.subresourcePath("finalizers")
		status, body := f.request(http.MethodPatch, finalizersPath, map[string]string{"operation": "add"})
		expectProxyStatus(status, body, http.StatusOK)
		o.Expect(proxyStringSlice(proxyObject(decodeProxyObject(body), "hostedCluster"), "metadata", "finalizers")).To(o.ContainElement(proxyFinalizer))

		status, body = f.request(http.MethodDelete, f.namedPath(), nil)
		o.Expect(status).To(o.BeNumerically(">=", 200), string(body))
		o.Expect(status).To(o.BeNumerically("<", 300), string(body))
		o.Eventually(func() bool {
			hc, getErr := f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Get(context.Background(), f.name, metav1.GetOptions{})
			if getErr != nil {
				return false
			}
			finalizers := hc.GetFinalizers()
			return hc.GetDeletionTimestamp() != nil && len(finalizers) == 1 && finalizers[0] == proxyFinalizer
		}, 2*time.Minute, 5*time.Second).Should(o.BeTrue())

		status, body = f.request(http.MethodPatch, finalizersPath, map[string]string{"operation": "remove"})
		// Removing the last finalizer can delete the object before the proxy's
		// follow-up GET. Both 200 and 404 mean the finalizer was removed.
		o.Expect([]int{http.StatusOK, http.StatusNotFound}).To(o.ContainElement(status), string(body))
		f.expectSpokeGone(utils.HostedClustersGVR, f.name)

		status, body = f.request(http.MethodPatch, finalizersPath, map[string]string{"operation": "remove"})
		expectProxyStatus(status, body, http.StatusNotFound)
	})

	g.It("returns the hosting cluster HyperShift Operator server version", g.Label("ACM-39227"), func() {
		cm := f.supportedVersionsConfigMap()
		path := "/apis/hcp.ocm.io/v1alpha1/namespaces/hypershift/version?hostingCluster=" + url.QueryEscape(f.spoke)
		status, body := f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusOK)
		var response struct {
			ServerVersion string `json:"serverVersion"`
		}
		o.Expect(json.Unmarshal(body, &response)).To(o.Succeed())
		o.Expect(response.ServerVersion).To(o.Equal(cm.Data["server-version"]))
		o.Expect(response.ServerVersion).NotTo(o.BeEmpty())

		badPath := "/apis/hcp.ocm.io/v1alpha1/namespaces/hypershift/version?hostingCluster=hcp-proxy-nonexistent-spoke"
		status, body = f.request(http.MethodGet, badPath, nil)
		expectProxyFailure(status, body, http.StatusServiceUnavailable)
	})

	g.It("returns supported OCP versions from the hosting cluster", g.Label("ACM-39228"), func() {
		cm := f.supportedVersionsConfigMap()
		var supported struct {
			Versions []string `json:"versions"`
		}
		o.Expect(json.Unmarshal([]byte(cm.Data["supported-versions"]), &supported)).To(o.Succeed())
		path := "/apis/hcp.ocm.io/v1alpha1/namespaces/hypershift/version?hostingCluster=" + url.QueryEscape(f.spoke)
		status, body := f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusOK)
		var response struct {
			SupportedVersions []string `json:"supportedVersions"`
		}
		o.Expect(json.Unmarshal(body, &response)).To(o.Succeed())
		o.Expect(response.SupportedVersions).To(o.Equal(supported.Versions))
	})

	g.It("fails closed when the supported-versions ConfigMap is missing", g.Label("ACM-39227"), func() {
		if strings.ToLower(os.Getenv("HCP_PROXY_TEST_MISSING_VERSION_CONFIGMAP")) != "true" {
			g.Skip("set HCP_PROXY_TEST_MISSING_VERSION_CONFIGMAP=true on a dedicated hosting cluster to run this destructive failure test")
		}

		cm := f.supportedVersionsConfigMap()
		err := f.spokeK8s.CoreV1().ConfigMaps(cm.Namespace).Delete(context.Background(), cm.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			restored := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: cm.Name, Namespace: cm.Namespace, Labels: cm.Labels},
				Data:       cm.Data,
				BinaryData: cm.BinaryData,
			}
			_, restoreErr := f.spokeK8s.CoreV1().ConfigMaps(cm.Namespace).Create(context.Background(), restored, metav1.CreateOptions{})
			if apierrors.IsAlreadyExists(restoreErr) {
				existing, getErr := f.spokeK8s.CoreV1().ConfigMaps(cm.Namespace).Get(context.Background(), cm.Name, metav1.GetOptions{})
				o.Expect(getErr).NotTo(o.HaveOccurred())
				existing.Labels = cm.Labels
				existing.Data = cm.Data
				existing.BinaryData = cm.BinaryData
				_, restoreErr = f.spokeK8s.CoreV1().ConfigMaps(cm.Namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
			}
			o.Expect(restoreErr).NotTo(o.HaveOccurred())
		})

		path := "/apis/hcp.ocm.io/v1alpha1/namespaces/hypershift/version?hostingCluster=" + url.QueryEscape(f.spoke)
		status, body := f.request(http.MethodGet, path, nil)
		expectProxyFailure(status, body)
	})

	g.It("validates duplicate names and incompatible architecture without writing resources", g.Label("ACM-42875"), func() {
		f.newResources()
		path := f.subresourcePath("validate")
		status, body := f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusOK)
		f.create(nil)
		status, body = f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusConflict)

		otherName := f.name + "-arch"
		arch := f.incompatibleArch()
		archPath := "/apis/hcp.ocm.io/v1alpha1/namespaces/" + f.ns + "/hostedclusters/" + otherName +
			"/validate?hostingCluster=" + url.QueryEscape(f.spoke) + "&arch=" + arch
		status, body = f.request(http.MethodGet, archPath, nil)
		expectProxyStatus(status, body, http.StatusBadRequest)
		_, getErr := f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Get(context.Background(), otherName, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(getErr)).To(o.BeTrue(), "validation must not create the HostedCluster: %v", getErr)
	})

	g.It("stops a duplicate-name create during preflight validation", g.Label("ACM-42875"), func() {
		f.newResources()
		f.requireCreatePayload()
		f.create(nil)

		status, body := f.request(http.MethodGet, f.subresourcePath("validate"), nil)
		expectProxyStatus(status, body, http.StatusConflict)
		o.Expect(string(body)).To(o.ContainSubstring("already exists"))
	})

	g.It("stops an incompatible-architecture create during preflight validation", g.Label("ACM-42875"), func() {
		f.newResourceNames()
		arch := f.incompatibleArch()
		path := f.subresourcePath("validate") + "&arch=" + url.QueryEscape(arch)
		status, body := f.request(http.MethodGet, path, nil)
		expectProxyStatus(status, body, http.StatusBadRequest)

		_, e := f.spokeK8s.CoreV1().Namespaces().Get(context.Background(), f.ns, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(e)).To(o.BeTrue(), "validation must not create the namespace: %v", e)
		_, e = f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Get(context.Background(), f.name, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(e)).To(o.BeTrue(), "validation must not create the HostedCluster: %v", e)
		_, e = f.spokeDyn.Resource(utils.NodePoolsGVR).Namespace(f.ns).Get(context.Background(), f.name+"-workers", metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(e)).To(o.BeTrue(), "validation must not create the NodePool: %v", e)
	})
})

func (f *proxyFixture) newResourceNames() {
	stamp := time.Now().UnixNano()
	f.ns = fmt.Sprintf("hcp-proxy-e2e-%d-%d", g.GinkgoParallelProcess(), stamp)
	f.name = fmt.Sprintf("hcp-proxy-%d-%d", g.GinkgoParallelProcess(), stamp)

	g.DeferCleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if hc, e := f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Get(ctx, f.name, metav1.GetOptions{}); e == nil && containsString(hc.GetFinalizers(), proxyFinalizer) {
			finalizers := make([]string, 0, len(hc.GetFinalizers()))
			for _, item := range hc.GetFinalizers() {
				if item != proxyFinalizer {
					finalizers = append(finalizers, item)
				}
			}
			patch, marshalErr := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"finalizers": finalizers}})
			if marshalErr == nil {
				_, _ = f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).
					Patch(ctx, f.name, types.MergePatchType, patch, metav1.PatchOptions{})
			}
		}
		_ = f.spokeDyn.Resource(utils.NodePoolsGVR).Namespace(f.ns).Delete(ctx, f.name+"-workers", metav1.DeleteOptions{})
		_ = f.spokeDyn.Resource(utils.HostedClustersGVR).Namespace(f.ns).Delete(ctx, f.name, metav1.DeleteOptions{})
		err := f.spokeK8s.CoreV1().Namespaces().Delete(ctx, f.ns, metav1.DeleteOptions{})
		o.Expect(err == nil || apierrors.IsNotFound(err)).To(o.BeTrue(), "delete test namespace %s: %v", f.ns, err)
	})
}

func (f *proxyFixture) newResources() {
	f.newResourceNames()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := f.spokeK8s.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: f.ns},
	}, metav1.CreateOptions{})
	cancel()
	o.Expect(err == nil || apierrors.IsAlreadyExists(err)).To(o.BeTrue(), "create test namespace %s: %v", f.ns, err)
}

func (f *proxyFixture) collectionPath() string {
	return "/apis/hcp.ocm.io/v1alpha1/namespaces/" + f.ns + "/hostedclusters?hostingCluster=" + url.QueryEscape(f.spoke)
}

func (f *proxyFixture) namedPath() string {
	return "/apis/hcp.ocm.io/v1alpha1/namespaces/" + f.ns + "/hostedclusters/" + f.name + "?hostingCluster=" + url.QueryEscape(f.spoke)
}

func (f *proxyFixture) subresourcePath(name string) string {
	return "/apis/hcp.ocm.io/v1alpha1/namespaces/" + f.ns + "/hostedclusters/" + f.name + "/" + name + "?hostingCluster=" + url.QueryEscape(f.spoke)
}

func (f *proxyFixture) supportedVersionsConfigMap() *corev1.ConfigMap {
	list, e := f.spokeK8s.CoreV1().ConfigMaps("hypershift").List(context.Background(), metav1.ListOptions{
		LabelSelector: proxySupportedVersionsLabel,
	})
	o.Expect(e).NotTo(o.HaveOccurred())
	var result *corev1.ConfigMap
	for i := range list.Items {
		if list.Items[i].Name == "supported-versions" {
			result = &list.Items[i]
			break
		}
	}
	o.Expect(result).NotTo(o.BeNil(), "labeled supported-versions ConfigMap not found in hypershift namespace")
	o.Expect(result.Labels[proxySupportedVersionsLabel]).To(o.Equal("true"))
	return result
}

func (f *proxyFixture) request(method, path string, payload interface{}) (int, []byte) {
	var body io.Reader
	if payload != nil {
		encoded, e := json.Marshal(payload)
		o.Expect(e).NotTo(o.HaveOccurred())
		body = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, method, f.hubURL+path, body)
	o.Expect(e).NotTo(o.HaveOccurred())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := httpc.Do(req)
	o.Expect(e).NotTo(o.HaveOccurred(), "%s %s", method, path)
	defer resp.Body.Close()
	result, e := io.ReadAll(resp.Body)
	o.Expect(e).NotTo(o.HaveOccurred())
	return resp.StatusCode, result
}

func (f *proxyFixture) createBody(extra []interface{}) map[string]interface{} {
	image := config.ReleaseImage
	if override := os.Getenv("HCP_PROXY_RELEASE_IMAGE"); override != "" {
		image = override
	}
	if path := os.Getenv("HCP_PROXY_CREATE_REQUEST_TEMPLATE"); path != "" {
		fixture, e := os.ReadFile(path)
		o.Expect(e).NotTo(o.HaveOccurred())
		rendered := strings.NewReplacer(
			"{{NAMESPACE}}", f.ns,
			"{{NAME}}", f.name,
			"{{RELEASE_IMAGE}}", image,
		).Replace(string(fixture))
		var request map[string]interface{}
		o.Expect(json.Unmarshal([]byte(rendered), &request)).To(o.Succeed())
		request["extraObjects"] = extra
		return request
	}
	return map[string]interface{}{
		"hostedCluster": map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1", "kind": "HostedCluster",
			"metadata": map[string]interface{}{"name": f.name, "namespace": f.ns},
			"spec": map[string]interface{}{
				"release":    map[string]string{"image": image},
				"pullSecret": map[string]string{"name": f.name + "-pull-secret"},
				"platform":   map[string]string{"type": "None"},
				"networking": map[string]string{"networkType": "OVNKubernetes"},
				"services": []interface{}{
					proxyService("APIServer"), proxyService("OAuthServer"), proxyService("Konnectivity"), proxyService("Ignition"),
				},
				"etcd": map[string]interface{}{
					"managementType": "Managed",
					"managed": map[string]interface{}{
						"storage": map[string]interface{}{
							"type":             "PersistentVolume",
							"persistentVolume": map[string]string{"size": "8Gi"},
						},
					},
				},
				"infraID": f.name,
			},
		},
		"nodePools": []interface{}{map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1", "kind": "NodePool",
			"metadata": map[string]interface{}{"name": f.name + "-workers", "namespace": f.ns},
			"spec": map[string]interface{}{
				"clusterName": f.name, "replicas": 0,
				"release":    map[string]string{"image": image},
				"platform":   map[string]string{"type": "None"},
				"management": map[string]string{"upgradeType": "Replace"},
			},
		}},
		"secrets": []interface{}{map[string]interface{}{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": map[string]string{"name": f.name + "-pull-secret"},
			"type":     "kubernetes.io/dockerconfigjson",
			"data":     map[string]string{".dockerconfigjson": "eyJhdXRocyI6e319"},
		}},
		"extraObjects": extra,
	}
}

func (f *proxyFixture) createBodyWithArch(extra []interface{}, arch string) map[string]interface{} {
	request := f.createBody(extra)
	nodePool := proxyObjects(request, "nodePools")[0]
	proxyObject(nodePool, "spec")["arch"] = arch
	return request
}

func (f *proxyFixture) incompatibleArch() string {
	version, e := f.spokeK8s.Discovery().ServerVersion()
	o.Expect(e).NotTo(o.HaveOccurred())
	if strings.Contains(version.Platform, "arm64") {
		return "amd64"
	}
	return "arm64"
}

func proxyProbeConfigMap(name string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": name},
		"data":     map[string]string{"probe": "must-not-apply"},
	}
}

func proxyService(name string) map[string]interface{} {
	return map[string]interface{}{"service": name, "servicePublishingStrategy": map[string]string{"type": "None"}}
}

func (f *proxyFixture) requireCreatePayload() {
	if hcpProxyCreatePayloadMissing {
		g.Skip("hcp-proxy create specs require HCP_PROXY_RELEASE_IMAGE, an options release image, or HCP_PROXY_CREATE_REQUEST_TEMPLATE")
	}
}

func (f *proxyFixture) create(extra []interface{}) map[string]interface{} {
	f.requireCreatePayload()
	status, body := f.request(http.MethodPost, f.collectionPath(), f.createBody(extra))
	expectProxyStatus(status, body, http.StatusCreated)
	return decodeProxyObject(body)
}

func (f *proxyFixture) getSpoke(gvr schema.GroupVersionResource, name string) *unstructured.Unstructured {
	var object *unstructured.Unstructured
	o.Eventually(func() error {
		var e error
		object, e = f.spokeDyn.Resource(gvr).Namespace(f.ns).Get(context.Background(), name, metav1.GetOptions{})
		return e
	}, 2*time.Minute, 5*time.Second).Should(o.Succeed())
	return object
}

func (f *proxyFixture) expectSpokeGone(gvr schema.GroupVersionResource, name string) {
	o.Eventually(func() bool {
		_, e := f.spokeDyn.Resource(gvr).Namespace(f.ns).Get(context.Background(), name, metav1.GetOptions{})
		return apierrors.IsNotFound(e)
	}, 2*time.Minute, 5*time.Second).Should(o.BeTrue(), "%s/%s must be removed from spoke", gvr.Resource, name)
}

func expectProxyFailure(actual int, body []byte, allowed ...int) {
	if len(allowed) > 0 {
		o.Expect(allowed).To(o.ContainElement(actual), "proxy response: %s", string(body))
	} else {
		o.Expect(actual).To(o.BeNumerically(">=", 400), "proxy response: %s", string(body))
	}
	o.Expect(strings.TrimSpace(string(body))).NotTo(o.BeEmpty(), "proxy failure response must contain an error message")
}

func expectProxyStatus(actual int, body []byte, wanted int) {
	o.Expect(actual).To(o.Equal(wanted), "proxy response: %s", string(body))
}

func decodeProxyObject(body []byte) map[string]interface{} {
	var object map[string]interface{}
	o.Expect(json.Unmarshal(body, &object)).To(o.Succeed(), "response: %s", string(body))
	return object
}

func proxyObject(parent map[string]interface{}, key string) map[string]interface{} {
	value, ok := parent[key].(map[string]interface{})
	o.Expect(ok).To(o.BeTrue(), "missing object %s in %#v", key, parent)
	return value
}

func proxyObjects(parent map[string]interface{}, key string) []map[string]interface{} {
	items, ok := parent[key].([]interface{})
	o.Expect(ok).To(o.BeTrue(), "missing list %s in %#v", key, parent)
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]interface{})
		o.Expect(ok).To(o.BeTrue())
		result = append(result, value)
	}
	return result
}

func proxyLabels(object map[string]interface{}) map[string]interface{} {
	metadata := proxyObject(object, "metadata")
	if metadata["labels"] == nil {
		metadata["labels"] = map[string]interface{}{}
	}
	return proxyObject(metadata, "labels")
}

func proxyName(object map[string]interface{}) string {
	name, ok := proxyObject(object, "metadata")["name"].(string)
	o.Expect(ok).To(o.BeTrue())
	return name
}

func proxyStringSlice(object map[string]interface{}, keys ...string) []string {
	for _, key := range keys[:len(keys)-1] {
		object = proxyObject(object, key)
	}
	values, ok := object[keys[len(keys)-1]].([]interface{})
	o.Expect(ok).To(o.BeTrue())
	result := make([]string, len(values))
	for i, value := range values {
		result[i], ok = value.(string)
		o.Expect(ok).To(o.BeTrue())
	}
	return result
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
