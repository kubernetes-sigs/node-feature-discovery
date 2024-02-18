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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("SetMasterDeploymentAsDesired", func() {
	var (
		deploymentAPI DeploymentAPI
	)

	BeforeEach(func() {
		deploymentAPI = NewDeploymentAPI(scheme)
	})

	ctx := context.Background()

	It("good flow, deployment object populated with correct values", func() {
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

		err := deploymentAPI.SetMasterDeploymentAsDesired(ctx, &nfdCR, &masterDep)

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
