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
	"flag"
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	dockerRepo  = flag.String("nfd.repo", "quay.io/kubernetes_incubator/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag   = flag.String("nfd.tag", "e2e-test", "Docker tag to use")
	labelPrefix = "feature.node.kubernetes.io/"
)

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
	err := cs.RbacV1().ClusterRoleBindings().Delete("nfd-master-e2e", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	err = cs.RbacV1().ClusterRoles().Delete("nfd-master-e2e", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	err = cs.CoreV1().ServiceAccounts(ns).Delete("nfd-master-e2e", &metav1.DeleteOptions{})
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
	return cs.CoreV1().ServiceAccounts(ns).Create(sa)
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
	return cs.RbacV1().ClusterRoles().Update(cr)
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

	return cs.RbacV1().ClusterRoleBindings().Update(crb)
}

// createService creates nfd-master Service
func createService(cs clientset.Interface, ns string) (*v1.Service, error) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-master-e2e",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"app": "nfd-master-e2e"},
			Ports: []v1.ServicePort{
				{
					Protocol: v1.ProtocolTCP,
					Port:     8080,
				},
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
	return cs.CoreV1().Services(ns).Create(svc)
}

func nfdMasterPod(ns string, image string, onMasterNode bool) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nfd-master-" + string(uuid.NewUUID()),
			Labels: map[string]string{"app": "nfd-master-e2e"},
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

func nfdWorkerPod(ns string, image string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfd-worker-" + string(uuid.NewUUID()),
		},
		Spec: v1.PodSpec{
			// NOTE: We omit Volumes/VolumeMounts, at the moment as we only test the fake source
			Containers: []v1.Container{
				{
					Name:            "node-feature-discovery",
					Image:           image,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"nfd-worker"},
					Args:            []string{"--oneshot", "--sources=fake", "--server=nfd-master-e2e:8080"},
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
			// NOTE: We do not set HostNetwork/DNSPolicy because we only test the fake source
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

// Actual test suite
var _ = framework.KubeDescribe("Node Feature Discovery", func() {
	f := framework.NewDefaultFramework("node-feature-discovery")

	BeforeEach(func() {
		err := configureRBAC(f.ClientSet, f.Namespace.Name)
		Expect(err).NotTo(HaveOccurred())

	})

	Context("when deployed with fake source enabled", func() {
		It("should decorate the node with the fake feature labels", func() {
			By("Creating a nfd master and worker pods and the nfd-master service on the selected node")
			ns := f.Namespace.Name
			image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
			fakeFeatureLabels := map[string]string{
				labelPrefix + "fake-fakefeature1": "true",
				labelPrefix + "fake-fakefeature2": "true",
				labelPrefix + "fake-fakefeature3": "true",
			}

			defer deconfigureRBAC(f.ClientSet, f.Namespace.Name)

			// Launch nfd-master
			masterPod := nfdMasterPod(ns, image, false)
			masterPod, err := f.ClientSet.CoreV1().Pods(ns).Create(masterPod)
			Expect(err).NotTo(HaveOccurred())

			// Create nfd-master service
			nfdSvc, err := createService(f.ClientSet, f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the nfd-master pod to be running")
			Expect(framework.WaitForPodRunningInNamespace(f.ClientSet, masterPod)).NotTo(HaveOccurred())

			By("Waiting for the nfd-master service to be up")
			Expect(framework.WaitForService(f.ClientSet, f.Namespace.Name, nfdSvc.ObjectMeta.Name, true, time.Second, 10*time.Second)).NotTo(HaveOccurred())

			// Launch nfd-worker
			workerPod := nfdWorkerPod(ns, image)
			workerPod, err = f.ClientSet.CoreV1().Pods(ns).Create(workerPod)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the nfd-worker pod to succeed")
			Expect(framework.WaitForPodSuccessInNamespace(f.ClientSet, workerPod.ObjectMeta.Name, ns)).NotTo(HaveOccurred())
			workerPod, err = f.ClientSet.CoreV1().Pods(ns).Get(workerPod.ObjectMeta.Name, metav1.GetOptions{})

			By(fmt.Sprintf("Making sure '%s' was decorated with the fake feature labels", workerPod.Spec.NodeName))
			node, err := f.ClientSet.CoreV1().Nodes().Get(workerPod.Spec.NodeName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			for k, v := range fakeFeatureLabels {
				Expect(node.Labels[k]).To(Equal(v))
			}

			By("Removing the fake feature labels advertised by the node-feature-discovery pod")
			for key := range fakeFeatureLabels {
				framework.RemoveLabelOffNode(f.ClientSet, workerPod.Spec.NodeName, key)
			}
		})
	})
})
