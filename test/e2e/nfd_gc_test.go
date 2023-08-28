/*
Copyright 2023 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	topologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	topologyclient "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	"sigs.k8s.io/node-feature-discovery/test/e2e/utils"
	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
	testdeploy "sigs.k8s.io/node-feature-discovery/test/e2e/utils/deployment"
	testpod "sigs.k8s.io/node-feature-discovery/test/e2e/utils/pod"
)

// Actual test suite
var _ = SIGDescribe("NFD GC", func() {
	f := framework.NewDefaultFramework("nfd-gc")

	Context("when deploying nfd-gc", Ordered, func() {
		var (
			crds           []*apiextensionsv1.CustomResourceDefinition
			extClient      *extclient.Clientset
			nfdClient      *nfdclient.Clientset
			topologyClient *topologyclient.Clientset
		)

		BeforeAll(func(ctx context.Context) {
			// Create clients for apiextensions and our CRD api
			extClient = extclient.NewForConfigOrDie(f.ClientConfig())
			nfdClient = nfdclient.NewForConfigOrDie(f.ClientConfig())
			topologyClient = topologyclient.NewForConfigOrDie(f.ClientConfig())

			By("Creating CRDs")
			var err error
			crds, err = testutils.CreateNfdCRDs(ctx, extClient)
			Expect(err).NotTo(HaveOccurred())
			crd, err := testutils.CreateNodeResourceTopologies(ctx, extClient)
			Expect(err).NotTo(HaveOccurred())
			crds = append(crds, crd)
		})

		AfterAll(func(ctx context.Context) {
			for _, crd := range crds {
				err := extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}
		})

		JustBeforeEach(func(ctx context.Context) {
			err := testutils.ConfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			cleanupCRs(ctx, nfdClient, f.Namespace.Name)
			cleanupNRTs(ctx, topologyClient)
		})

		AfterEach(func(ctx context.Context) {
			Expect(testutils.DeconfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)).NotTo(HaveOccurred())
			cleanupCRs(ctx, nfdClient, f.Namespace.Name)
			cleanupNRTs(ctx, topologyClient)
		})

		// Helper functions
		createCRs := func(ctx context.Context, nodeNames []string) error {
			for _, name := range nodeNames {
				if err := utils.CreateNodeFeature(ctx, nfdClient, f.Namespace.Name, name, name); err != nil {
					return err
				}
				if err := utils.CreateNodeResourceTopology(ctx, topologyClient, name); err != nil {
					return err
				}
				framework.Logf("CREATED CRS FOR node %q", name)
			}
			return nil
		}

		getNodeFeatures := func(ctx context.Context) ([]v1alpha1.NodeFeature, error) {
			nfl, err := nfdClient.NfdV1alpha1().NodeFeatures("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return nfl.Items, nil
		}

		getNodeResourceTopologies := func(ctx context.Context) ([]topologyv1alpha2.NodeResourceTopology, error) {
			nrtl, err := topologyClient.TopologyV1alpha2().NodeResourceTopologies().List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return nrtl.Items, nil
		}

		//
		// Test GC at startup
		//
		Context("with pre-existing NodeFeature and NodeResourceTopology objects", func() {
			It("it should delete stale objects at startup", func(ctx context.Context) {
				nodes, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				targetNodeNames := []string{nodes.Items[0].GetName(), nodes.Items[len(nodes.Items)-1].GetName()}
				staleNodeNames := []string{"non-existent-node-1", "non-existent-node-2"}

				// Create NodeFeature and NodeResourceTopology objects
				By("Creating CRs")
				Expect(createCRs(ctx, targetNodeNames)).NotTo(HaveOccurred())
				Expect(createCRs(ctx, staleNodeNames)).NotTo(HaveOccurred())

				// Deploy nfd-gc
				By("Creating nfd-gc deployment")
				podSpecOpts := []testpod.SpecOption{testpod.SpecWithContainerImage(dockerImage())}
				gcDeploy := testdeploy.NFDGC(podSpecOpts...)
				gcDeploy, err = f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Create(ctx, gcDeploy, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for gc deployment pods to be ready")
				Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, gcDeploy.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

				// Check that only expected objects exist
				By("Verifying CRs")
				Eventually(getNodeFeatures).WithPolling(1 * time.Second).WithTimeout(3 * time.Second).WithContext(ctx).Should(ConsistOf(haveNames(targetNodeNames...)...))
				Eventually(getNodeResourceTopologies).WithPolling(1 * time.Second).WithTimeout(3 * time.Second).WithContext(ctx).Should(ConsistOf(haveNames(targetNodeNames...)...))
			})
		})

		//
		// Test periodic GC
		//
		Context("with stale NodeFeature and NodeResourceTopology objects appearing", func() {
			It("it should remove stale objects", func(ctx context.Context) {
				nodes, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				targetNodeNames := []string{nodes.Items[0].GetName(), nodes.Items[len(nodes.Items)-1].GetName()}
				staleNodeNames := []string{"non-existent-node-2.1", "non-existent-node-2.2"}

				// Deploy nfd-gc
				By("Creating nfd-gc deployment")
				podSpecOpts := []testpod.SpecOption{
					testpod.SpecWithContainerImage(dockerImage()),
					testpod.SpecWithContainerExtraArgs("-gc-interval", "1s"),
				}
				gcDeploy := testdeploy.NFDGC(podSpecOpts...)
				gcDeploy, err = f.ClientSet.AppsV1().Deployments(f.Namespace.Name).Create(ctx, gcDeploy, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for gc deployment pods to be ready")
				Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, gcDeploy.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

				// Create NodeFeature and NodeResourceTopology objects
				By("Creating CRs")
				Expect(createCRs(ctx, targetNodeNames)).NotTo(HaveOccurred())
				Expect(createCRs(ctx, staleNodeNames)).NotTo(HaveOccurred())

				// Check that only expected objects exist
				By("Verifying CRs")
				Eventually(getNodeFeatures).WithPolling(1 * time.Second).WithTimeout(3 * time.Second).WithContext(ctx).Should(ConsistOf(haveNames(targetNodeNames...)...))
				Eventually(getNodeResourceTopologies).WithPolling(1 * time.Second).WithTimeout(3 * time.Second).WithContext(ctx).Should(ConsistOf(haveNames(targetNodeNames...)...))
			})
		})
	})
})

// haveNames is a helper that returns a slice of Gomega matchers for asserting the names of k8s API objects
func haveNames(names ...string) []interface{} {
	m := make([]interface{}, len(names))
	for i, n := range names {
		m[i] = HaveField("Name", n)
	}
	return m
}

func cleanupNRTs(ctx context.Context, cli *topologyclient.Clientset) {
	By("Deleting NodeResourceTopology objects from the cluster")
	err := cli.TopologyV1alpha2().NodeResourceTopologies().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
}
