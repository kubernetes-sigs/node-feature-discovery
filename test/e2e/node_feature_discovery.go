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
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	master "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	"sigs.k8s.io/node-feature-discovery/source/custom"
	testutils "sigs.k8s.io/node-feature-discovery/test/e2e/utils"
)

var (
	dockerRepo = flag.String("nfd.repo", "gcr.io/k8s-staging-nfd/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag  = flag.String("nfd.tag", "master", "Docker tag to use")
)

// cleanupNode deletes all NFD-related metadata from the Node object, i.e.
// labels and annotations
func cleanupNode(cs clientset.Interface) {
	nodeList, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, n := range nodeList.Items {
		var err error
		var node *v1.Node
		for retry := 0; retry < 5; retry++ {
			node, err = cs.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			update := false
			// Remove labels
			for key := range node.Labels {
				if strings.HasPrefix(key, master.LabelNs) {
					delete(node.Labels, key)
					update = true
				}
			}

			// Remove annotations
			for key := range node.Annotations {
				if strings.HasPrefix(key, master.AnnotationNsBase) {
					delete(node.Annotations, key)
					update = true
				}
			}

			if !update {
				break
			}

			By("Deleting NFD labels and annotations from node " + node.Name)
			_, err = cs.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			if err != nil {
				time.Sleep(100 * time.Millisecond)
			} else {
				break
			}

		}
		Expect(err).NotTo(HaveOccurred())
	}
}

// Actual test suite
var _ = Describe("[NFD] Node Feature Discovery", func() {
	f := framework.NewDefaultFramework("node-feature-discovery")

	Context("when deploying a single nfd-master pod", func() {
		var masterPod *v1.Pod

		BeforeEach(func() {
			err := testutils.ConfigureRBAC(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			// Launch nfd-master
			By("Creating nfd master pod and nfd-master service")
			image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
			masterPod = f.PodClient().CreateSync(testutils.NFDMasterPod(image, false))
			// Create nfd-master service

			// Create nfd-master service
			nfdSvc, err := testutils.CreateService(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the nfd-master pod to be running")
			Expect(e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, masterPod.Name, masterPod.Namespace, time.Minute)).NotTo(HaveOccurred())

			By("Waiting for the nfd-master service to be up")
			Expect(e2enetwork.WaitForService(f.ClientSet, f.Namespace.Name, nfdSvc.ObjectMeta.Name, true, time.Second, 10*time.Second)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := testutils.DeconfigureRBAC(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

		})

		//
		// Simple test with only the fake source enabled
		//
		Context("and a single worker pod with fake source enabled", func() {
			It("it should decorate the node with the fake feature labels", func() {

				fakeFeatureLabels := map[string]string{
					master.LabelNs + "/fake-fakefeature1": "true",
					master.LabelNs + "/fake-fakefeature2": "true",
					master.LabelNs + "/fake-fakefeature3": "true",
				}

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				// Launch nfd-worker
				By("Creating a nfd worker pod")
				image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
				workerPod := testutils.NFDWorkerPod(image, []string{"--oneshot", "--sources=fake"})
				workerPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), workerPod, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for the nfd-worker pod to succeed")
				Expect(e2epod.WaitForPodSuccessInNamespace(f.ClientSet, workerPod.ObjectMeta.Name, f.Namespace.Name)).NotTo(HaveOccurred())
				workerPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), workerPod.ObjectMeta.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Making sure '%s' was decorated with the fake feature labels", workerPod.Spec.NodeName))
				node, err := f.ClientSet.CoreV1().Nodes().Get(context.TODO(), workerPod.Spec.NodeName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				for k, v := range fakeFeatureLabels {
					Expect(node.Labels[k]).To(Equal(v))
				}

				// Check that there are no unexpected NFD labels
				for k := range node.Labels {
					if strings.HasPrefix(k, master.LabelNs) {
						Expect(fakeFeatureLabels).Should(HaveKey(k))
					}
				}

				By("Deleting the node-feature-discovery worker pod")
				err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), workerPod.ObjectMeta.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				cleanupNode(f.ClientSet)
			})
		})

		//
		// More comprehensive test when --e2e-node-config is enabled
		//
		Context("and nfd-workers as a daemonset with default sources enabled", func() {
			It("the node labels and annotations listed in the e2e config should be present", func() {
				err := testutils.ReadConfig()
				Expect(err).ToNot(HaveOccurred())

				if testutils.E2EConfigFile == nil {
					Skip("no e2e-config was specified")
				}
				if testutils.E2EConfigFile.DefaultFeatures == nil {
					Skip("no 'defaultFeatures' specified in e2e-config")
				}
				fConf := testutils.E2EConfigFile.DefaultFeatures

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				By("Creating nfd-worker daemonset")
				workerDS := testutils.NFDWorkerDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})
				workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), workerDS, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for daemonset pods to be ready")
				Expect(e2epod.WaitForPodsReady(f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 5)).NotTo(HaveOccurred())

				By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				for _, node := range nodeList.Items {
					nodeConf := testutils.FindNodeConfig(node.Name)
					if nodeConf == nil {
						e2elog.Logf("node %q has no matching rule in e2e-config, skipping...", node.Name)
						continue
					}

					// Check labels
					e2elog.Logf("verifying labels of node %q...", node.Name)
					for k, v := range nodeConf.ExpectedLabelValues {
						Expect(node.Labels).To(HaveKeyWithValue(k, v))
					}
					for k := range nodeConf.ExpectedLabelKeys {
						Expect(node.Labels).To(HaveKey(k))
					}
					for k := range node.Labels {
						if strings.HasPrefix(k, master.LabelNs) {
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
					e2elog.Logf("verifying annotations of node %q...", node.Name)
					for k, v := range nodeConf.ExpectedAnnotationValues {
						Expect(node.Annotations).To(HaveKeyWithValue(k, v))
					}
					for k := range nodeConf.ExpectedAnnotationKeys {
						Expect(node.Annotations).To(HaveKey(k))
					}
					for k := range node.Annotations {
						if strings.HasPrefix(k, master.AnnotationNsBase) {
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

					// Node running nfd-master should have master version annotation
					if node.Name == masterPod.Spec.NodeName {
						Expect(node.Annotations).To(HaveKey(master.AnnotationNsBase + "master.version"))
					}
				}

				By("Deleting nfd-worker daemonset")
				err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(context.TODO(), workerDS.ObjectMeta.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				cleanupNode(f.ClientSet)
			})
		})

		//
		// Test custom nodename source configured in 2 additional ConfigMaps
		//
		Context("and nfd-workers as a daemonset with 2 additional configmaps for the custom source configured", func() {
			It("the nodename matching features listed in the configmaps should be present", func() {
				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				By("Getting a worker node")

				// We need a valid nodename for the configmap
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				targetNodeName := ""
				for _, node := range nodeList.Items {
					if _, ok := node.Labels["node-role.kubernetes.io/master"]; !ok {
						targetNodeName = node.Name
						break
					}
				}
				Expect(targetNodeName).ToNot(BeEmpty(), "No worker node found")

				// create a wildcard name as well for this node
				targetNodeNameWildcard := fmt.Sprintf("%s.*%s", targetNodeName[:2], targetNodeName[4:])

				By("Creating the configmaps")
				targetLabelName := "nodename-test"
				targetLabelValue := "true"

				targetLabelNameWildcard := "nodename-test-wildcard"
				targetLabelValueWildcard := "customValue"

				targetLabelNameNegative := "nodename-test-negative"

				// create 2 configmaps
				data1 := make(map[string]string)
				data1["custom1.conf"] = `
- name: ` + targetLabelName + `
  matchOn:
  # default value is true
  - nodename:
    - ` + targetNodeName

				cm1 := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom-config-extra-" + string(uuid.NewUUID()),
					},
					Data: data1,
				}
				cm1, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(context.TODO(), cm1, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				data2 := make(map[string]string)
				data2["custom1.conf"] = `
- name: ` + targetLabelNameWildcard + `
  value: ` + targetLabelValueWildcard + `
  matchOn:
  - nodename:
    - ` + targetNodeNameWildcard + `
- name: ` + targetLabelNameNegative + `
  matchOn:
  - nodename:
    - "thisNameShouldNeverMatch"`

				cm2 := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom-config-extra-" + string(uuid.NewUUID()),
					},
					Data: data2,
				}
				cm2, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(context.TODO(), cm2, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Creating nfd-worker daemonset with configmap mounted")
				workerDS := testutils.NFDWorkerDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})

				// add configmap mount config
				volumeName1 := "custom-configs-extra1"
				volumeName2 := "custom-configs-extra2"
				workerDS.Spec.Template.Spec.Volumes = append(workerDS.Spec.Template.Spec.Volumes,
					v1.Volume{
						Name: volumeName1,
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: cm1.Name,
								},
							},
						},
					},
					v1.Volume{
						Name: volumeName2,
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: cm2.Name,
								},
							},
						},
					},
				)
				workerDS.Spec.Template.Spec.Containers[0].VolumeMounts = append(workerDS.Spec.Template.Spec.Containers[0].VolumeMounts,
					v1.VolumeMount{
						Name:      volumeName1,
						ReadOnly:  true,
						MountPath: filepath.Join(custom.Directory, "cm1"),
					},
					v1.VolumeMount{
						Name:      volumeName2,
						ReadOnly:  true,
						MountPath: filepath.Join(custom.Directory, "cm2"),
					},
				)

				workerDS, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), workerDS, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for daemonset pods to be ready")
				Expect(e2epod.WaitForPodsReady(f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 5)).NotTo(HaveOccurred())

				By("Getting target node and checking labels")
				targetNode, err := f.ClientSet.CoreV1().Nodes().Get(context.TODO(), targetNodeName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				labelFound := false
				labelWildcardFound := false
				labelNegativeFound := false
				for k := range targetNode.Labels {
					if strings.Contains(k, targetLabelName) {
						if targetNode.Labels[k] == targetLabelValue {
							labelFound = true
						}
					}
					if strings.Contains(k, targetLabelNameWildcard) {
						if targetNode.Labels[k] == targetLabelValueWildcard {
							labelWildcardFound = true
						}
					}
					if strings.Contains(k, targetLabelNameNegative) {
						labelNegativeFound = true
					}
				}

				Expect(labelFound).To(BeTrue(), "label not found!")
				Expect(labelWildcardFound).To(BeTrue(), "label for wildcard nodename not found!")
				Expect(labelNegativeFound).To(BeFalse(), "label for not existing nodename found!")

				By("Deleting nfd-worker daemonset")
				err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(context.TODO(), workerDS.ObjectMeta.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				cleanupNode(f.ClientSet)
			})
		})

	})

})
