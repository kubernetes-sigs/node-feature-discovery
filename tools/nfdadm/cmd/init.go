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

package cmd

import (
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/node-feature-discovery/tools/nfdadm/common"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type InitCmdFlags struct {
	image      string
	job        bool
	kubeconfig string
	namespace  string
	nfdArgs    []string
}

var initCmdFlags = &InitCmdFlags{}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initCmdFlags.image, "image", "i", "quay.io/kubernetes_incubator/node-feature-discovery:v0.1.0", "Image to use for the node-feature-discovery binary")
	initCmd.Flags().BoolVarP(&initCmdFlags.job, "job", "j", false, "Deploy node feature discovery as a one-shot Job, instead of DaemonSet")
	initCmd.Flags().StringVarP(&initCmdFlags.kubeconfig, "kubeconfig", "c", common.DefaultKubeconfig(), "Kubeconfig file to use for communicating with the API server")
	initCmd.Flags().StringVarP(&initCmdFlags.namespace, "namespace", "n", "default", "Namespace where node-feature-discovery is created")
	initCmd.Flags().StringArrayVarP(&initCmdFlags.nfdArgs, "nfd-arg", "a", []string{}, "Extra argument for the node-feature-discovery binary (can be specified multiple times)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize NFD",
	Long:  "Initialize Node Feature Discovery on a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		// Read kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", initCmdFlags.kubeconfig)
		if err != nil {
			glog.Exitf("failed to read kubeconfig: %v", err)
		}

		// Create the client interface
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			glog.Exitf("failed to create clientset: %v", err)
		}

		_, err = createNamespace(initCmdFlags.namespace, clientset)
		if err != nil {
			glog.Exitf("failed to create namespace: %v", err)
		}

		// Configure RBAC objects
		glog.Info("configuring RBAC")
		err = configureRBAC(initCmdFlags, clientset)
		if err != nil {
			glog.Exitf("failed to configure RBAC: %v", err)
		}

		if initCmdFlags.job == false {
			// Configure DaemonSet
			glog.Info("creating DaemonSet object for Node Feature Discovery")
			_, err = createDaemonSet(initCmdFlags, clientset)
			if err != nil {
				glog.Exitf("failed to create DaemonSet: %v", err)
			}
		} else {
			// Configure Job
			glog.Info("creating Job object for Node Feature Discovery")
			_, err = createJob(initCmdFlags, clientset)
			if err != nil {
				glog.Exitf("failed to create Job: %v", err)
			}
		}
	},
}

func createNamespace(name string, clientset kubernetes.Interface) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	// Check if namespace already exists
	ns, err := clientset.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
	if err == nil {
		glog.Infof("Namespace '%v' found", name)
		return ns, nil
	}

	glog.Infof("Creating namespace '%v'", name)
	return clientset.CoreV1().Namespaces().Create(ns)
}

func configureRBAC(flags *InitCmdFlags, clientset kubernetes.Interface) error {
	_, err := createServiceAccount(flags.namespace, clientset)
	if err != nil {
		glog.Errorf("failed to create ServiceAccount: %v", err)
		return err
	}

	_, err = createClusterRole(clientset)
	if err != nil {
		glog.Errorf("failed to create ClusterRole: %v", err)
		return err
	}

	_, err = createClusterRoleBinding(flags.namespace, clientset)
	if err != nil {
		glog.Errorf("failed to create ClusterRoleBinding: %v", err)
		return err
	}

	return nil
}

func createServiceAccount(namespace string, clientset kubernetes.Interface) (*v1.ServiceAccount, error) {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-feature-discovery",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-feature-discovery",
			},
		},
	}
	return clientset.CoreV1().ServiceAccounts(namespace).Create(sa)
}

func createClusterRole(clientset kubernetes.Interface) (*rbacv1.ClusterRole, error) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-feature-discovery",
			Labels: map[string]string{
				"app": "node-feature-discovery",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "pods"},
				Verbs:     []string{"get", "patch", "update"},
			},
		},
	}
	return clientset.RbacV1().ClusterRoles().Create(cr)
}

func createClusterRoleBinding(namespace string, clientset kubernetes.Interface) (*rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-feature-discovery",
			Labels: map[string]string{
				"app": "node-feature-discovery",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "node-feature-discovery",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "node-feature-discovery",
		},
	}

	return clientset.RbacV1().ClusterRoleBindings().Create(crb)
}

func podSpecCommon(imageName string) v1.PodSpec {
	return v1.PodSpec{
		Volumes: []v1.Volume{
			{
				Name: "host-sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
					},
				},
			},
		},
		Containers: []v1.Container{
			{
				Name:  "node-feature-discovery",
				Image: imageName,
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
						Name:      "host-sys",
						ReadOnly:  true,
						MountPath: "/host-sys",
					},
				},
			},
		},
		ServiceAccountName: "node-feature-discovery",
		HostNetwork:        true,
	}
}

func createDaemonSet(flags *InitCmdFlags, clientset kubernetes.Interface) (*appsv1.DaemonSet, error) {
	// Create pod spec from the common template
	podSpec := podSpecCommon(flags.image)
	podSpec.Containers[0].Args = append([]string{"--sleep-interval=60s"}, initCmdFlags.nfdArgs...)

	// Define the complete daemonset object
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-feature-discovery",
			Namespace: flags.namespace,
			Labels: map[string]string{
				"app": "node-feature-discovery",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "node-feature-discovery",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "node-feature-discovery",
					},
				},
				Spec: podSpec,
			},
		},
	}

	return clientset.AppsV1().DaemonSets(flags.namespace).Create(ds)
}

func createJob(flags *InitCmdFlags, clientset kubernetes.Interface) (*batchv1.Job, error) {
	// Create pod spec from the common template
	podSpec := podSpecCommon(flags.image)
	podSpec.RestartPolicy = v1.RestartPolicyNever
	podSpec.Containers[0].Args = append([]string{"--oneshot"}, initCmdFlags.nfdArgs...)

	numReadyNodes, err := getAvailableNodes(clientset)
	if err != nil {
		return nil, err
	}

	// Define the complete Job object
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-feature-discovery",
			Namespace: flags.namespace,
			Labels: map[string]string{
				"app": "node-feature-discovery",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &numReadyNodes,
			Completions: &numReadyNodes,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "node-feature-discovery",
					},
				},
				Spec: podSpec,
			},
		},
	}

	return clientset.BatchV1().Jobs(flags.namespace).Create(job)
}

// Get nodes that are available for running a workload
func getAvailableNodes(clientset kubernetes.Interface) (int32, error) {
	var readyNodes int32 = 0
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("failed to query nodes: %v", err)
		return 0, err
	}

	for _, node := range nodes.Items {
		if node.Spec.Unschedulable == false {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
					readyNodes += 1
				}
			}
		}
	}

	return readyNodes, nil
}
