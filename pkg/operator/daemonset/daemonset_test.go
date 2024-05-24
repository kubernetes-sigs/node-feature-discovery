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

package daemonset

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/client"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetTopologyDaemonsetAsDesired", func() {
	var (
		daemonsetAPI DaemonsetAPI
	)

	BeforeEach(func() {
		daemonsetAPI = NewDaemonsetAPI(nil, scheme)
	})

	ctx := context.Background()

	It("good flow, topology daemonset populated with correct values", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image: "test-image",
				},
			},
		}
		topologyDS := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-topology-updater",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "DaemonSet",
				APIVersion: "apps/v1",
			},
		}

		err := daemonsetAPI.SetTopologyDaemonsetAsDesired(ctx, &nfdCR, &topologyDS)

		Expect(err).To(BeNil())
		expectedYAMLFile, err := os.ReadFile("testdata/test_topology_daemonset.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		testTopologyDS := appsv1.DaemonSet{}
		err = yaml.Unmarshal(expectedJSON, &testTopologyDS)
		Expect(err).To(BeNil())
		Expect(topologyDS).To(BeComparableTo(testTopologyDS))
	})
})

var _ = Describe("SetWorkerDaemonsetAsDesired", func() {
	var (
		daemonsetAPI DaemonsetAPI
	)

	BeforeEach(func() {
		daemonsetAPI = NewDaemonsetAPI(nil, scheme)
	})

	ctx := context.Background()

	It("worker daemonset populated with correct values", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image: "test-image",
				},
			},
		}
		actualWorkerDS := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-worker",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "DaemonSet",
				APIVersion: "apps/v1",
			},
		}

		err := daemonsetAPI.SetWorkerDaemonsetAsDesired(ctx, &nfdCR, &actualWorkerDS)

		Expect(err).To(BeNil())
		expectedYAMLFile, err := os.ReadFile("testdata/test_worker_daemonset.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		expectedWorkerDS := appsv1.DaemonSet{}
		err = yaml.Unmarshal(expectedJSON, &expectedWorkerDS)
		Expect(err).To(BeNil())
		Expect(&expectedWorkerDS).To(BeComparableTo(&actualWorkerDS))
	})
})

var _ = Describe("DeleteDaemonSet", func() {
	var (
		ctrl         *gomock.Controller
		clnt         *client.MockClient
		daemonsetAPI DaemonsetAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		daemonsetAPI = NewDaemonsetAPI(clnt, scheme)
	})

	ctx := context.Background()
	name := "ds-name"
	namespace := "ds-namespace"

	expectedDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	It("failure to delete daemonset from the cluster", func() {
		clnt.EXPECT().Delete(ctx, expectedDS).Return(fmt.Errorf("some error"))

		err := daemonsetAPI.DeleteDaemonSet(ctx, namespace, name)
		Expect(err).To(HaveOccurred())
	})

	It("daemonset is not present in the cluster", func() {
		clnt.EXPECT().Delete(ctx, expectedDS).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever"))

		err := daemonsetAPI.DeleteDaemonSet(ctx, namespace, name)
		Expect(err).To(BeNil())
	})

	It("daemonset deleted successfully", func() {
		clnt.EXPECT().Delete(ctx, expectedDS).Return(nil)

		err := daemonsetAPI.DeleteDaemonSet(ctx, namespace, name)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("GetDaemonSet", func() {
	var (
		ctrl         *gomock.Controller
		clnt         *client.MockClient
		daemonsetAPI DaemonsetAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		daemonsetAPI = NewDaemonsetAPI(clnt, scheme)
	})

	ctx := context.Background()
	testName := "test-name"
	testNamespace := "test-namespace"

	It("good flow", func() {
		expectedDS := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testName},
		}
		clnt.EXPECT().Get(ctx, ctrlclient.ObjectKey{Namespace: testNamespace, Name: testName}, gomock.Any()).DoAndReturn(
			func(_ interface{}, _ interface{}, ds *appsv1.DaemonSet, _ ...ctrlclient.GetOption) error {
				ds.SetName(testName)
				ds.SetNamespace(testNamespace)
				return nil
			},
		)
		res, err := daemonsetAPI.GetDaemonSet(ctx, testNamespace, testName)
		Expect(err).To(BeNil())
		Expect(res).To(Equal(expectedDS))
	})

	It("error flow", func() {
		clnt.EXPECT().Get(ctx, ctrlclient.ObjectKey{Namespace: testNamespace, Name: testName}, gomock.Any()).Return(fmt.Errorf("some error"))
		_, err := daemonsetAPI.GetDaemonSet(ctx, testNamespace, testName)
		Expect(err).To(HaveOccurred())
	})
})
