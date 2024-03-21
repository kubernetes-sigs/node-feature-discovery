/*
Copyright 2020-2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	"github.com/k8stopologyawareschedwg/podfingerprint"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubeletconfig "k8s.io/kubernetes/test/e2e_node/kubeletconfig"
	admissionapi "k8s.io/pod-security-admission/api"

	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
	testds "sigs.k8s.io/node-feature-discovery/test/e2e/utils/daemonset"
	testpod "sigs.k8s.io/node-feature-discovery/test/e2e/utils/pod"
)

var _ = NFDDescribe(Label("nfd-topology-updater"), func() {
	var (
		extClient                *extclient.Clientset
		topologyClient           *topologyclientset.Clientset
		topologyUpdaterNode      *corev1.Node
		topologyUpdaterDaemonSet *appsv1.DaemonSet
		workerNodes              []corev1.Node
		kubeletConfig            *kubeletconfig.KubeletConfiguration
	)

	f := framework.NewDefaultFramework("node-topology-updater")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	JustBeforeEach(func(ctx context.Context) {
		var err error

		if extClient == nil {
			extClient, err = extclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		}

		if topologyClient == nil {
			topologyClient, err = topologyclientset.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		}

		By("Creating the node resource topologies CRD")
		Expect(testutils.CreateNodeResourceTopologies(ctx, extClient)).ToNot(BeNil())

		By("Configuring RBAC")
		Expect(testutils.ConfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)).NotTo(HaveOccurred())

		By("Creating nfd-topology-updater daemonset")
		topologyUpdaterDaemonSet, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(ctx, topologyUpdaterDaemonSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for daemonset pods to be ready")
		Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, topologyUpdaterDaemonSet.Spec.Template.Labels["name"], 5)).NotTo(HaveOccurred())

		label := labels.SelectorFromSet(map[string]string{"name": topologyUpdaterDaemonSet.Spec.Template.Labels["name"]})
		pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, metav1.ListOptions{LabelSelector: label.String()})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).ToNot(BeEmpty())

		topologyUpdaterNode, err = f.ClientSet.CoreV1().Nodes().Get(ctx, pods.Items[0].Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		kubeletConfig, err = e2ekubeletconfig.GetCurrentKubeletConfig(ctx, topologyUpdaterNode.Name, "", true, false)
		Expect(err).NotTo(HaveOccurred())

		workerNodes, err = testutils.GetWorkerNodes(ctx, f)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx context.Context) {
		framework.Logf("Node Feature Discovery topology updater CRD and RBAC removal")
		err := testutils.DeconfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)
		if err != nil {
			framework.Failf("AfterEach: Failed to delete RBAC resources: %v", err)
		}
	})

	Context("with topology-updater daemonset running", func() {
		BeforeEach(func(ctx context.Context) {
			cfg, err := testutils.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			kcfg := cfg.GetKubeletConfig()
			By(fmt.Sprintf("Using config (%#v)", kcfg))
			podSpecOpts := []testpod.SpecOption{testpod.SpecWithContainerImage(dockerImage()), testpod.SpecWithContainerExtraArgs("-sleep-interval=3s")}
			topologyUpdaterDaemonSet = testds.NFDTopologyUpdater(kcfg, podSpecOpts...)
		})

		It("should fill the node resource topologies CR with the data", func(ctx context.Context) {
			nodeTopology := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)
			isValid := testutils.IsValidNodeTopology(nodeTopology, kubeletConfig)
			Expect(isValid).To(BeTrue(), "received invalid topology: %v", nodeTopology)
		})

		It("it should not account for any cpus if a container doesn't request exclusive cpus (best effort QOS)", func(ctx context.Context) {
			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming resources from the shared, non-exclusive CPU pool (best-effort QoS)")
			sleeperPod := testpod.BestEffortSleeper()

			podMap := make(map[string]*corev1.Pod)
			pod := e2epod.NewPodClient(f).CreateSync(ctx, sleeperPod)
			podMap[pod.Name] = pod
			defer testpod.DeleteAsync(ctx, f, podMap)

			cooldown := 30 * time.Second
			By(fmt.Sprintf("getting the updated topology - sleeping for %v", cooldown))
			// the object, hance the resource version must NOT change, so we can only sleep
			time.Sleep(cooldown)
			By("checking the changes in the updated topology - expecting none")
			finalNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)

			initialAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(initialNodeTopo)
			finalAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(finalNodeTopo)
			if len(initialAllocRes) == 0 || len(finalAllocRes) == 0 {
				Fail(fmt.Sprintf("failed to find allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
			}
			zoneName, resName, cmp, ok := testutils.CompareAllocatableResources(initialAllocRes, finalAllocRes)
			framework.Logf("zone=%q resource=%q cmp=%v ok=%v", zoneName, resName, cmp, ok)
			if !ok {
				Fail(fmt.Sprintf("failed to compare allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
			}

			// This is actually a workaround.
			// Depending on the (random, by design) order on which ginkgo runs the tests, a test which exclusively allocates CPUs may run before.
			// We cannot (nor should) care about what runs before this test, but we know that this may happen.
			// The proper solution is to wait for ALL the container requesting exclusive resources to be gone before to end the related test.
			// To date, we don't yet have a clean way to wait for these pod (actually containers) to be completely gone
			// (hence, releasing the exclusively allocated CPUs) before to end the test, so this test can run with some leftovers hanging around,
			// which makes the accounting harder. And this is what we handle here.
			isGreaterEqual := (cmp >= 0)
			Expect(isGreaterEqual).To(BeTrue(), fmt.Sprintf("final allocatable resources not restored - cmp=%d initial=%v final=%v", cmp, initialAllocRes, finalAllocRes))
		})

		It("it should not account for any cpus if a container doesn't request exclusive cpus (guaranteed QOS, nonintegral cpu request)", func(ctx context.Context) {
			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming resources from the shared, non-exclusive CPU pool (guaranteed QoS, nonintegral request)")
			sleeperPod := testpod.GuaranteedSleeper(testpod.WithLimits(
				corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
					// any random reasonable amount is fine
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				}))

			podMap := make(map[string]*corev1.Pod)
			pod := e2epod.NewPodClient(f).CreateSync(ctx, sleeperPod)
			podMap[pod.Name] = pod
			defer testpod.DeleteAsync(ctx, f, podMap)

			cooldown := 30 * time.Second
			By(fmt.Sprintf("getting the updated topology - sleeping for %v", cooldown))
			// the object, hence the resource version must NOT change, so we can only sleep
			time.Sleep(cooldown)
			By("checking the changes in the updated topology - expecting none")
			finalNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)

			initialAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(initialNodeTopo)
			finalAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(finalNodeTopo)
			if len(initialAllocRes) == 0 || len(finalAllocRes) == 0 {
				Fail(fmt.Sprintf("failed to find allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
			}
			zoneName, resName, cmp, ok := testutils.CompareAllocatableResources(initialAllocRes, finalAllocRes)
			framework.Logf("zone=%q resource=%q cmp=%v ok=%v", zoneName, resName, cmp, ok)
			if !ok {
				Fail(fmt.Sprintf("failed to compare allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
			}

			// This is actually a workaround.
			// Depending on the (random, by design) order on which ginkgo runs the tests, a test which exclusively allocates CPUs may run before.
			// We cannot (nor should) care about what runs before this test, but we know that this may happen.
			// The proper solution is to wait for ALL the container requesting exclusive resources to be gone before to end the related test.
			// To date, we don't yet have a clean way to wait for these pod (actually containers) to be completely gone
			// (hence, releasing the exclusively allocated CPUs) before to end the test, so this test can run with some leftovers hanging around,
			// which makes the accounting harder. And this is what we handle here.
			isGreaterEqual := (cmp >= 0)
			Expect(isGreaterEqual).To(BeTrue(), fmt.Sprintf("final allocatable resources not restored - cmp=%d initial=%v final=%v", cmp, initialAllocRes, finalAllocRes))
		})

		It("it should account for containers requesting exclusive cpus", func(ctx context.Context) {
			nodes, err := testutils.FilterNodesWithEnoughCores(workerNodes, "1000m")
			Expect(err).NotTo(HaveOccurred())
			if len(nodes) < 1 {
				Skip("not enough allocatable cores for this test")
			}

			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming exclusive CPUs")
			sleeperPod := testpod.GuaranteedSleeper(testpod.WithLimits(
				corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1000m"),
					// any random reasonable amount is fine
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				}))
			// in case there is more than a single node in the cluster
			// we need to set the node name, so we'll have certainty about
			// which node we need to examine
			sleeperPod.Spec.NodeName = topologyUpdaterNode.Name

			podMap := make(map[string]*corev1.Pod)
			pod := e2epod.NewPodClient(f).CreateSync(ctx, sleeperPod)
			podMap[pod.Name] = pod
			defer testpod.DeleteAsync(ctx, f, podMap)

			By("checking the changes in the updated topology")
			var finalNodeTopo *v1alpha2.NodeResourceTopology
			Eventually(func() bool {
				finalNodeTopo, err = topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("failed to get the node topology resource: %v", err)
					return false
				}
				if finalNodeTopo.ObjectMeta.ResourceVersion == initialNodeTopo.ObjectMeta.ResourceVersion {
					framework.Logf("node topology resource %s was not updated", topologyUpdaterNode.Name)
				}

				initialAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(initialNodeTopo)
				finalAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(finalNodeTopo)
				if len(initialAllocRes) == 0 || len(finalAllocRes) == 0 {
					Fail(fmt.Sprintf("failed to find allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
				}

				zoneName, resName, isLess := lessAllocatableResources(initialAllocRes, finalAllocRes)
				framework.Logf("zone=%q resource=%q isLess=%v", zoneName, resName, isLess)
				if !isLess {
					framework.Logf("final allocatable resources not decreased - initial=%v final=%v", initialAllocRes, finalAllocRes)
				}
				return true
			}, time.Minute, 5*time.Second).Should(BeTrue(), "didn't get updated node topology info")
		})

		It("should check that that topology object is garbage colleted", func(ctx context.Context) {

			By("Check if the topology object has owner reference")
			ns, err := f.ClientSet.CoreV1().Namespaces().Get(ctx, f.Namespace.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			t, err := topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, topologyUpdaterNode.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			owned := false
			for _, r := range t.OwnerReferences {
				if r.UID == ns.UID {
					owned = true
					break
				}
			}
			Expect(owned).Should(BeTrue())

			By("Deleting the nfd-topology namespace")
			err = f.ClientSet.CoreV1().Namespaces().Delete(ctx, f.Namespace.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("checking that topology was garbage collected")
			Eventually(func() bool {
				t, err := topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						framework.Logf("missing node topology resource for %q", topologyUpdaterNode.Name)
						return false
					}
					framework.Logf("failed to get the node topology resource: %v", err)
					return true
				}
				framework.Logf("topology resource: %v", t)
				return true
			}).WithPolling(15 * time.Second).WithTimeout(60 * time.Second).Should(BeFalse())
		})
	})

	When("sleep interval disabled", func() {
		BeforeEach(func(ctx context.Context) {
			cfg, err := testutils.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			kcfg := cfg.GetKubeletConfig()
			By(fmt.Sprintf("Using config (%#v)", kcfg))
			podSpecOpts := []testpod.SpecOption{testpod.SpecWithContainerImage(dockerImage()), testpod.SpecWithContainerExtraArgs("-sleep-interval=0s")}
			topologyUpdaterDaemonSet = testds.NFDTopologyUpdater(kcfg, podSpecOpts...)
		})
		It("should still create CRs using a reactive updates", func(ctx context.Context) {
			nodes, err := testutils.FilterNodesWithEnoughCores(workerNodes, "1000m")
			Expect(err).NotTo(HaveOccurred())
			if len(nodes) < 1 {
				Skip("not enough allocatable cores for this test")
			}

			By("creating a pod consuming exclusive CPUs")
			sleeperPod := testpod.GuaranteedSleeper(testpod.WithLimits(
				corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1000m"),
					// any random reasonable amount is fine
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				}))
			// in case there is more than a single node in the cluster
			// we need to set the node name, so we'll have certainty about
			// which node we need to examine
			sleeperPod.Spec.NodeName = topologyUpdaterNode.Name

			podMap := make(map[string]*corev1.Pod)
			pod := e2epod.NewPodClient(f).CreateSync(ctx, sleeperPod)
			podMap[pod.Name] = pod
			defer testpod.DeleteAsync(ctx, f, podMap)

			By("checking initial CR created")
			initialNodeTopo := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)

			By("creating additional pod consuming exclusive CPUs")
			sleeperPod2 := testpod.GuaranteedSleeper(testpod.WithLimits(
				corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1000m"),
					// any random reasonable amount is fine
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				}))

			// in case there is more than a single node in the cluster
			// we need to set the node name, so we'll have certainty about
			// which node we need to examine
			sleeperPod2.Spec.NodeName = topologyUpdaterNode.Name
			sleeperPod2.Name = sleeperPod2.Name + "2"
			pod2 := e2epod.NewPodClient(f).CreateSync(ctx, sleeperPod2)
			podMap[pod.Name] = pod2

			By("checking the changes in the updated topology")
			var finalNodeTopo *v1alpha2.NodeResourceTopology
			Eventually(func() bool {
				finalNodeTopo, err = topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("failed to get the node topology resource: %v", err)
					return false
				}
				if finalNodeTopo.ObjectMeta.ResourceVersion == initialNodeTopo.ObjectMeta.ResourceVersion {
					framework.Logf("node topology resource %s was not updated", topologyUpdaterNode.Name)
				}

				initialAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(initialNodeTopo)
				finalAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(finalNodeTopo)
				if len(initialAllocRes) == 0 || len(finalAllocRes) == 0 {
					Fail(fmt.Sprintf("failed to find allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
				}

				zoneName, resName, isLess := lessAllocatableResources(initialAllocRes, finalAllocRes)
				framework.Logf("zone=%q resource=%q isLess=%v", zoneName, resName, isLess)
				if !isLess {
					framework.Logf("final allocatable resources not decreased - initial=%v final=%v", initialAllocRes, finalAllocRes)
				}
				return true
				// timeout must be lower than sleep interval
				// otherwise we won't be able to determine what
				// triggered the CR update
			}, time.Second*20, 5*time.Second).Should(BeTrue(), "didn't get updated node topology info")
		})
	})

	When("topology-updater configure to exclude memory", func() {
		BeforeEach(func(ctx context.Context) {
			cm := testutils.NewConfigMap("nfd-topology-updater-conf", "nfd-topology-updater.conf", `
excludeList:
  '*': [memory]
`)
			cm, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			cfg, err := testutils.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			kcfg := cfg.GetKubeletConfig()
			By(fmt.Sprintf("Using config (%#v)", kcfg))

			podSpecOpts := []testpod.SpecOption{
				testpod.SpecWithContainerImage(dockerImage()),
				testpod.SpecWithConfigMap(cm.Name, "/etc/kubernetes/node-feature-discovery"),
			}
			topologyUpdaterDaemonSet = testds.NFDTopologyUpdater(kcfg, podSpecOpts...)
		})

		It("noderesourcetopology should not advertise the memory resource", func(ctx context.Context) {
			Eventually(func() bool {
				memoryFound := false
				nodeTopology := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)
				for _, zone := range nodeTopology.Zones {
					for _, res := range zone.Resources {
						if res.Name == string(corev1.ResourceMemory) {
							memoryFound = true
							framework.Logf("resource:%s was found for nodeTopology:%s on zone:%s while it should not", corev1.ResourceMemory, nodeTopology.Name, zone.Name)
							break
						}
					}
				}
				return memoryFound
			}, 2*time.Minute, 10*time.Second).Should(BeFalse())
		})
	})

	When("kubelet state monitoring disabled", func() {
		BeforeEach(func(ctx context.Context) {
			cfg, err := testutils.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			kcfg := cfg.GetKubeletConfig()
			By(fmt.Sprintf("Using config (%#v)", kcfg))
			// we need a predictable and "low enough" sleep interval to make sure we wait enough time, and still we don't want to waste too much time waiting
			podSpecOpts := []testpod.SpecOption{testpod.SpecWithContainerImage(dockerImage()), testpod.SpecWithContainerExtraArgs("-kubelet-state-dir=", "-sleep-interval=3s")}
			topologyUpdaterDaemonSet = testds.NFDTopologyUpdater(kcfg, podSpecOpts...)
		})

		It("should still create or update CRs with periodic updates", func(ctx context.Context) {
			// this is the simplest test. A more refined test would be check updates. We do like this to minimize flakes.
			By("deleting existing CRs")

			err := topologyClient.TopologyV1alpha2().NodeResourceTopologies().Delete(ctx, topologyUpdaterNode.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			// need to set the polling interval explicitly and bigger than the sleep interval
			By("checking the topology was recreated or updated")
			Eventually(func() bool {
				_, err = topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						framework.Logf("missing node topology resource for %q", topologyUpdaterNode.Name)
						return true // intentionally retry
					}
					framework.Logf("failed to get the node topology resource: %v", err)
					return false
				}
				return true
			}).WithPolling(5 * time.Second).WithTimeout(30 * time.Second).Should(BeTrue())

			framework.Logf("found NRT data for node %q!", topologyUpdaterNode.Name)
		})
	})

	When("topology-updater configure to compute pod fingerprint", func() {
		BeforeEach(func(ctx context.Context) {
			cfg, err := testutils.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			kcfg := cfg.GetKubeletConfig()
			By(fmt.Sprintf("Using config (%#v)", kcfg))

			podSpecOpts := []testpod.SpecOption{
				testpod.SpecWithContainerImage(dockerImage()),
				testpod.SpecWithContainerExtraArgs("-pods-fingerprint"),
			}
			topologyUpdaterDaemonSet = testds.NFDTopologyUpdater(kcfg, podSpecOpts...)
		})
		It("noderesourcetopology should advertise pod fingerprint in top-level attribute", func(ctx context.Context) {
			Eventually(func() bool {
				// get node topology
				nodeTopology := testutils.GetNodeTopology(ctx, topologyClient, topologyUpdaterNode.Name)

				// look for attribute
				podFingerprintAttribute, err := findAttribute(nodeTopology.Attributes, podfingerprint.Attribute)
				if err != nil {
					framework.Logf("podFingerprint attributte %q not found:  %v", podfingerprint.Attribute, err)
					return false
				}
				// get pods in node
				pods, err := f.ClientSet.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + topologyUpdaterNode.Name})
				if err != nil {
					framework.Logf("podFingerprint error while recovering %q node pods: %v", topologyUpdaterNode.Name, err)
					return false
				}
				if len(pods.Items) == 0 {
					framework.Logf("podFingerprint No pods in node  %q", topologyUpdaterNode.Name)
					return false
				}

				// compute expected value
				pf := podfingerprint.NewFingerprint(len(pods.Items))
				for _, pod := range pods.Items {
					err = pf.Add(pod.Namespace, pod.Name)
					if err != nil {
						framework.Logf("error while computing expected podFingerprint %v", err)
						return false
					}
				}
				expectedPodFingerprint := pf.Sign()

				if podFingerprintAttribute.Value != expectedPodFingerprint {
					framework.Logf("podFingerprint attributte error expected: %q actual: %q", expectedPodFingerprint, podFingerprintAttribute.Value)
					return false
				}

				return true

			}, 1*time.Minute, 10*time.Second).Should(BeTrue())
		})
	})
})

func findAttribute(attributes v1alpha2.AttributeList, attributeName string) (v1alpha2.AttributeInfo, error) {
	for _, attrInfo := range attributes {
		if attrInfo.Name == attributeName {
			return attrInfo, nil
		}
	}
	return v1alpha2.AttributeInfo{}, fmt.Errorf("attribute %q not found", attributeName)
}

// lessAllocatableResources specialize CompareAllocatableResources for this specific e2e use case.
func lessAllocatableResources(expected, got map[string]corev1.ResourceList) (string, string, bool) {
	zoneName, resName, cmp, ok := testutils.CompareAllocatableResources(expected, got)
	if !ok {
		framework.Logf("-> cmp failed (not ok)")
		return "", "", false
	}
	if cmp < 0 {
		return zoneName, resName, true
	}
	framework.Logf("-> cmp failed (value=%d)", cmp)
	return "", "", false
}
