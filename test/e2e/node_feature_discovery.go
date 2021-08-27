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
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	master "sigs.k8s.io/node-feature-discovery/pkg/nfd-master"
	"sigs.k8s.io/node-feature-discovery/source/custom"
	"sigs.k8s.io/yaml"
)

var (
	dockerRepo    = flag.String("nfd.repo", "gcr.io/k8s-staging-nfd/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag     = flag.String("nfd.tag", "master", "Docker tag to use")
	e2eConfigFile = flag.String("nfd.e2e-config", "", "Configuration parameters for end-to-end tests")
	openShift     = flag.Bool("nfd.openshift", false, "Enable OpenShift specific bits")

	conf *e2eConfig
)

type e2eConfig struct {
	DefaultFeatures *struct {
		LabelWhitelist      lookupMap
		AnnotationWhitelist lookupMap
		Nodes               map[string]nodeConfig
	}
}

type nodeConfig struct {
	nameRe                   *regexp.Regexp
	ExpectedLabelValues      map[string]string
	ExpectedLabelKeys        lookupMap
	ExpectedAnnotationValues map[string]string
	ExpectedAnnotationKeys   lookupMap
}

type lookupMap map[string]struct{}

func (l *lookupMap) UnmarshalJSON(data []byte) error {
	*l = lookupMap{}
	slice := []string{}

	err := yaml.Unmarshal(data, &slice)
	if err != nil {
		return err
	}

	for _, k := range slice {
		(*l)[k] = struct{}{}
	}
	return nil
}

func readConfig() {
	// Read and parse only once
	if conf != nil || *e2eConfigFile == "" {
		return
	}

	By("Reading end-to-end test configuration file")
	data, err := ioutil.ReadFile(*e2eConfigFile)
	Expect(err).NotTo(HaveOccurred())

	By("Parsing end-to-end test configuration data")
	err = yaml.Unmarshal(data, &conf)
	Expect(err).NotTo(HaveOccurred())

	// Pre-compile node name matching regexps
	for name, nodeConf := range conf.DefaultFeatures.Nodes {
		nodeConf.nameRe, err = regexp.Compile(name)
		Expect(err).NotTo(HaveOccurred())
		conf.DefaultFeatures.Nodes[name] = nodeConf
	}
}

// Create required RBAC configuration
func configureRBAC(cs clientset.Interface, ns string) error {
	_, err := createServiceAccount(cs, ns)
	if err != nil {
		return err
	}

	_, err = createClusterRole(cs)
	if err != nil {
		return err
	}

	_, err = createClusterRoleBinding(cs, ns)
	if err != nil {
		return err
	}

	return nil
}

// Remove RBAC configuration
func deconfigureRBAC(cs clientset.Interface, ns string) error {
	err := cs.RbacV1().ClusterRoleBindings().Delete(context.TODO(), "nfd-master-e2e", metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	err = cs.RbacV1().ClusterRoles().Delete(context.TODO(), "nfd-master-e2e", metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	err = cs.CoreV1().ServiceAccounts(ns).Delete(context.TODO(), "nfd-master-e2e", metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Configure service account required by NFD
func createServiceAccount(cs clientset.Interface, ns string) (*v1.ServiceAccount, error) {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nfd-master-e2e",
			Namespace: ns,
		},
	}
	return cs.CoreV1().ServiceAccounts(ns).Create(context.TODO(), sa, metav1.CreateOptions{})
}

// Configure cluster role required by NFD
func createClusterRole(cs clientset.Interface) (*rbacv1.ClusterRole, error) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-master-e2e",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "patch", "update"},
			},
		},
	}
	if *openShift {
		cr.Rules = append(cr.Rules,
			rbacv1.PolicyRule{
				// needed on OpenShift clusters
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"hostaccess"},
				Verbs:         []string{"use"},
			})
	}
	return cs.RbacV1().ClusterRoles().Update(context.TODO(), cr, metav1.UpdateOptions{})
}

// Configure cluster role binding required by NFD
func createClusterRoleBinding(cs clientset.Interface, ns string) (*rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-master-e2e",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "nfd-master-e2e",
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "nfd-master-e2e",
		},
	}

	return cs.RbacV1().ClusterRoleBindings().Update(context.TODO(), crb, metav1.UpdateOptions{})
}

// createService creates nfd-master Service
func createService(cs clientset.Interface, ns string) (*v1.Service, error) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-master-e2e",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"name": "nfd-master-e2e"},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     8080,
				},
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
	return cs.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
}

func nfdMasterPod(image string, onMasterNode bool) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nfd-master-" + string(uuid.NewUUID()),
			Labels: map[string]string{"name": "nfd-master-e2e"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "node-feature-discovery",
					Image:           image,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"nfd-master"},
					Env: []v1.EnvVar{
						{
							Name: "NODE_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
				},
			},
			ServiceAccountName: "nfd-master-e2e",
			RestartPolicy:      v1.RestartPolicyNever,
		},
	}
	if onMasterNode {
		p.Spec.NodeSelector = map[string]string{"node-role.kubernetes.io/master": ""}
		p.Spec.Tolerations = []v1.Toleration{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: v1.TolerationOpEqual,
				Value:    "",
				Effect:   v1.TaintEffectNoSchedule,
			},
		}
	}
	return p
}

func nfdWorkerPod(image string, extraArgs []string) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: nfdWorkerPodSpec(image, extraArgs),
	}

	p.Spec.RestartPolicy = v1.RestartPolicyNever

	return p
}

func nfdWorkerDaemonSet(image string, extraArgs []string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "nfd-worker"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": "nfd-worker"},
				},
				Spec: nfdWorkerPodSpec(image, extraArgs),
			},
			MinReadySeconds: 5,
		},
	}
}

func nfdWorkerPodSpec(image string, extraArgs []string) v1.PodSpec {
	return v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:            "node-feature-discovery",
				Image:           image,
				ImagePullPolicy: v1.PullAlways,
				Command:         []string{"nfd-worker"},
				Args:            append([]string{"--server=nfd-master-e2e:8080"}, extraArgs...),
				Env: []v1.EnvVar{
					{
						Name: "NODE_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								FieldPath: "spec.nodeName",
							},
						},
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "host-boot",
						MountPath: "/host-boot",
						ReadOnly:  true,
					},
					{
						Name:      "host-os-release",
						MountPath: "/host-etc/os-release",
						ReadOnly:  true,
					},
					{
						Name:      "host-sys",
						MountPath: "/host-sys",
						ReadOnly:  true,
					},
					{
						Name:      "host-usr-lib",
						MountPath: "/host-usr/lib",
						ReadOnly:  true,
					},
					{
						Name:      "host-usr-src",
						MountPath: "/host-usr/src",
						ReadOnly:  true,
					},
				},
			},
		},
		ServiceAccountName: "nfd-master-e2e",
		DNSPolicy:          v1.DNSClusterFirstWithHostNet,
		Volumes: []v1.Volume{
			{
				Name: "host-boot",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/boot",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-os-release",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/etc/os-release",
						Type: newHostPathType(v1.HostPathFile),
					},
				},
			},
			{
				Name: "host-sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-usr-lib",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/usr/lib",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
			{
				Name: "host-usr-src",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/usr/src",
						Type: newHostPathType(v1.HostPathDirectory),
					},
				},
			},
		},
	}

}

func newHostPathType(typ v1.HostPathType) *v1.HostPathType {
	hostPathType := new(v1.HostPathType)
	*hostPathType = v1.HostPathType(typ)
	return hostPathType
}

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
				if strings.HasPrefix(key, master.FeatureLabelNs) {
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
var _ = SIGDescribe("Node Feature Discovery", func() {
	f := framework.NewDefaultFramework("node-feature-discovery")

	Context("when deploying a single nfd-master pod", func() {
		var masterPod *v1.Pod

		BeforeEach(func() {
			err := configureRBAC(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			// Launch nfd-master
			By("Creating nfd master pod and nfd-master service")
			image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
			masterPod = nfdMasterPod(image, false)
			masterPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), masterPod, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Create nfd-master service
			nfdSvc, err := createService(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the nfd-master pod to be running")
			Expect(e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, masterPod.Name, masterPod.Namespace, time.Minute)).NotTo(HaveOccurred())

			By("Waiting for the nfd-master service to be up")
			Expect(e2enetwork.WaitForService(f.ClientSet, f.Namespace.Name, nfdSvc.ObjectMeta.Name, true, time.Second, 10*time.Second)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := deconfigureRBAC(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

		})

		//
		// Simple test with only the fake source enabled
		//
		Context("and a single worker pod with fake source enabled", func() {
			It("it should decorate the node with the fake feature labels", func() {

				fakeFeatureLabels := map[string]string{
					master.FeatureLabelNs + "/fake-fakefeature1": "true",
					master.FeatureLabelNs + "/fake-fakefeature2": "true",
					master.FeatureLabelNs + "/fake-fakefeature3": "true",
				}

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				// Launch nfd-worker
				By("Creating a nfd worker pod")
				image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
				workerPod := nfdWorkerPod(image, []string{"--oneshot", "--sources=fake"})
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
					if strings.HasPrefix(k, master.FeatureLabelNs) {
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
				readConfig()
				if conf == nil {
					Skip("no e2e-config was specified")
				}
				if conf.DefaultFeatures == nil {
					Skip("no 'defaultFeatures' specified in e2e-config")
				}
				fConf := conf.DefaultFeatures

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				By("Creating nfd-worker daemonset")
				workerDS := nfdWorkerDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})
				workerDS, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), workerDS, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for daemonset pods to be ready")
				Expect(e2epod.WaitForPodsReady(f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 5)).NotTo(HaveOccurred())

				By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				for _, node := range nodeList.Items {
					var nodeConf *nodeConfig
					for _, conf := range fConf.Nodes {
						if conf.nameRe.MatchString(node.Name) {
							e2elog.Logf("node %q matches rule %q", node.Name, conf.nameRe)
							nodeConf = &conf
							break
						}
					}
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
						if strings.HasPrefix(k, master.FeatureLabelNs) {
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
				workerDS := nfdWorkerDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})

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
