/*
Copyright 2018 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	taintutils "k8s.io/kubernetes/pkg/util/taints"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	"sigs.k8s.io/node-feature-discovery/source/custom"
	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
	testds "sigs.k8s.io/node-feature-discovery/test/e2e/utils/daemonset"
	testpod "sigs.k8s.io/node-feature-discovery/test/e2e/utils/pod"
)

// cleanupNode deletes all NFD-related metadata from the Node object, i.e.
// labels and annotations
func cleanupNode(ctx context.Context, cs clientset.Interface) {
	// Per-node cleanup function
	cleanup := func(node *corev1.Node) error {
		update := false
		updateStatus := false
		// Gather info about all NFD-managed node assets outside the default prefix
		nfdLabels := map[string]struct{}{}
		for _, name := range strings.Split(node.Annotations[nfdv1alpha1.FeatureLabelsAnnotation], ",") {
			if strings.Contains(name, "/") {
				nfdLabels[name] = struct{}{}
			}
		}
		nfdAnnotations := map[string]struct{}{}
		for _, name := range strings.Split(node.Annotations[nfdv1alpha1.FeatureAnnotationsTrackingAnnotation], ",") {
			if strings.Contains(name, "/") {
				nfdAnnotations[name] = struct{}{}
			}
		}
		nfdERs := map[string]struct{}{}
		for _, name := range strings.Split(node.Annotations[nfdv1alpha1.ExtendedResourceAnnotation], ",") {
			if strings.Contains(name, "/") {
				nfdERs[name] = struct{}{}
			}
		}

		// Remove labels
		for key := range node.Labels {
			_, ok := nfdLabels[key]
			if ok || strings.HasPrefix(key, nfdv1alpha1.FeatureLabelNs) {
				delete(node.Labels, key)
				update = true
			}
		}

		// Remove annotations
		for key := range node.Annotations {
			_, ok := nfdAnnotations[key]
			if ok || strings.HasPrefix(key, nfdv1alpha1.AnnotationNs) || strings.HasPrefix(key, nfdv1alpha1.FeatureAnnotationNs) {
				delete(node.Annotations, key)
				update = true
			}
		}

		// Remove taints
		for _, taint := range node.Spec.Taints {
			if strings.HasPrefix(taint.Key, nfdv1alpha1.TaintNs) {
				newTaints, removed := taintutils.DeleteTaint(node.Spec.Taints, &taint)
				if removed {
					node.Spec.Taints = newTaints
					update = true
				}
			}
		}

		// Remove extended resources
		for key := range node.Status.Capacity {
			// We check for FeatureLabelNs as -resource-labels can create ERs there
			_, ok := nfdERs[string(key)]
			if ok || strings.HasPrefix(string(key), nfdv1alpha1.FeatureLabelNs) {
				delete(node.Status.Capacity, key)
				delete(node.Status.Allocatable, key)
				updateStatus = true
			}
		}

		if updateStatus {
			By("Deleting NFD extended resources from node " + node.Name)
			if _, err := cs.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}

		if update {
			By("Deleting NFD labels, annotations and taints from node " + node.Name)
			if _, err := cs.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
		return nil
	}

	// Cleanup all nodes
	nodeList, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, n := range nodeList.Items {
		var err error
		for retry := 0; retry < 5; retry++ {
			if err = cleanup(&n); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		Expect(err).NotTo(HaveOccurred())
	}
}

func cleanupCRs(ctx context.Context, cli *nfdclient.Clientset, namespace string) {
	// Drop NodeFeatureRule objects
	nfrs, err := cli.NfdV1alpha1().NodeFeatureRules().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	if len(nfrs.Items) != 0 {
		By("Deleting NodeFeatureRule objects from the cluster")
		for _, nfr := range nfrs.Items {
			err = cli.NfdV1alpha1().NodeFeatureRules().Delete(ctx, nfr.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	nfs, err := cli.NfdV1alpha1().NodeFeatures(namespace).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	if len(nfs.Items) != 0 {
		By("Deleting NodeFeature objects from namespace " + namespace)
		for _, nf := range nfs.Items {
			err = cli.NfdV1alpha1().NodeFeatures(namespace).Delete(ctx, nf.Name, metav1.DeleteOptions{})
			Expect(func() error {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}()).NotTo(HaveOccurred())
		}
	}
}

// Actual test suite
var _ = NFDDescribe(Label("nfd-master"), func() {
	f := framework.NewDefaultFramework("node-feature-discovery")

	nfdTestSuite := func(useNodeFeatureApi bool) {
		createPodSpecOpts := func(opts ...testpod.SpecOption) []testpod.SpecOption {
			if !useNodeFeatureApi {
				return append(opts, testpod.SpecWithContainerExtraArgs("-enable-nodefeature-api=false"))
			}
			return opts
		}

		Context("when deploying a single nfd-master pod", Ordered, func() {
			var (
				crds                   []*apiextensionsv1.CustomResourceDefinition
				extClient              *extclient.Clientset
				nfdClient              *nfdclient.Clientset
				extraMasterPodSpecOpts []testpod.SpecOption
			)

			checkNodeFeatureObject := func(ctx context.Context, name string) {
				_, err := nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Get(ctx, name, metav1.GetOptions{})
				if useNodeFeatureApi {
					By(fmt.Sprintf("Check that NodeFeature object for the node %q was created", name))
					Expect(err).NotTo(HaveOccurred())
				} else {
					By(fmt.Sprintf("Check that NodeFeature object for the node %q hasn't been created", name))
					Expect(err).To(HaveOccurred())
				}
			}

			BeforeAll(func(ctx context.Context) {
				// Create clients for apiextensions and our CRD api
				extClient = extclient.NewForConfigOrDie(f.ClientConfig())
				nfdClient = nfdclient.NewForConfigOrDie(f.ClientConfig())

				By("Creating NFD CRDs")
				var err error
				crds, err = testutils.CreateNfdCRDs(ctx, extClient)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterAll(func(ctx context.Context) {
				for _, crd := range crds {
					err := extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())
				}
			})

			JustBeforeEach(func(ctx context.Context) {
				// Drop the pod security admission label as nfd-worker needs host mounts
				if _, ok := f.Namespace.Labels[admissionapi.EnforceLevelLabel]; ok {
					framework.Logf("Deleting %s label from the test namespace", admissionapi.EnforceLevelLabel)
					delete(f.Namespace.Labels, admissionapi.EnforceLevelLabel)
					_, err := f.ClientSet.CoreV1().Namespaces().Update(ctx, f.Namespace, metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				}

				err := testutils.ConfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)
				Expect(err).NotTo(HaveOccurred())

				// Remove pre-existing stale annotations and labels etc and CRDs
				cleanupCRs(ctx, nfdClient, f.Namespace.Name)
				cleanupNode(ctx, f.ClientSet)

				// Launch nfd-master
				By("Creating nfd master pod and nfd-master service")
				podSpecOpts := createPodSpecOpts(
					append(extraMasterPodSpecOpts,
						testpod.SpecWithContainerImage(dockerImage()),
					)...)

				masterPod := e2epod.NewPodClient(f).CreateSync(ctx, testpod.NFDMaster(podSpecOpts...))

				// Create nfd-master service
				nfdSvc, err := testutils.CreateService(ctx, f.ClientSet, f.Namespace.Name)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for the nfd-master pod to be running")
				Expect(e2epod.WaitTimeoutForPodRunningInNamespace(ctx, f.ClientSet, masterPod.Name, masterPod.Namespace, time.Minute)).NotTo(HaveOccurred())
				By("Waiting for the nfd-master service to be up")
				Expect(e2enetwork.WaitForService(ctx, f.ClientSet, f.Namespace.Name, nfdSvc.Name, true, time.Second, 10*time.Second)).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx context.Context) {
				Expect(testutils.DeconfigureRBAC(ctx, f.ClientSet, f.Namespace.Name)).NotTo(HaveOccurred())

				cleanupNode(ctx, f.ClientSet)
				cleanupCRs(ctx, nfdClient, f.Namespace.Name)
				extraMasterPodSpecOpts = nil
			})

			//
			// Simple test with only the fake source enabled
			//
			Context("and a single worker pod with fake source enabled", func() {
				It("it should decorate the node with the fake feature labels", Label("nfd-worker"), func(ctx context.Context) {
					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					// Launch nfd-worker
					By("Creating a nfd worker pod")
					podSpecOpts := createPodSpecOpts(
						testpod.SpecWithRestartPolicy(corev1.RestartPolicyNever),
						testpod.SpecWithContainerImage(dockerImage()),
						testpod.SpecWithContainerExtraArgs("-oneshot", "-label-sources=fake"),
					)
					workerPod := testpod.NFDWorker(podSpecOpts...)
					workerPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, workerPod, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for the nfd-worker pod to succeed")
					Expect(e2epod.WaitForPodSuccessInNamespace(ctx, f.ClientSet, workerPod.Name, f.Namespace.Name)).NotTo(HaveOccurred())
					workerPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(ctx, workerPod.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())

					By(fmt.Sprintf("Making sure '%s' was decorated with the fake feature labels", workerPod.Spec.NodeName))
					expectedLabels := map[string]k8sLabels{
						workerPod.Spec.NodeName: {
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature1": "true",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature2": "true",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature3": "true",
						},
						"*": {},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					checkNodeFeatureObject(ctx, workerPod.Spec.NodeName)

					By("Deleting the node-feature-discovery worker pod")
					err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, workerPod.Name, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					if useNodeFeatureApi {
						By("Verify that labels from nfd-worker are garbage-collected")
						delete(expectedLabels, workerPod.Spec.NodeName)
						eventuallyNonControlPlaneNodes(ctx, f.ClientSet).WithTimeout(1 * time.Minute).Should(MatchLabels(expectedLabels, nodes))
					}
				})
			})

			//
			// More comprehensive test when --e2e-node-config is enabled
			//
			Context("and nfd-workers as a daemonset with default sources enabled", func() {
				It("the node labels and annotations listed in the e2e config should be present", Label("nfd-worker"), func(ctx context.Context) {
					cfg, err := testutils.GetConfig()
					Expect(err).ToNot(HaveOccurred())

					if cfg == nil {
						Skip("no e2e-config was specified")
					}
					if cfg.DefaultFeatures == nil {
						Skip("no 'defaultFeatures' specified in e2e-config")
					}
					fConf := cfg.DefaultFeatures

					By("Creating nfd-worker daemonset")
					podSpecOpts := createPodSpecOpts(
						testpod.SpecWithContainerImage(dockerImage()),
					)
					workerDS := testds.NFDWorker(podSpecOpts...)
					workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(ctx, workerDS, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for worker daemonset pods to be ready")
					Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

					By("Getting node objects")
					nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(nodeList.Items)).ToNot(BeZero())

					for _, node := range nodeList.Items {
						nodeConf := testutils.FindNodeConfig(cfg, node.Name)
						if nodeConf == nil {
							framework.Logf("node %q has no matching rule in e2e-config, skipping...", node.Name)
							continue
						}

						// Check labels
						framework.Logf("verifying labels of node %q...", node.Name)
						for k, v := range nodeConf.ExpectedLabelValues {
							Expect(node.Labels).To(HaveKeyWithValue(k, v))
						}
						for k := range nodeConf.ExpectedLabelKeys {
							Expect(node.Labels).To(HaveKey(k))
						}
						for k := range node.Labels {
							if strings.HasPrefix(k, nfdv1alpha1.FeatureLabelNs) {
								if _, ok := nodeConf.ExpectedLabelValues[k]; ok {
									continue
								}
								if _, ok := nodeConf.ExpectedLabelKeys[k]; ok {
									continue
								}
								// Ignore if the label key was not whitelisted
								Expect(fConf.LabelWhitelist).NotTo(HaveKey(k))
							}
						}

						// Check annotations
						framework.Logf("verifying annotations of node %q...", node.Name)
						for k, v := range nodeConf.ExpectedAnnotationValues {
							Expect(node.Annotations).To(HaveKeyWithValue(k, v))
						}
						for k := range nodeConf.ExpectedAnnotationKeys {
							Expect(node.Annotations).To(HaveKey(k))
						}
						for k := range node.Annotations {
							if strings.HasPrefix(k, nfdv1alpha1.AnnotationNs) {
								if _, ok := nodeConf.ExpectedAnnotationValues[k]; ok {
									continue
								}
								if _, ok := nodeConf.ExpectedAnnotationKeys[k]; ok {
									continue
								}
								// Ignore if the annotation was not whitelisted
								Expect(fConf.AnnotationWhitelist).NotTo(HaveKey(k))
							}
						}

						// Check existence of NodeFeature object
						checkNodeFeatureObject(ctx, node.Name)

					}

					By("Deleting nfd-worker daemonset")
					err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(ctx, workerDS.Name, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			//
			// Test custom nodename source configured in 2 additional ConfigMaps
			//
			Context("and nfd-workers as a daemonset with 2 additional configmaps for the custom source configured", func() {
				It("the nodename matching features listed in the configmaps should be present", Label("nfd-worker"), func(ctx context.Context) {
					By("Getting a worker node")

					// We need a valid nodename for the configmap
					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No worker node found")

					// create a wildcard name as well for this node
					targetNodeNameWildcard := fmt.Sprintf("%s.*%s", targetNodeName[:2], targetNodeName[4:])

					By("Creating the configmaps")
					targetLabelName := "nodename-test"
					targetLabelValue := "true"

					targetLabelNameWildcard := "nodename-test-wildcard"
					targetLabelValueWildcard := "customValue"

					// create 2 configmaps
					data1 := fmt.Sprintf(`
- name: nodename-test-rule
  labels:
    "%s": "%s"
  matchFeatures:
    - feature: system.name
      matchExpressions:
         nodename: {op: In, value: ["%s"]}`, targetLabelName, targetLabelValue, targetNodeName)

					cm1 := testutils.NewConfigMap("custom-config-extra-1", "custom.conf", data1)
					cm1, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm1, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					data2 := fmt.Sprintf(`
- name: nodename-test-regexp-rule
  labels:
    "%s": "%s"
  matchFeatures:
    - feature: system.name
      matchExpressions:
         nodename: {op: InRegexp, value: ["^%s$"]}

- name: nodename-test-negative-rule
  labels:
    "nodename-test-negative": "true"
  matchFeatures:
    - feature: system.name
      matchExpressions:
         nodename: {op: In, value: ["thisNameShouldNeverMatch"]}`, targetLabelNameWildcard, targetLabelValueWildcard, targetNodeNameWildcard)

					cm2 := testutils.NewConfigMap("custom-config-extra-2", "custom.conf", data2)
					cm2, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm2, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Creating nfd-worker daemonset with configmap mounted")
					podSpecOpts := createPodSpecOpts(
						testpod.SpecWithContainerImage(dockerImage()),
						testpod.SpecWithContainerExtraArgs("-label-sources=custom"),
						testpod.SpecWithConfigMap(cm1.Name, filepath.Join(custom.Directory, "cm1")),
						testpod.SpecWithConfigMap(cm2.Name, filepath.Join(custom.Directory, "cm2")),
					)
					workerDS := testds.NFDWorker(podSpecOpts...)

					workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(ctx, workerDS, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for worker daemonset pods to be ready")
					Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

					By("Verifying node labels")
					expectedLabels := map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/" + targetLabelName:         targetLabelValue,
							nfdv1alpha1.FeatureLabelNs + "/" + targetLabelNameWildcard: targetLabelValueWildcard,
						},
						"*": {},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Deleting nfd-worker daemonset")
					err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(ctx, workerDS.Name, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					if useNodeFeatureApi {
						By("Verify that labels from nfd-worker are garbage-collected")
						delete(expectedLabels, targetNodeName)
						eventuallyNonControlPlaneNodes(ctx, f.ClientSet).WithTimeout(1 * time.Minute).Should(MatchLabels(expectedLabels, nodes))
					}
				})
			})

			//
			// Test NodeFeature
			//
			Context("and NodeFeature objects deployed", func() {
				BeforeEach(func(ctx context.Context) {
					extraMasterPodSpecOpts = []testpod.SpecOption{
						testpod.SpecWithContainerExtraArgs(
							"-deny-label-ns=*.denied.ns,random.unwanted.ns,*.vendor.io",
							"-extra-label-ns=custom.vendor.io",
						),
					}
				})
				It("labels from the NodeFeature objects should be created", Label("nfd-worker"), func(ctx context.Context) {
					if !useNodeFeatureApi {
						Skip("NodeFeature API not enabled")
					}

					// We pick one node targeted for our NodeFeature objects
					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

					By("Creating NodeFeature object")
					nodeFeatures, err := testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-1.yaml", f.Namespace.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object #1")
					expectedLabels := map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-1": "obj-1",
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-2": "obj-1",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature3":      "overridden",
						},
						"*": {},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
					By("Deleting NodeFeature object")
					err = nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Delete(ctx, nodeFeatures[0], metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object were removed")
					expectedLabels[targetNodeName] = k8sLabels{}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Creating nfd-worker daemonset")
					podSpecOpts := createPodSpecOpts(
						testpod.SpecWithContainerImage(dockerImage()),
						testpod.SpecWithContainerExtraArgs("-label-sources=fake"),
					)
					workerDS := testds.NFDWorker(podSpecOpts...)
					workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(ctx, workerDS, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for worker daemonset pods to be ready")
					Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

					By("Verifying node labels from nfd-worker")
					expectedLabels = map[string]k8sLabels{
						"*": {
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature1": "true",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature2": "true",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature3": "true",
						},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Re-creating NodeFeature object")
					_, err = testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-1.yaml", f.Namespace.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object #1 are created")
					expectedLabels[targetNodeName] = k8sLabels{
						nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-1": "obj-1",
						nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-2": "obj-1",
						nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature1":      "true",
						nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature2":      "true",
						nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature3":      "overridden",
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Creating extra namespace")
					extraNs, err := f.CreateNamespace(ctx, "node-feature-discvery-extra-ns", nil)
					Expect(err).NotTo(HaveOccurred())

					By("Create NodeFeature object in the extra namespace")
					nodeFeatures, err = testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-2.yaml", extraNs.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object #2 are created")
					expectedLabels[targetNodeName][nfdv1alpha1.FeatureLabelNs+"/e2e-nodefeature-test-1"] = "overridden-from-obj-2"
					expectedLabels[targetNodeName][nfdv1alpha1.FeatureLabelNs+"/e2e-nodefeature-test-3"] = "obj-2"
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Deleting NodeFeature object from the extra namespace")
					err = nfdClient.NfdV1alpha1().NodeFeatures(extraNs.Name).Delete(ctx, nodeFeatures[0], metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object were removed")
					expectedLabels[targetNodeName][nfdv1alpha1.FeatureLabelNs+"/e2e-nodefeature-test-1"] = "obj-1"
					delete(expectedLabels[targetNodeName], nfdv1alpha1.FeatureLabelNs+"/e2e-nodefeature-test-3")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
				})

				It("denied labels should not be created by the NodeFeature object", func(ctx context.Context) {
					if !useNodeFeatureApi {
						Skip("NodeFeature API not enabled")
					}

					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

					// Apply Node Feature object
					By("Create NodeFeature object")
					nodeFeatures, err := testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-3.yaml", f.Namespace.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					// Verify that denied label was not added
					By("Verifying that denied labels were not added")
					expectedLabels := map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-4": "obj-4",
							"custom.vendor.io/e2e-nodefeature-test-3":              "vendor-ns",
						},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Deleting NodeFeature object")
					err = nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Delete(ctx, nodeFeatures[0], metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					expectedLabels[targetNodeName] = k8sLabels{}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
				})
			})

			// Test NodeFeatureRule
			//
			Context("and nfd-worker and NodeFeatureRules objects deployed", Label("nodefeaturerule"), func() {
				testTolerations := []corev1.Toleration{
					{
						Key:    "feature.node.kubernetes.io/fake-special-node",
						Value:  "exists",
						Effect: "NoExecute",
					},
					{
						Key:    "feature.node.kubernetes.io/fake-dedicated-node",
						Value:  "true",
						Effect: "NoExecute",
					},
					{
						Key:    "feature.node.kubernetes.io/performance-optimized-node",
						Value:  "true",
						Effect: "NoExecute",
					},
					{
						Key:    "feature.node.kubernetes.io/foo",
						Value:  "true",
						Effect: "NoExecute",
					},
				}
				BeforeEach(func(ctx context.Context) {
					extraMasterPodSpecOpts = []testpod.SpecOption{
						testpod.SpecWithContainerExtraArgs("-enable-taints"),
						testpod.SpecWithTolerations(testTolerations),
					}
				})
				It("custom features from the NodeFeatureRule rules should be created", Label("nfd-worker"), func(ctx context.Context) {
					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

					By("Creating nfd-worker config")
					cm := testutils.NewConfigMap("nfd-worker-conf", "nfd-worker.conf", `
core:
  sleepInterval: "1s"
  featureSources: ["fake"]
  labelSources: []
`)
					cm, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					By("Creating nfd-worker daemonset")
					podSpecOpts := createPodSpecOpts(
						testpod.SpecWithContainerImage(dockerImage()),
						testpod.SpecWithConfigMap(cm.Name, "/etc/kubernetes/node-feature-discovery"),
						testpod.SpecWithTolerations(testTolerations),
					)
					workerDS := testds.NFDWorker(podSpecOpts...)
					workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(ctx, workerDS, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Waiting for worker daemonset pods to be ready")
					Expect(testpod.WaitForReady(ctx, f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 2)).NotTo(HaveOccurred())

					expectedLabels := map[string]k8sLabels{
						"*": {
							nfdv1alpha1.FeatureLabelNs + "/e2e-flag-test-1":      "true",
							nfdv1alpha1.FeatureLabelNs + "/e2e-flag-test-2":      "true",
							nfdv1alpha1.FeatureLabelNs + "/e2e-attribute-test-1": "true",
							nfdv1alpha1.FeatureLabelNs + "/e2e-attribute-test-2": "true",
							nfdv1alpha1.FeatureLabelNs + "/e2e-instance-test-1":  "true",
							nfdv1alpha1.FeatureLabelNs + "/e2e-instance-test-2":  "true",
						},
					}

					By("Creating NodeFeatureRules #1")
					Expect(testutils.CreateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-1.yaml")).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeatureRules #1")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Creating NodeFeatureRules #2")
					Expect(testutils.CreateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-2.yaml")).NotTo(HaveOccurred())

					// Add features from NodeFeatureRule #2
					maps.Copy(expectedLabels["*"], k8sLabels{
						nfdv1alpha1.FeatureLabelNs + "/e2e-matchany-test-1":            "true",
						nfdv1alpha1.FeatureLabelNs + "/e2e-template-test-1-instance_1": "found",
						nfdv1alpha1.FeatureLabelNs + "/e2e-template-test-1-instance_2": "found",
						nfdv1alpha1.FeatureLabelNs + "/e2e-template-test-2-attr_2":     "false",
						nfdv1alpha1.FeatureLabelNs + "/e2e-template-test-2-attr_3":     "10",
						nfdv1alpha1.FeatureLabelNs + "/dynamic-label":                  "true",
					})
					expectedAnnotations := map[string]k8sAnnotations{
						"*": {
							"nfd.node.kubernetes.io/feature-labels": "dynamic-label,e2e-attribute-test-1,e2e-attribute-test-2,e2e-flag-test-1,e2e-flag-test-2,e2e-instance-test-1,e2e-instance-test-2,e2e-matchany-test-1,e2e-template-test-1-instance_1,e2e-template-test-1-instance_2,e2e-template-test-2-attr_2,e2e-template-test-2-attr_3"},
					}

					By("Verifying node labels from NodeFeatureRules #1 and #2")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					// Add features from NodeFeatureRule #3
					By("Creating NodeFeatureRules #3")
					Expect(testutils.CreateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-3.yaml")).NotTo(HaveOccurred())

					By("Verifying node taints and annotation from NodeFeatureRules #3")
					expectedTaints := map[string][]corev1.Taint{
						"*": {
							{
								Key:    "feature.node.kubernetes.io/fake-special-node",
								Value:  "exists",
								Effect: "PreferNoSchedule",
							},
							{
								Key:    "feature.node.kubernetes.io/fake-dedicated-node",
								Value:  "true",
								Effect: "NoExecute",
							},
							{
								Key:    "feature.node.kubernetes.io/performance-optimized-node",
								Value:  "true",
								Effect: "NoExecute",
							},
						},
					}
					expectedAnnotations["*"]["nfd.node.kubernetes.io/taints"] = "feature.node.kubernetes.io/fake-special-node=exists:PreferNoSchedule,feature.node.kubernetes.io/fake-dedicated-node=true:NoExecute,feature.node.kubernetes.io/performance-optimized-node=true:NoExecute"

					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchTaints(expectedTaints, nodes))
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Re-applying NodeFeatureRules #3 with updated taints")
					Expect(testutils.UpdateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-3-updated.yaml")).NotTo(HaveOccurred())
					expectedTaints["*"] = []corev1.Taint{
						{
							Key:    "feature.node.kubernetes.io/fake-special-node",
							Value:  "exists",
							Effect: "PreferNoSchedule",
						},
						{
							Key:    "feature.node.kubernetes.io/foo",
							Value:  "true",
							Effect: "NoExecute",
						},
					}
					expectedAnnotations["*"]["nfd.node.kubernetes.io/taints"] = "feature.node.kubernetes.io/fake-special-node=exists:PreferNoSchedule,feature.node.kubernetes.io/foo=true:NoExecute"

					By("Verifying updated node taints and annotation from NodeFeatureRules #3")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchTaints(expectedTaints, nodes))
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Deleting NodeFeatureRule #3")
					err = nfdClient.NfdV1alpha1().NodeFeatureRules().Delete(ctx, "e2e-test-3", metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())
					By("Verifying taints from NodeFeatureRules #3 were removed")
					expectedTaints["*"] = []corev1.Taint{}
					delete(expectedAnnotations["*"], "nfd.node.kubernetes.io/taints")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchTaints(expectedTaints, nodes))
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					expectedAnnotations["*"]["nfd.node.kubernetes.io/extended-resources"] = "nons,vendor.feature.node.kubernetes.io/static,vendor.io/dynamic"

					expectedCapacity := map[string]corev1.ResourceList{
						"*": {
							"feature.node.kubernetes.io/nons":          resourcev1.MustParse("123"),
							"vendor.io/dynamic":                        resourcev1.MustParse("10"),
							"vendor.feature.node.kubernetes.io/static": resourcev1.MustParse("123"),
						},
					}

					By("Creating NodeFeatureRules #4")
					Expect(testutils.CreateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-4.yaml")).NotTo(HaveOccurred())

					By("Verifying node annotations from NodeFeatureRules #4")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Verifying node status capacity from NodeFeatureRules #4")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).WithTimeout(1 * time.Minute).Should(MatchCapacity(expectedCapacity, nodes))

					By("Deleting NodeFeatureRules #4")
					err = nfdClient.NfdV1alpha1().NodeFeatureRules().Delete(ctx, "e2e-extened-resource-test", metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node status capacity from NodeFeatureRules #4 was removed")
					expectedCapacity = map[string]corev1.ResourceList{"*": {}}
					delete(expectedAnnotations["*"], "nfd.node.kubernetes.io/extended-resources")
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).WithTimeout(1 * time.Minute).Should(MatchCapacity(expectedCapacity, nodes))
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Creating NodeFeatureRules #5")
					Expect(testutils.CreateNodeFeatureRulesFromFile(ctx, nfdClient, "nodefeaturerule-5.yaml")).NotTo(HaveOccurred())

					By("Verifying node annotations from NodeFeatureRules #5")
					expectedAnnotations["*"][nfdv1alpha1.FeatureAnnotationNs+"/defaul-ns-annotation"] = "foo"
					expectedAnnotations["*"][nfdv1alpha1.FeatureAnnotationNs+"/defaul-ns-annotation-2"] = "bar"
					expectedAnnotations["*"]["custom.vendor.io/feature"] = "baz"
					expectedAnnotations["*"][nfdv1alpha1.FeatureAnnotationsTrackingAnnotation] = "custom.vendor.io/feature,defaul-ns-annotation,defaul-ns-annotation-2"

					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Deleting NodeFeatureRule object")
					err = nfdClient.NfdV1alpha1().NodeFeatureRules().Delete(ctx, "e2e-feature-annotations-test", metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node annotations from NodeFeatureRules #5 are deleted")
					delete(expectedAnnotations["*"], nfdv1alpha1.FeatureAnnotationNs+"/defaul-ns-annotation")
					delete(expectedAnnotations["*"], nfdv1alpha1.FeatureAnnotationNs+"/defaul-ns-annotation-2")
					delete(expectedAnnotations["*"], "custom.vendor.io/feature")
					delete(expectedAnnotations["*"], nfdv1alpha1.FeatureAnnotationsTrackingAnnotation)
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchAnnotations(expectedAnnotations, nodes))

					By("Deleting nfd-worker daemonset")
					err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(ctx, workerDS.Name, metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					if useNodeFeatureApi {
						By("Verify that labels from nfd-worker are garbage-collected")
						expectedLabels = map[string]k8sLabels{
							"*": {},
						}
						eventuallyNonControlPlaneNodes(ctx, f.ClientSet).WithTimeout(1 * time.Minute).Should(MatchLabels(expectedLabels, nodes))
					}
				})
			})

			Context("and check whether master config passed successfully or not", func() {
				BeforeEach(func(ctx context.Context) {
					extraMasterPodSpecOpts = []testpod.SpecOption{
						testpod.SpecWithConfigMap("nfd-master-conf", "/etc/kubernetes/node-feature-discovery"),
					}
					cm := testutils.NewConfigMap("nfd-master-conf", "nfd-master.conf", `
denyLabelNs: ["*.denied.ns","random.unwanted.ns"]
`)
					_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
				It("master configuration should take place", func(ctx context.Context) {
					// deploy node feature object
					if !useNodeFeatureApi {
						Skip("NodeFeature API not enabled")
					}

					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

					// Apply Node Feature object
					By("Create NodeFeature object")
					nodeFeatures, err := testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-3.yaml", f.Namespace.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					// Verify that denied label was not added
					By("Verifying that denied labels were not added")
					expectedLabels := map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-4": "obj-4",
							"custom.vendor.io/e2e-nodefeature-test-3":              "vendor-ns",
						},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
					By("Deleting NodeFeature object")
					err = nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Delete(ctx, nodeFeatures[0], metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())

					// TODO: Find a better way to handle the timeout that happens to reflect the configmap changes
					Skip("Testing the master dynamic configuration")
					// Verify that config changes were applied
					By("Updating the master config")
					Expect(testutils.UpdateConfigMap(ctx, f.ClientSet, "nfd-master-conf", f.Namespace.Name, "nfd-master.conf", `
denyLabelNs: []
`))
					By("Verifying that denied labels were removed")
					expectedLabels = map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-4": "obj-4",
							"custom.vendor.io/e2e-nodefeature-test-3":              "vendor-ns",
							"random.denied.ns/e2e-nodefeature-test-1":              "denied-ns",
							"random.unwanted.ns/e2e-nodefeature-test-2":            "unwanted-ns",
						},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))
				})
			})

			Context("and test whether resyncPeriod is passed successfully or not", func() {
				BeforeEach(func(ctx context.Context) {
					extraMasterPodSpecOpts = []testpod.SpecOption{
						testpod.SpecWithConfigMap("nfd-master-conf", "/etc/kubernetes/node-feature-discovery"),
					}
					cm := testutils.NewConfigMap("nfd-master-conf", "nfd-master.conf", `
resyncPeriod: "1s"
`)
					_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, cm, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
				It("labels should be restored to the original ones", func(ctx context.Context) {
					// deploy node feature object
					if !useNodeFeatureApi {
						Skip("NodeFeature API not enabled")
					}

					nodes, err := getNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

					// Apply Node Feature object
					By("Creating NodeFeature object")
					nodeFeatures, err := testutils.CreateOrUpdateNodeFeaturesFromFile(ctx, nfdClient, "nodefeature-1.yaml", f.Namespace.Name, targetNodeName)
					Expect(err).NotTo(HaveOccurred())

					By("Verifying node labels from NodeFeature object #1")
					expectedLabels := map[string]k8sLabels{
						targetNodeName: {
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-1": "obj-1",
							nfdv1alpha1.FeatureLabelNs + "/e2e-nodefeature-test-2": "obj-1",
							nfdv1alpha1.FeatureLabelNs + "/fake-fakefeature3":      "overridden",
						},
						"*": {},
					}
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					patches, err := json.Marshal(
						[]apihelper.JsonPatch{
							apihelper.NewJsonPatch(
								"replace",
								"/metadata/labels",
								nfdv1alpha1.FeatureLabelNs+"/e2e-nodefeature-test-1",
								"randomValue",
							),
						},
					)
					Expect(err).NotTo(HaveOccurred())

					_, err = f.ClientSet.CoreV1().Nodes().Patch(ctx, targetNodeName, types.JSONPatchType, patches, metav1.PatchOptions{})
					Expect(err).NotTo(HaveOccurred())

					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(expectedLabels, nodes))

					By("Deleting NodeFeature object")
					err = nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Delete(ctx, nodeFeatures[0], metav1.DeleteOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	}

	// Run the actual tests
	Context("when running NFD with gRPC API enabled", func() {
		nfdTestSuite(false)
	})

	Context("when running NFD with NodeFeature CRD API enabled", func() {
		nfdTestSuite(true)
	})

})

// getNonControlPlaneNodes gets the nodes that are not tainted for exclusive control-plane usage
func getNonControlPlaneNodes(ctx context.Context, cli clientset.Interface) ([]corev1.Node, error) {
	nodeList, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("no nodes found in the cluster")
	}

	controlPlaneTaint := corev1.Taint{
		Effect: corev1.TaintEffectNoSchedule,
		Key:    "node-role.kubernetes.io/control-plane",
	}
	out := []corev1.Node{}
	for _, node := range nodeList.Items {
		if !taintutils.TaintExists(node.Spec.Taints, &controlPlaneTaint) {
			out = append(out, node)
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no non-control-plane nodes found in the cluster")
	}
	return out, nil
}

func getNode(nodes []corev1.Node, nodeName string) corev1.Node {
	for _, node := range nodes {
		if node.Name == nodeName {
			return node
		}
	}
	return corev1.Node{}
}
