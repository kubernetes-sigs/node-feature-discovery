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

package configmap

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetWorkerDaemonsetAsDesired", func() {
	var (
		configmapAPI ConfigMapAPI
	)

	BeforeEach(func() {
		configmapAPI = NewConfigMapAPI(scheme)
	})

	ctx := context.Background()

	It("worker config populated with correct values", func() {
		discoveryYAML, err := os.ReadFile("testdata/nfd.kubernetes.io_v1_nodefeaturediscovery.yaml")
		Expect(err).To(BeNil())
		discoveryJSON, err := yaml.YAMLToJSON(discoveryYAML)
		Expect(err).To(BeNil())
		actualNfdCR := nfdv1.NodeFeatureDiscovery{}
		err = yaml.Unmarshal(discoveryJSON, &actualNfdCR)
		Expect(err).To(BeNil())

		actualWorkerCM := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nfd-worker",
				Namespace: "test-namespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
		}

		err = configmapAPI.SetWorkerConfigMapAsDesired(ctx, &actualNfdCR, &actualWorkerCM)
		Expect(err).To(BeNil())
		expectedYAMLFile, err := os.ReadFile("testdata/test_worker_configmap.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		expectedWorkerCM := corev1.ConfigMap{}
		err = yaml.Unmarshal(expectedJSON, &expectedWorkerCM)
		Expect(err).To(BeNil())
		Expect(&expectedWorkerCM).To(BeComparableTo(&actualWorkerCM))
	})
})
