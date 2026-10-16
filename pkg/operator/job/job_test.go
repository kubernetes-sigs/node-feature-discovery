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

package job

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	nfdv1 "sigs.k8s.io/node-feature-discovery/api/operator/v1"
	"sigs.k8s.io/node-feature-discovery/pkg/operator/client"
	"sigs.k8s.io/yaml"
)

var _ = Describe("GetJob", func() {
	var (
		ctrl   *gomock.Controller
		clnt   *client.MockClient
		jobAPI JobAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		jobAPI = NewJobAPI(clnt, scheme)
	})

	ctx := context.Background()

	It("test successfull and failed call to client", func() {
		namespace := "namespace"
		name := "name"
		clnt.EXPECT().Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, gomock.Any()).Return(nil)

		_, err := jobAPI.GetJob(ctx, namespace, name)
		Expect(err).To(BeNil())

		clnt.EXPECT().Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, gomock.Any()).Return(fmt.Errorf("some error"))

		job, err := jobAPI.GetJob(ctx, namespace, name)
		Expect(err).To(HaveOccurred())
		Expect(job).To(BeNil())
	})
})

var _ = Describe("CreatePruneJob", func() {
	var (
		ctrl   *gomock.Controller
		clnt   *client.MockClient
		jobAPI JobAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		jobAPI = NewJobAPI(clnt, scheme)
	})

	ctx := context.Background()

	// createPruneJob runs CreatePruneJob for the CR and returns the Job it
	// passed to the client.
	createPruneJob := func(nfdCR *nfdv1.NodeFeatureDiscovery) *batchv1.Job {
		var created *batchv1.Job
		clnt.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&batchv1.Job{})).DoAndReturn(
			func(_ context.Context, obj ctrlclient.Object, _ ...ctrlclient.CreateOption) error {
				created = obj.(*batchv1.Job)
				return nil
			},
		)
		Expect(jobAPI.CreatePruneJob(ctx, nfdCR)).To(Succeed())
		Expect(created).NotTo(BeNil())
		return created
	}

	pruneCR := func(pullPolicy string) *nfdv1.NodeFeatureDiscovery {
		return &nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "nfd",
			},
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image:           "test-image",
					ImagePullPolicy: pullPolicy,
				},
			},
		}
	}

	It("good flow, prune job populated with correct values", func() {
		created := createPruneJob(pruneCR(""))

		expectedYAMLFile, err := os.ReadFile("testdata/test_prune_job.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		testPruneJob := batchv1.Job{}
		err = yaml.Unmarshal(expectedJSON, &testPruneJob)
		Expect(err).To(BeNil())
		// CreatePruneJob does not set TypeMeta; the client fills it in.
		testPruneJob.TypeMeta = metav1.TypeMeta{}
		Expect(*created).To(BeComparableTo(testPruneJob))
	})

	It("prune job uses the image pull policy from the NFD CR", func() {
		// Never is neither the operator default (Always) nor the NFD chart
		// default (IfNotPresent), so only a policy read from the CR passes.
		created := createPruneJob(pruneCR("Never"))
		Expect(created.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))
	})

	It("prune job defaults the image pull policy to Always", func() {
		created := createPruneJob(pruneCR(""))
		Expect(created.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
	})
})
