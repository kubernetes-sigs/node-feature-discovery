/*
Copyright 2020 The Kubernetes Authors.

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

	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubelet "k8s.io/kubernetes/test/e2e/framework/kubelet"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
)

var _ = ginkgo.Describe("[NFD] Node topology updater", func() {
	var (
		extClient           *extclient.Clientset
		topologyClient      *topologyclientset.Clientset
		crd                 *apiextensionsv1.CustomResourceDefinition
		topologyUpdaterNode *v1.Node
		kubeletConfig       *kubeletconfig.KubeletConfiguration
	)

	f := framework.NewDefaultFramework("node-topology-updater")

	ginkgo.BeforeEach(func() {
		var err error

		if extClient == nil {
			extClient, err = extclient.NewForConfig(f.ClientConfig())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		if topologyClient == nil {
			topologyClient, err = topologyclientset.NewForConfig(f.ClientConfig())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		ginkgo.By("Creating the node resource topologies CRD")
		crd, err = CreateNodeResourceTopologies(extClient)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = testutils.ConfigureRBAC(f.ClientSet, f.Namespace.Name)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
		f.PodClient().CreateSync(testutils.NFDMasterPod(image, false))

		// Create nfd-master service
		masterService, err := testutils.CreateService(f.ClientSet, f.Namespace.Name)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Waiting for the nfd-master service to be up")
		gomega.Expect(e2enetwork.WaitForService(f.ClientSet, f.Namespace.Name, masterService.Name, true, time.Second, 10*time.Second)).NotTo(gomega.HaveOccurred())

		ginkgo.By("Creating nfd-topology-updater daemonset")
		topologyUpdaterDaemonSet := testutils.NFDTopologyUpdaterDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})
		topologyUpdaterDaemonSet, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), topologyUpdaterDaemonSet, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ginkgo.By("Waiting for daemonset pods to be ready")
		gomega.Expect(e2epod.WaitForPodsReady(f.ClientSet, f.Namespace.Name, topologyUpdaterDaemonSet.Spec.Template.Labels["name"], 5)).NotTo(gomega.HaveOccurred())

		label := labels.SelectorFromSet(map[string]string{"name": topologyUpdaterDaemonSet.Spec.Template.Labels["name"]})
		pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{LabelSelector: label.String()})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pods.Items).ToNot(gomega.BeEmpty())

		topologyUpdaterNode, err = f.ClientSet.CoreV1().Nodes().Get(context.TODO(), pods.Items[0].Spec.NodeName, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		kubeletConfig, err = e2ekubelet.GetCurrentKubeletConfig(topologyUpdaterNode.Name, "", true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.Context("with single nfd-master pod", func() {
		ginkgo.It("should fill the node resource topologies CR with the data", func() {
			gomega.Eventually(func() bool {
				// TODO: we should avoid to use hardcoded namespace name
				nodeTopology, err := topologyClient.TopologyV1alpha1().NodeResourceTopologies("default").Get(context.TODO(), topologyUpdaterNode.Name, metav1.GetOptions{})
				if err != nil {
					framework.Logf("failed to get the node topology resource: %v", err)
					return false
				}

				if nodeTopology == nil || len(nodeTopology.TopologyPolicies) == 0 {
					framework.Logf("failed to get topology policy from the node topology resource")
					return false
				}

				if nodeTopology.TopologyPolicies[0] != (*kubeletConfig).TopologyManagerPolicy {
					return false
				}

				// TODO: add more checks like checking distances, NUMA node and allocated CPUs

				return true
			}, time.Minute, 5*time.Second).Should(gomega.BeTrue())
		})
	})

	ginkgo.JustAfterEach(func() {
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

const nodeResourceTopologiesName = "noderesourcetopologies.topology.node.k8s.io"

func newNodeResourceTopologies() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeResourceTopologiesName,
			Annotations: map[string]string{
				"api-approved.kubernetes.io": "https://github.com/kubernetes/enhancements/pull/1870",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "topology.node.k8s.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "noderesourcetopologies",
				Singular: "noderesourcetopology",
				ShortNames: []string{
					"node-res-topo",
				},
				Kind: "NodeResourceTopology",
			},
			Scope: "Namespaced",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"topologyPolicies": {
									Type: "array",
									Items: &apiextensionsv1.JSONSchemaPropsOrArray{
										Schema: &apiextensionsv1.JSONSchemaProps{
											Type: "string",
										},
									},
								},
							},
						},
					},
					Served:  true,
					Storage: true,
				},
			},
		},
	}
}

func CreateNodeResourceTopologies(extClient extclient.Interface) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd, err := extClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), nodeResourceTopologiesName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return crd, nil
	}

	crd, err = extClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), newNodeResourceTopologies(), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return crd, nil
}
