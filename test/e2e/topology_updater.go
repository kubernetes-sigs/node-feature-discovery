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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"

	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubelet "k8s.io/kubernetes/test/e2e/framework/kubelet"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"

	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
)

var _ = SIGDescribe("Node Feature Discovery topology updater", func() {
	var (
		extClient           *extclient.Clientset
		topologyClient      *topologyclientset.Clientset
		crd                 *apiextensionsv1.CustomResourceDefinition
		topologyUpdaterNode *v1.Node
		workerNodes         []v1.Node
		kubeletConfig       *kubeletconfig.KubeletConfiguration
	)

	f := framework.NewDefaultFramework("node-topology-updater")

	BeforeEach(func() {
		var err error

		if extClient == nil {
			extClient, err = extclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		}

		if topologyClient == nil {
			topologyClient, err = topologyclientset.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		}

		cfg, err := testutils.GetConfig()
		Expect(err).ToNot(HaveOccurred())

		kcfg := cfg.GetKubeletConfig()
		By(fmt.Sprintf("Using config (%#v)", kcfg))

		By("Creating the node resource topologies CRD")
		crd, err = testutils.CreateNodeResourceTopologies(extClient)
		Expect(err).NotTo(HaveOccurred())

		err = testutils.ConfigureRBAC(f.ClientSet, f.Namespace.Name)
		Expect(err).NotTo(HaveOccurred())

		image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
		f.PodClient().CreateSync(testutils.NFDMasterPod(image, false))

		// Create nfd-master service
		masterService, err := testutils.CreateService(f.ClientSet, f.Namespace.Name)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for the nfd-master service to be up")
		Expect(e2enetwork.WaitForService(f.ClientSet, f.Namespace.Name, masterService.Name, true, time.Second, 10*time.Second)).NotTo(HaveOccurred())

		By("Creating nfd-topology-updater daemonset")
		topologyUpdaterDaemonSet := testutils.NFDTopologyUpdaterDaemonSet(kcfg, fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})
		topologyUpdaterDaemonSet, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), topologyUpdaterDaemonSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for daemonset pods to be ready")
		Expect(testutils.WaitForPodsReady(f.ClientSet, f.Namespace.Name, topologyUpdaterDaemonSet.Spec.Template.Labels["name"], 5)).NotTo(HaveOccurred())

		label := labels.SelectorFromSet(map[string]string{"name": topologyUpdaterDaemonSet.Spec.Template.Labels["name"]})
		pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{LabelSelector: label.String()})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).ToNot(BeEmpty())

		topologyUpdaterNode, err = f.ClientSet.CoreV1().Nodes().Get(context.TODO(), pods.Items[0].Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		kubeletConfig, err = e2ekubelet.GetCurrentKubeletConfig(topologyUpdaterNode.Name, "", true)
		Expect(err).NotTo(HaveOccurred())

		workerNodes, err = testutils.GetWorkerNodes(f)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with single nfd-master pod", func() {
		It("should fill the node resource topologies CR with the data", func() {
			nodeTopology := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)
			isValid := testutils.IsValidNodeTopology(nodeTopology, kubeletConfig)
			Expect(isValid).To(BeTrue(), "received invalid topology: %v", nodeTopology)
		})

		It("it should not account for any cpus if a container doesn't request exclusive cpus (best effort QOS)", func() {
			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming resources from the shared, non-exclusive CPU pool (best-effort QoS)")
			sleeperPod := testutils.BestEffortSleeperPod()

			podMap := make(map[string]*v1.Pod)
			pod := f.PodClient().CreateSync(sleeperPod)
			podMap[pod.Name] = pod
			defer testutils.DeletePodsAsync(f, podMap)

			cooldown := 30 * time.Second
			By(fmt.Sprintf("getting the updated topology - sleeping for %v", cooldown))
			// the object, hance the resource version must NOT change, so we can only sleep
			time.Sleep(cooldown)
			By("checking the changes in the updated topology - expecting none")
			finalNodeTopo := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)

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

		It("it should not account for any cpus if a container doesn't request exclusive cpus (guaranteed QOS, nonintegral cpu request)", func() {
			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming resources from the shared, non-exclusive CPU pool (guaranteed QoS, nonintegral request)")
			sleeperPod := testutils.GuaranteedSleeperPod("500m")

			podMap := make(map[string]*v1.Pod)
			pod := f.PodClient().CreateSync(sleeperPod)
			podMap[pod.Name] = pod
			defer testutils.DeletePodsAsync(f, podMap)

			cooldown := 30 * time.Second
			By(fmt.Sprintf("getting the updated topology - sleeping for %v", cooldown))
			// the object, hance the resource version must NOT change, so we can only sleep
			time.Sleep(cooldown)
			By("checking the changes in the updated topology - expecting none")
			finalNodeTopo := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)

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

		It("it should account for containers requesting exclusive cpus", func() {
			nodes, err := testutils.FilterNodesWithEnoughCores(workerNodes, "1000m")
			Expect(err).NotTo(HaveOccurred())
			if len(nodes) < 1 {
				Skip("not enough allocatable cores for this test")
			}

			By("getting the initial topology information")
			initialNodeTopo := testutils.GetNodeTopology(topologyClient, topologyUpdaterNode.Name)
			By("creating a pod consuming exclusive CPUs")
			sleeperPod := testutils.GuaranteedSleeperPod("1000m")

			podMap := make(map[string]*v1.Pod)
			pod := f.PodClient().CreateSync(sleeperPod)
			podMap[pod.Name] = pod
			defer testutils.DeletePodsAsync(f, podMap)

			By("getting the updated topology")
			var finalNodeTopo *v1alpha1.NodeResourceTopology
			Eventually(func() bool {
				finalNodeTopo, err = topologyClient.TopologyV1alpha1().NodeResourceTopologies().Get(context.TODO(), topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("failed to get the node topology resource: %v", err)
					return false
				}
				return finalNodeTopo.ObjectMeta.ResourceVersion != initialNodeTopo.ObjectMeta.ResourceVersion
			}, time.Minute, 5*time.Second).Should(BeTrue(), "didn't get updated node topology info")
			By("checking the changes in the updated topology")

			initialAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(initialNodeTopo)
			finalAllocRes := testutils.AllocatableResourceListFromNodeResourceTopology(finalNodeTopo)
			if len(initialAllocRes) == 0 || len(finalAllocRes) == 0 {
				Fail(fmt.Sprintf("failed to find allocatable resources from node topology initial=%v final=%v", initialAllocRes, finalAllocRes))
			}
			zoneName, resName, isLess := lessAllocatableResources(initialAllocRes, finalAllocRes)
			framework.Logf("zone=%q resource=%q isLess=%v", zoneName, resName, isLess)
			Expect(isLess).To(BeTrue(), fmt.Sprintf("final allocatable resources not decreased - initial=%v final=%v", initialAllocRes, finalAllocRes))
		})

	})

	JustAfterEach(func() {
		err := testutils.DeconfigureRBAC(f.ClientSet, f.Namespace.Name)
		if err != nil {
			framework.Logf("failed to delete RBAC resources: %v", err)
		}

		err = extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.TODO(), crd.Name, metav1.DeleteOptions{})
		if err != nil {
			framework.Logf("failed to delete node resources topologies CRD: %v", err)
		}
	})
})

// lessAllocatableResources specialize CompareAllocatableResources for this specific e2e use case.
func lessAllocatableResources(expected, got map[string]v1.ResourceList) (string, string, bool) {
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
