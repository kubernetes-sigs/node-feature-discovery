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
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

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
	"sigs.k8s.io/yaml"
)

var (
	dockerRepo    = flag.String("nfd.repo", "quay.io/kubernetes_incubator/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag     = flag.String("nfd.tag", "e2e-test", "Docker tag to use")
	e2eConfigFile = flag.String("nfd.e2e-config", "", "Configuration parameters for end-to-end tests")

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

	ginkgo.By("Reading end-to-end test configuration file")
	data, err := ioutil.ReadFile(*e2eConfigFile)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Parsing end-to-end test configuration data")
	err = yaml.Unmarshal(data, &conf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	for _, n := range nodeList.Items {
		var err error
		var node *v1.Node
		for retry := 0; retry < 5; retry++ {
			node, err = cs.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

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
				if strings.HasPrefix(key, master.AnnotationNs) {
					delete(node.Annotations, key)
					update = true
				}
			}

			if !update {
				break
			}

			ginkgo.By("Deleting NFD labels and annotations from node " + node.Name)
			_, err = cs.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			if err != nil {
				time.Sleep(100 * time.Millisecond)
			} else {
				break
			}

		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

// Actual test suite
var _ = framework.KubeDescribe("[NFD] Node Feature Discovery", func() {
	f := framework.NewDefaultFramework("node-feature-discovery")

	ginkgo.Context("when deploying a single nfd-master pod", func() {
		var masterPod *v1.Pod

		ginkgo.BeforeEach(func() {
			err := configureRBAC(f.ClientSet, f.Namespace.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Launch nfd-master
			ginkgo.By("Creating nfd master pod and nfd-master service")
			image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
			masterPod = nfdMasterPod(image, false)
			masterPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), masterPod, metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Create nfd-master service
			nfdSvc, err := createService(f.ClientSet, f.Namespace.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Waiting for the nfd-master pod to be running")
			gomega.Expect(e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, masterPod.Name, masterPod.Namespace, time.Minute)).NotTo(gomega.HaveOccurred())

			ginkgo.By("Waiting for the nfd-master service to be up")
			gomega.Expect(e2enetwork.WaitForService(f.ClientSet, f.Namespace.Name, nfdSvc.ObjectMeta.Name, true, time.Second, 10*time.Second)).NotTo(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			err := deconfigureRBAC(f.ClientSet, f.Namespace.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

		})

		//
		// Simple test with only the fake source enabled
		//
		ginkgo.Context("and a single worker pod with fake source enabled", func() {
			ginkgo.It("it should decorate the node with the fake feature labels", func() {

				fakeFeatureLabels := map[string]string{
					master.LabelNs + "fake-fakefeature1": "true",
					master.LabelNs + "fake-fakefeature2": "true",
					master.LabelNs + "fake-fakefeature3": "true",
				}

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				// Launch nfd-worker
				ginkgo.By("Creating a nfd worker pod")
				image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
				workerPod := nfdWorkerPod(image, []string{"--oneshot", "--sources=fake"})
				workerPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), workerPod, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By("Waiting for the nfd-worker pod to succeed")
				gomega.Expect(e2epod.WaitForPodSuccessInNamespace(f.ClientSet, workerPod.ObjectMeta.Name, f.Namespace.Name)).NotTo(gomega.HaveOccurred())
				workerPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), workerPod.ObjectMeta.Name, metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By(fmt.Sprintf("Making sure '%s' was decorated with the fake feature labels", workerPod.Spec.NodeName))
				node, err := f.ClientSet.CoreV1().Nodes().Get(context.TODO(), workerPod.Spec.NodeName, metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				for k, v := range fakeFeatureLabels {
					gomega.Expect(node.Labels[k]).To(gomega.Equal(v))
				}

				// Check that there are no unexpected NFD labels
				for k := range node.Labels {
					if strings.HasPrefix(k, master.LabelNs) {
						gomega.Expect(fakeFeatureLabels).Should(gomega.HaveKey(k))
					}
				}

				ginkgo.By("Deleting the node-feature-discovery worker pod")
				err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), workerPod.ObjectMeta.Name, metav1.DeleteOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				cleanupNode(f.ClientSet)
			})
		})

		//
		// More comprehensive test when --e2e-node-config is enabled
		//
		ginkgo.Context("and nfd-workers as a daemonset with default sources enabled", func() {
			ginkgo.It("the node labels and annotations listed in the e2e config should be present", func() {
				readConfig()
				if conf == nil {
					ginkgo.Skip("no e2e-config was specified")
				}
				if conf.DefaultFeatures == nil {
					ginkgo.Skip("no 'defaultFeatures' specified in e2e-config")
				}
				fConf := conf.DefaultFeatures

				// Remove pre-existing stale annotations and labels
				cleanupNode(f.ClientSet)

				ginkgo.By("Creating nfd-worker daemonset")
				workerDS := nfdWorkerDaemonSet(fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag), []string{})
				workerDS, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.TODO(), workerDS, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By("Waiting for daemonset pods to be ready")
				gomega.Expect(e2epod.WaitForPodsReady(f.ClientSet, f.Namespace.Name, workerDS.Spec.Template.Labels["name"], 5)).NotTo(gomega.HaveOccurred())

				ginkgo.By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				for _, node := range nodeList.Items {
					if _, ok := fConf.Nodes[node.Name]; !ok {
						e2elog.Logf("node %q missing from e2e-config, skipping...", node.Name)
						continue
					}
					nodeConf := fConf.Nodes[node.Name]

					// Check labels
					e2elog.Logf("verifying labels of node %q...", node.Name)
					for k, v := range nodeConf.ExpectedLabelValues {
						gomega.Expect(node.Labels).To(gomega.HaveKeyWithValue(k, v))
					}
					for k := range nodeConf.ExpectedLabelKeys {
						gomega.Expect(node.Labels).To(gomega.HaveKey(k))
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
							gomega.Expect(fConf.LabelWhitelist).NotTo(gomega.HaveKey(k))
						}
					}

					// Check annotations
					e2elog.Logf("verifying annotations of node %q...", node.Name)
					for k, v := range nodeConf.ExpectedAnnotationValues {
						gomega.Expect(node.Annotations).To(gomega.HaveKeyWithValue(k, v))
					}
					for k := range nodeConf.ExpectedAnnotationKeys {
						gomega.Expect(node.Annotations).To(gomega.HaveKey(k))
					}
					for k := range node.Annotations {
						if strings.HasPrefix(k, master.AnnotationNs) {
							if _, ok := nodeConf.ExpectedAnnotationValues[k]; ok {
								continue
							}
							if _, ok := nodeConf.ExpectedAnnotationKeys[k]; ok {
								continue
							}
							// Ignore if the annotation was not whitelisted
							gomega.Expect(fConf.AnnotationWhitelist).NotTo(gomega.HaveKey(k))
						}
					}

					// Node running nfd-master should have master version annotation
					if node.Name == masterPod.Spec.NodeName {
						gomega.Expect(node.Annotations).To(gomega.HaveKey(master.AnnotationNs + "master.version"))
					}
				}

				ginkgo.By("Deleting nfd-worker daemonset")
				err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Delete(context.TODO(), workerDS.ObjectMeta.Name, metav1.DeleteOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				cleanupNode(f.ClientSet)
			})
		})
	})

})
