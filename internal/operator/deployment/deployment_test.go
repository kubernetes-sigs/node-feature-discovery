/*
Copyright 2024 The Kubernetes Authors.

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

package deployment

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/client"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetMasterDeploymentAsDesired", func() {
	var (
		deploymentAPI DeploymentAPI
	)

	BeforeEach(func() {
		deploymentAPI = NewDeploymentAPI(nil, scheme)
	})

	It("good flow, master deployment object populated with correct values", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image: "test-image",
				},
			},
		}
		masterDep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-master",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
		}

		err := deploymentAPI.SetMasterDeploymentAsDesired(&nfdCR, &masterDep)

		Expect(err).To(BeNil())
		expectedYAMLFile, err := os.ReadFile("testdata/test_master_deployment.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		testMasterDep := appsv1.Deployment{}
		err = yaml.Unmarshal(expectedJSON, &testMasterDep)
		Expect(err).To(BeNil())
		Expect(masterDep).To(BeComparableTo(testMasterDep))
	})
})

var _ = Describe("SetGCDeploymentAsDesired", func() {
	var (
		deploymentAPI DeploymentAPI
	)

	BeforeEach(func() {
		deploymentAPI = NewDeploymentAPI(nil, scheme)
	})

	It("good flow, GC deployment object populated with correct values", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image: "test-image",
				},
			},
		}
		masterDep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-gc",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
		}

		err := deploymentAPI.SetGCDeploymentAsDesired(&nfdCR, &masterDep)

		Expect(err).To(BeNil())
		expectedYAMLFile, err := os.ReadFile("testdata/test_gc_deployment.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		testMasterDep := appsv1.Deployment{}
		err = yaml.Unmarshal(expectedJSON, &testMasterDep)
		Expect(err).To(BeNil())
		Expect(masterDep).To(BeComparableTo(testMasterDep))
	})
})

var _ = Describe("DeleteDeployment", func() {
	var (
		ctrl          *gomock.Controller
		clnt          *client.MockClient
		deploymentAPI DeploymentAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		deploymentAPI = NewDeploymentAPI(clnt, scheme)
	})

	ctx := context.Background()
	name := "dep-name"
	namespace := "dep-namespace"
	expectedDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	It("failure to delete deployment from the cluster", func() {
		clnt.EXPECT().Delete(ctx, expectedDep).Return(fmt.Errorf("some error"))

		err := deploymentAPI.DeleteDeployment(ctx, namespace, name)
		Expect(err).To(HaveOccurred())
	})

	It("deployment is not present in the cluster", func() {
		clnt.EXPECT().Delete(ctx, expectedDep).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever"))

		err := deploymentAPI.DeleteDeployment(ctx, namespace, name)
		Expect(err).To(BeNil())
	})

	It("deployment deleted successfully", func() {
		clnt.EXPECT().Delete(ctx, expectedDep).Return(nil)

		err := deploymentAPI.DeleteDeployment(ctx, namespace, name)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("GetDeployment", func() {
	var (
		ctrl          *gomock.Controller
		clnt          *client.MockClient
		deploymentAPI DeploymentAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		deploymentAPI = NewDeploymentAPI(clnt, scheme)
	})

	ctx := context.Background()
	testName := "test-name"
	testNamespace := "test-namespace"

	It("good flow", func() {
		expectedDep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testName},
		}
		clnt.EXPECT().Get(ctx, ctrlclient.ObjectKey{Namespace: testNamespace, Name: testName}, gomock.Any()).DoAndReturn(
			func(_ interface{}, _ interface{}, dep *appsv1.Deployment, _ ...ctrlclient.GetOption) error {
				dep.SetName(testName)
				dep.SetNamespace(testNamespace)
				return nil
			},
		)
		res, err := deploymentAPI.GetDeployment(ctx, testNamespace, testName)
		Expect(err).To(BeNil())
		Expect(res).To(Equal(expectedDep))
	})

	It("error flow", func() {
		clnt.EXPECT().Get(ctx, ctrlclient.ObjectKey{Namespace: testNamespace, Name: testName}, gomock.Any()).Return(fmt.Errorf("some error"))
		_, err := deploymentAPI.GetDeployment(ctx, testNamespace, testName)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("getPodsTolerations", func() {
	It("no tolerations defined in the NFD CR", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{},
			},
		}
		expectedTolerations := []corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		res := getPodsTolerations(&nfdCR)
		Expect(res).To(Equal(expectedTolerations))
	})

	It("tolerations defined in the NFD CR", func() {
		masterTolerations := []corev1.Toleration{
			{
				Key:      "key1",
				Value:    "value1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "key1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					MasterTolerations: masterTolerations,
				},
			},
		}
		expectedTolerations := []corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "key1",
				Value:    "value1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "key1",
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		res := getPodsTolerations(&nfdCR)
		Expect(res).To(Equal(expectedTolerations))
	})
})
