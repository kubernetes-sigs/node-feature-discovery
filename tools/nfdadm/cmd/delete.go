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
	"fmt"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/node-feature-discovery/tools/nfdadm/common"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type DeleteCmdFlags struct {
	kubeconfig string
	namespace  string
}

var deleteCmdFlags = &DeleteCmdFlags{}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().StringVarP(&deleteCmdFlags.kubeconfig, "kubeconfig", "c", common.DefaultKubeconfig(), "Kubeconfig file to use for communicating with the API server")
	deleteCmd.Flags().StringVarP(&deleteCmdFlags.namespace, "namespace", "n", "default", "Namespace where node-feature-discovery is deployed")
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete NFD",
	Long:  "Delete Node Feature Discovery from a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		errors := false

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

		// Check that the given namespace is valid
		_, err = clientset.CoreV1().Namespaces().Get(deleteCmdFlags.namespace, metav1.GetOptions{})
		if err != nil {
			glog.Exitf("Namespace '%v' not found", deleteCmdFlags.namespace)
		}

		// Try to delete DaemonSet object
		foundDS, err := deleteDaemonSet(deleteCmdFlags.namespace, clientset)
		if err != nil {
			glog.Errorf("failed to delete DaemonSet: %v", err)
			errors = true
		}

		// Try to delete Job object
		foundJob, err := deleteJob(deleteCmdFlags.namespace, clientset)
		if err != nil {
			glog.Errorf("failed to delete Job: %v", err)
			errors = true
		}

		if foundDS == false && foundJob == false {
			glog.Errorf("failed to find workload (Job or DaemonSet)")
			errors = true
		}

		// Remove RBAC objects
		err = deleteRBAC(deleteCmdFlags.namespace, clientset)
		if err != nil {
			glog.Errorf("failed to delete RBAC objects: %v", err)
			errors = true
		}

		if errors == true {
			glog.Exitf("errors were encountered when deleting objects!")
		} else {
			glog.Info("node feature discovery successfully deleted")
		}
	},
}

func deleteDaemonSet(namespace string, clientset kubernetes.Interface) (bool, error) {
	dsInterface := clientset.AppsV1().DaemonSets(namespace)

	// Check if a DaemonSet object exists
	if _, err := dsInterface.Get("node-feature-discovery", metav1.GetOptions{}); err != nil {
		glog.Info("DaemonSet object for node feature discovery not found")
		return false, nil
	}

	glog.Info("deleting DaemonSet object")
	return true, dsInterface.Delete("node-feature-discovery", &metav1.DeleteOptions{})
}

func deleteJob(namespace string, clientset kubernetes.Interface) (bool, error) {
	jobInterface := clientset.BatchV1().Jobs(namespace)

	// Check if a DaemonSet object exists
	if _, err := jobInterface.Get("node-feature-discovery", metav1.GetOptions{}); err != nil {
		glog.Info("Job object for node feature discovery not found")
		return false, nil
	}

	glog.Info("deleting Job object")
	return true, jobInterface.Delete("node-feature-discovery", &metav1.DeleteOptions{})
}

func deleteRBAC(namespace string, clientset kubernetes.Interface) error {
	var failures []string

	glog.Info("deleting ClusterRoleBinding")
	err := clientset.RbacV1().ClusterRoleBindings().Delete("node-feature-discovery", &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf("failed to delete ClusterRoleBinding: %v", err)
		failures = append(failures, "ClusterRoleBinding")
	}

	glog.Info("deleting ClusterRole")
	err = clientset.RbacV1().ClusterRoles().Delete("node-feature-discovery", &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf("failed to delete ClusterRole: %v", err)
		failures = append(failures, "ClusterRole")
	}

	glog.Info("deleting ServiceAccount")
	err = clientset.CoreV1().ServiceAccounts(namespace).Delete("node-feature-discovery", &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf("failed to delete ServiceAccount: %v", err)
		failures = append(failures, "ServiceAccount")
	}

	if len(failures) > 0 {
		return fmt.Errorf("unable to delete some objects %v", failures)
	}
	return nil
}
