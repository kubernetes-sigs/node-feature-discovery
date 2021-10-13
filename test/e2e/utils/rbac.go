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

package utils

import (
	"context"
	"flag"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var (
	openShift = flag.Bool("nfd.openshift", false, "Enable OpenShift specific bits")
)

// ConfigureRBAC creates required RBAC configuration
func ConfigureRBAC(cs clientset.Interface, ns string) error {
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

// DeconfigureRBAC removes RBAC configuration
func DeconfigureRBAC(cs clientset.Interface, ns string) error {
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
			{
				APIGroups: []string{"topology.node.k8s.io"},
				Resources: []string{"noderesourcetopologies"},
				Verbs: []string{
					"create",
					"get",
					"update",
				},
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
