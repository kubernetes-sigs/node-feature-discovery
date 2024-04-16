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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	nfdv1 "sigs.k8s.io/node-feature-discovery-operator/api/v1"
	"sigs.k8s.io/node-feature-discovery-operator/internal/client"
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

	It("good flow, prune job populated with correct values", func() {
		nfdCR := nfdv1.NodeFeatureDiscovery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "nfd",
			},
			Spec: nfdv1.NodeFeatureDiscoverySpec{
				Operand: nfdv1.OperandSpec{
					Image: "test-image",
				},
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/test_prune_job.yaml")
		Expect(err).To(BeNil())
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		testPruneJob := batchv1.Job{}
		err = yaml.Unmarshal(expectedJSON, &testPruneJob)
		Expect(err).To(BeNil())

		clnt.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&testPruneJob))

		err = jobAPI.CreatePruneJob(ctx, &nfdCR)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("DeleteJob", func() {
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

	deleteJob := batchv1.Job{}

	It("successfull deletion", func() {
		clnt.EXPECT().Delete(ctx, &deleteJob).Return(nil)

		err := jobAPI.DeleteJob(ctx, &deleteJob)
		Expect(err).To(BeNil())
	})

	It("job does not exist, function should return no error", func() {
		clnt.EXPECT().Delete(ctx, &deleteJob).Return(apierrors.NewNotFound(schema.GroupResource{}, "whatever"))

		err := jobAPI.DeleteJob(ctx, &deleteJob)
		Expect(err).To(BeNil())
	})

	It("delete fails, function should return error", func() {
		clnt.EXPECT().Delete(ctx, &deleteJob).Return(fmt.Errorf("some error"))

		err := jobAPI.DeleteJob(ctx, &deleteJob)
		Expect(err).To(HaveOccurred())
	})
})
