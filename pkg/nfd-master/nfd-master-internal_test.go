/*
Copyright 2019-2021 The Kubernetes Authors.

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

package nfdmaster

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/smartystreets/assertions"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/yaml"
)

const (
	mockNodeName = "mock-node"
)

func newMockNode() *corev1.Node {
	n := corev1.Node{}
	n.Name = mockNodeName
	n.Labels = map[string]string{}
	n.Annotations = map[string]string{}
	n.Status.Capacity = corev1.ResourceList{}
	return &n
}

func newMockMaster(apihelper apihelper.APIHelpers) *nfdMaster {
	return &nfdMaster{
		nodeName:  mockNodeName,
		config:    &NFDConfig{LabelWhiteList: utils.RegexpVal{Regexp: *regexp.MustCompile("")}},
		apihelper: apihelper,
	}
}

func TestUpdateNodeObject(t *testing.T) {
	Convey("When I update the node using fake client", t, func() {
		fakeFeatureLabels := map[string]string{
			nfdv1alpha1.FeatureLabelNs + "/source-feature.1": "1",
			nfdv1alpha1.FeatureLabelNs + "/source-feature.2": "2",
			nfdv1alpha1.FeatureLabelNs + "/source-feature.3": "val3",
			nfdv1alpha1.ProfileLabelNs + "/profile-a":        "val4"}
		fakeAnnotations := map[string]string{"my-annotation": "my-val"}
		fakeExtResources := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/source-feature.1": "1", nfdv1alpha1.FeatureLabelNs + "/source-feature.2": "2"}

		fakeFeatureLabelNames := make([]string, 0, len(fakeFeatureLabels))
		for k := range fakeFeatureLabels {
			fakeFeatureLabelNames = append(fakeFeatureLabelNames, strings.TrimPrefix(k, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(fakeFeatureLabelNames)

		fakeExtResourceNames := make([]string, 0, len(fakeExtResources))
		for k := range fakeExtResources {
			fakeExtResourceNames = append(fakeExtResourceNames, strings.TrimPrefix(k, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(fakeExtResourceNames)

		// Create a list of expected node status patches
		statusPatches := []apihelper.JsonPatch{}
		for k, v := range fakeExtResources {
			statusPatches = append(statusPatches, apihelper.NewJsonPatch("add", "/status/capacity", k, v))
		}

		mockAPIHelper := new(apihelper.MockAPIHelpers)
		mockMaster := newMockMaster(mockAPIHelper)
		mockClient := &k8sclient.Clientset{}
		// Mock node with old features
		mockNode := newMockNode()
		mockNode.Labels[nfdv1alpha1.FeatureLabelNs+"/old-feature"] = "old-value"
		mockNode.Annotations[nfdv1alpha1.AnnotationNs+"/feature-labels"] = "old-feature"

		Convey("When I successfully update the node with feature labels", func() {
			// Create a list of expected node metadata patches
			metadataPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("replace", "/metadata/annotations", nfdv1alpha1.AnnotationNs+"/feature-labels", strings.Join(fakeFeatureLabelNames, ",")),
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.AnnotationNs+"/extended-resources", strings.Join(fakeExtResourceNames, ",")),
				apihelper.NewJsonPatch("remove", "/metadata/labels", nfdv1alpha1.FeatureLabelNs+"/old-feature", ""),
			}
			for k, v := range fakeFeatureLabels {
				metadataPatches = append(metadataPatches, apihelper.NewJsonPatch("add", "/metadata/labels", k, v))
			}
			for k, v := range fakeAnnotations {
				metadataPatches = append(metadataPatches, apihelper.NewJsonPatch("add", "/metadata/annotations", k, v))
			}

			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Twice()
			mockAPIHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(statusPatches))).Return(nil)
			mockAPIHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(metadataPatches))).Return(nil)
			err := mockMaster.updateNodeObject(mockClient, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources, nil)

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When I fail to update the node with feature labels", func() {
			expectedError := fmt.Errorf("no client is passed, client:  <nil>")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := mockMaster.updateNodeObject(nil, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources, nil)

			Convey("Error is produced", func() {
				So(err, ShouldResemble, expectedError)
			})
		})

		Convey("When I fail to get a mock client while updating feature labels", func() {
			expectedError := fmt.Errorf("no client is passed, client:  <nil>")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := mockMaster.updateNodeObject(nil, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources, nil)

			Convey("Error is produced", func() {
				So(err, ShouldResemble, expectedError)
			})
		})

		Convey("When I fail to get a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(nil, expectedError).Twice()
			err := mockMaster.updateNodeObject(mockClient, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources, nil)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to update a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Twice()
			mockAPIHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(statusPatches))).Return(nil)
			mockAPIHelper.On("PatchNode", mockClient, mockNodeName, mock.Anything).Return(expectedError).Twice()
			err := mockMaster.updateNodeObject(mockClient, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources, nil)

			Convey("Error is produced", func() {
				So(err.Error(), ShouldEndWith, expectedError.Error())
			})
		})

	})
}

func TestUpdateMasterNode(t *testing.T) {
	Convey("When updating the nfd-master node", t, func() {
		mockHelper := &apihelper.MockAPIHelpers{}
		mockMaster := newMockMaster(mockHelper)
		mockClient := &k8sclient.Clientset{}
		mockNode := newMockNode()
		Convey("When update operation succeeds", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.AnnotationNs+"/master.version", version.Get())}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			err := mockMaster.updateMasterNode()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})

		mockErr := errors.New("failed to patch node annotations: mock-error'")
		Convey("When getting API client fails", func() {
			mockHelper.On("GetClient").Return(mockClient, mockErr)
			err := mockMaster.updateMasterNode()
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		Convey("When getting API node object fails", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, mockErr)
			err := mockMaster.updateMasterNode()
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		Convey("When updating node object fails", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.Anything).Return(mockErr)
			err := mockMaster.updateMasterNode()
			Convey("An error should be returned", func() {
				So(err.Error(), ShouldEndWith, mockErr.Error())
			})
		})
	})
}

func TestAddingExtResources(t *testing.T) {
	Convey("When adding extended resources", t, func() {
		mockMaster := newMockMaster(nil)
		Convey("When there are no matching labels", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{}
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When there are matching labels", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{"feature-1": "1", "feature-2": "2"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/status/capacity", "feature-1", "1"),
				apihelper.NewJsonPatch("add", "/status/capacity", "feature-2", "2"),
			}
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})

		Convey("When the resource already exists", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockResourceLabels := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-1": "1"}
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When the resource already exists but its capacity has changed", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[corev1.ResourceName("feature-1")] = *resource.NewQuantity(2, resource.BinarySI)
			mockResourceLabels := ExtendedResources{"feature-1": "1"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("replace", "/status/capacity", "feature-1", "1"),
				apihelper.NewJsonPatch("replace", "/status/allocatable", "feature-1", "1"),
			}
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})
	})
}

func TestRemovingExtResources(t *testing.T) {
	Convey("When removing extended resources", t, func() {
		mockMaster := newMockMaster(nil)
		Convey("When none are removed", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-1": "1", nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			mockNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})
		Convey("When the related label is gone", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-4": "", nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			mockNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-4,feature-2"
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-4")] = *resource.NewQuantity(4, resource.BinarySI)
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
		Convey("When the extended resource is no longer wanted", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			mockResourceLabels := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			mockNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			patches := mockMaster.createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
	})
}

func TestSetLabels(t *testing.T) {
	Convey("When servicing SetLabels request", t, func() {
		const workerName = mockNodeName
		const workerVer = "0.1-test"
		mockHelper := &apihelper.MockAPIHelpers{}
		mockMaster := newMockMaster(mockHelper)
		mockClient := &k8sclient.Clientset{}
		mockNode := newMockNode()
		mockCtx := context.Background()
		// In the gRPC request the label names may omit the default ns
		mockLabels := map[string]string{"feature-1": "1", "feature-2": "val-2", "feature-3": "3"}
		mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: mockLabels}

		mockLabelNames := make([]string, 0, len(mockLabels))
		for k := range mockLabels {
			mockLabelNames = append(mockLabelNames, k)
		}
		sort.Strings(mockLabelNames)

		expectedStatusPatches := []apihelper.JsonPatch{}

		Convey("When node update succeeds", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.WorkerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.FeatureLabelsAnnotation, strings.Join(mockLabelNames, ",")),
			}
			for k, v := range mockLabels {
				expectedPatches = append(expectedPatches, apihelper.NewJsonPatch("add", "/metadata/labels", nfdv1alpha1.FeatureLabelNs+"/"+k, v))
			}

			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil).Twice()
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When -label-whitelist is specified", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.WorkerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.FeatureLabelsAnnotation, "feature-2"),
				apihelper.NewJsonPatch("add", "/metadata/labels", nfdv1alpha1.FeatureLabelNs+"/feature-2", mockLabels["feature-2"]),
			}

			mockMaster.config.LabelWhiteList.Regexp = *regexp.MustCompile("^f.*2$")
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When -extra-label-ns, -deny-label-ns and -instance are specified", func() {
			// In the gRPC request the label names may omit the default ns
			instance := "foo"
			vendorFeatureLabel := "vendor." + nfdv1alpha1.FeatureLabelNs + "/feature-4"
			vendorProfileLabel := "vendor." + nfdv1alpha1.ProfileLabelNs + "/feature-5"
			mockLabels := map[string]string{
				"feature-1":                      "val-1",
				"valid.ns/feature-2":             "val-2",
				"random.denied.ns/feature-3":     "val-3",
				"kubernetes.io/feature-4":        "val-4",
				"sub.ns.kubernetes.io/feature-5": "val-5",
				vendorFeatureLabel:               "val-6",
				vendorProfileLabel:               "val-7",
				"--invalid-name--":               "valid-val",
				"valid-name":                     "--invalid-val--"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", instance+"."+nfdv1alpha1.WorkerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations",
					instance+"."+nfdv1alpha1.FeatureLabelsAnnotation,
					"feature-1,valid.ns/feature-2,"+vendorFeatureLabel+","+vendorProfileLabel),
				apihelper.NewJsonPatch("add", "/metadata/labels", nfdv1alpha1.FeatureLabelNs+"/feature-1", mockLabels["feature-1"]),
				apihelper.NewJsonPatch("add", "/metadata/labels", "valid.ns/feature-2", mockLabels["valid.ns/feature-2"]),
				apihelper.NewJsonPatch("add", "/metadata/labels", vendorFeatureLabel, mockLabels[vendorFeatureLabel]),
				apihelper.NewJsonPatch("add", "/metadata/labels", vendorProfileLabel, mockLabels[vendorProfileLabel]),
			}

			mockMaster.deniedNs.normal = map[string]struct{}{"random.denied.ns": {}}
			mockMaster.deniedNs.wildcard = map[string]struct{}{"kubernetes.io": {}}
			mockMaster.config.ExtraLabelNs = map[string]struct{}{"valid.ns": {}}
			mockMaster.args.Instance = instance
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: mockLabels}
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			mockMaster.args.Instance = ""
		})

		Convey("When -resource-labels is specified", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.WorkerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.FeatureLabelsAnnotation, "feature-2"),
				apihelper.NewJsonPatch("add", "/metadata/annotations", nfdv1alpha1.ExtendedResourceAnnotation, "feature-1,feature-3"),
				apihelper.NewJsonPatch("add", "/metadata/labels", nfdv1alpha1.FeatureLabelNs+"/feature-2", mockLabels["feature-2"]),
			}
			expectedStatusPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/status/capacity", nfdv1alpha1.FeatureLabelNs+"/feature-1", mockLabels["feature-1"]),
				apihelper.NewJsonPatch("add", "/status/capacity", nfdv1alpha1.FeatureLabelNs+"/feature-3", mockLabels["feature-3"]),
			}

			mockMaster.config.ResourceLabels = map[string]struct{}{"feature-3": {}, "feature-1": {}}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		mockErr := errors.New("mock-error")
		Convey("When node update fails", func() {
			mockHelper.On("GetClient").Return(mockClient, mockErr)
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		mockMaster.config.NoPublish = true
		Convey("With '-no-publish'", func() {
			_, err := mockMaster.SetLabels(mockCtx, mockReq)
			Convey("Operation should succeed", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestCreatePatches(t *testing.T) {
	Convey("When creating JSON patches", t, func() {
		existingItems := map[string]string{"key-1": "val-1", "key-2": "val-2", "key-3": "val-3"}
		jsonPath := "/root"

		Convey("When when there are neither itmes to remoe nor to add or update", func() {
			p := createPatches([]string{"foo", "bar"}, existingItems, map[string]string{}, jsonPath)
			So(len(p), ShouldEqual, 0)
		})

		Convey("When when there are itmes to remoe but none to add or update", func() {
			p := createPatches([]string{"key-2", "key-3", "foo"}, existingItems, map[string]string{}, jsonPath)
			expected := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("remove", jsonPath, "key-2", ""),
				apihelper.NewJsonPatch("remove", jsonPath, "key-3", ""),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})

		Convey("When when there are no itmes to remove but new items to add", func() {
			newItems := map[string]string{"new-key": "new-val", "key-1": "new-1"}
			p := createPatches([]string{"key-1"}, existingItems, newItems, jsonPath)
			expected := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", jsonPath, "new-key", newItems["new-key"]),
				apihelper.NewJsonPatch("replace", jsonPath, "key-1", newItems["key-1"]),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})

		Convey("When when there are items to remove add and update", func() {
			newItems := map[string]string{"new-key": "new-val", "key-2": "new-2", "key-4": "val-4"}
			p := createPatches([]string{"key-1", "key-2", "key-3", "foo"}, existingItems, newItems, jsonPath)
			expected := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", jsonPath, "new-key", newItems["new-key"]),
				apihelper.NewJsonPatch("add", jsonPath, "key-4", newItems["key-4"]),
				apihelper.NewJsonPatch("replace", jsonPath, "key-2", newItems["key-2"]),
				apihelper.NewJsonPatch("remove", jsonPath, "key-1", ""),
				apihelper.NewJsonPatch("remove", jsonPath, "key-3", ""),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})
	})
}

func TestRemoveLabelsWithPrefix(t *testing.T) {
	Convey("When removing labels", t, func() {
		n := &corev1.Node{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: map[string]string{
					"single-label": "123",
					"multiple_A":   "a",
					"multiple_B":   "b",
				},
			},
		}

		Convey("a unique label should be removed", func() {
			p := removeLabelsWithPrefix(n, "single")
			So(p, ShouldResemble, []apihelper.JsonPatch{apihelper.NewJsonPatch("remove", "/metadata/labels", "single-label", "")})
		})

		Convey("a non-unique search string should remove all matching keys", func() {
			p := removeLabelsWithPrefix(n, "multiple")
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches([]apihelper.JsonPatch{
				apihelper.NewJsonPatch("remove", "/metadata/labels", "multiple_A", ""),
				apihelper.NewJsonPatch("remove", "/metadata/labels", "multiple_B", ""),
			}))
		})

		Convey("a search string with no matches should not alter labels", func() {
			removeLabelsWithPrefix(n, "unique")
			So(n.Labels, ShouldContainKey, "single-label")
			So(n.Labels, ShouldContainKey, "multiple_A")
			So(n.Labels, ShouldContainKey, "multiple_B")
			So(len(n.Labels), ShouldEqual, 3)
		})
	})
}

func TestConfigParse(t *testing.T) {
	Convey("When parsing configuration", t, func() {
		m, err := NewNfdMaster(&Args{})
		So(err, ShouldBeNil)
		master := m.(*nfdMaster)
		overrides := `{"noPublish": true, "enableTaints": true, "extraLabelNs": ["added.ns.io","added.kubernetes.io"], "denyLabelNs": ["denied.ns.io","denied.kubernetes.io"], "resourceLabels": ["vendor-1.com/feature-1","vendor-2.io/feature-2"], "labelWhiteList": "foo"}`

		Convey("and no core cmdline flags have been specified", func() {
			So(master.configure("non-existing-file", overrides), ShouldBeNil)
			Convey("overrides should be in effect", func() {
				So(master.config.NoPublish, ShouldResemble, true)
				So(master.config.EnableTaints, ShouldResemble, true)
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"added.ns.io": struct{}{}, "added.kubernetes.io": struct{}{}})
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}, "denied.kubernetes.io": struct{}{}})
				So(master.config.ResourceLabels, ShouldResemble, utils.StringSetVal{"vendor-1.com/feature-1": struct{}{}, "vendor-2.io/feature-2": struct{}{}})
				So(master.config.LabelWhiteList.String(), ShouldEqual, "foo")
			})
		})
		Convey("and a non-accessible file, but cmdline flags and some overrides are specified", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{
				ExtraLabelNs: &utils.StringSetVal{"override.added.ns.io": struct{}{}},
				DenyLabelNs:  &utils.StringSetVal{"override.denied.ns.io": struct{}{}}}}
			So(master.configure("non-existing-file", overrides), ShouldBeNil)

			Convey("cmdline flags should be in effect instead overrides", func() {
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"override.added.ns.io": struct{}{}})
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"override.denied.ns.io": struct{}{}})
			})
			Convey("overrides should take effect", func() {
				So(master.config.NoPublish, ShouldBeTrue)
				So(master.config.EnableTaints, ShouldBeTrue)
			})
		})
		// Create a temporary config file
		f, err := os.CreateTemp("", "nfd-test-")
		defer os.Remove(f.Name())
		So(err, ShouldBeNil)
		_, err = f.WriteString(`
noPublish: true
denyLabelNs: ["denied.ns.io","denied.kubernetes.io"]
resourceLabels: ["vendor-1.com/feature-1","vendor-2.io/feature-2"]
enableTaints: false
labelWhiteList: "foo"
leaderElection:
  leaseDuration: 20s
  renewDeadline: 4s
  retryPeriod: 30s
`)
		f.Close()
		So(err, ShouldBeNil)

		Convey("and a proper config file is specified", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{ExtraLabelNs: &utils.StringSetVal{"override.added.ns.io": struct{}{}}}}
			So(master.configure(f.Name(), ""), ShouldBeNil)
			Convey("specified configuration should take effect", func() {
				// Verify core config
				So(master.config.NoPublish, ShouldBeTrue)
				So(master.config.EnableTaints, ShouldBeFalse)
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"override.added.ns.io": struct{}{}})
				So(master.config.ResourceLabels, ShouldResemble, utils.StringSetVal{"vendor-1.com/feature-1": struct{}{}, "vendor-2.io/feature-2": struct{}{}}) // from cmdline
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}, "denied.kubernetes.io": struct{}{}})
				So(master.config.LabelWhiteList.String(), ShouldEqual, "foo")
				So(master.config.LeaderElection.LeaseDuration.Seconds(), ShouldEqual, float64(20))
				So(master.config.LeaderElection.RenewDeadline.Seconds(), ShouldEqual, float64(4))
				So(master.config.LeaderElection.RetryPeriod.Seconds(), ShouldEqual, float64(30))
			})
		})

		Convey("and a proper config file and overrides are given", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{DenyLabelNs: &utils.StringSetVal{"denied.ns.io": struct{}{}}}}
			overrides := `{"extraLabelNs": ["added.ns.io"], "noPublish": true}`
			So(master.configure(f.Name(), overrides), ShouldBeNil)

			Convey("overrides should take precedence over the config file", func() {
				// Verify core config
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"added.ns.io": struct{}{}}) // from overrides
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}}) // from cmdline
			})
		})
	})
}

func TestDynamicConfig(t *testing.T) {
	Convey("When running nfd-master", t, func() {
		tmpDir, err := os.MkdirTemp("", "*.nfd-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)

		// Create (temporary) dir for config
		configDir := filepath.Join(tmpDir, "subdir-1", "subdir-2", "master.conf")
		err = os.MkdirAll(configDir, 0755)
		So(err, ShouldBeNil)

		// Create config file
		configFile := filepath.Join(configDir, "master.conf")

		writeConfig := func(data string) {
			f, err := os.Create(configFile)
			So(err, ShouldBeNil)
			_, err = f.WriteString(data)
			So(err, ShouldBeNil)
			err = f.Close()
			So(err, ShouldBeNil)
		}
		writeConfig(`
extraLabelNs: ["added.ns.io"]
`)

		noPublish := true
		m, err := NewNfdMaster(&Args{
			ConfigFile: configFile,
			Overrides: ConfigOverrideArgs{
				NoPublish: &noPublish,
			},
		})
		So(err, ShouldBeNil)
		master := m.(*nfdMaster)

		Convey("config file updates should take effect", func() {
			go func() { _ = m.Run() }()
			defer m.Stop()
			// Check initial config
			time.Sleep(10 * time.Second)
			So(func() interface{} { return master.config.ExtraLabelNs },
				withTimeout, 2*time.Second, ShouldResemble, utils.StringSetVal{"added.ns.io": struct{}{}})

			// Update config and verify the effect
			writeConfig(`
extraLabelNs: ["override.ns.io"]
resyncPeriod: '2h'
`)
			So(func() interface{} { return master.config.ExtraLabelNs },
				withTimeout, 2*time.Second, ShouldResemble, utils.StringSetVal{"override.ns.io": struct{}{}})
			So(func() interface{} { return master.config.ResyncPeriod.Duration },
				withTimeout, 2*time.Second, ShouldResemble, time.Duration(2)*time.Hour)

			// Removing config file should get back our defaults
			err = os.RemoveAll(tmpDir)
			So(err, ShouldBeNil)
			So(func() interface{} { return master.config.ExtraLabelNs },
				withTimeout, 2*time.Second, ShouldResemble, utils.StringSetVal{})
			So(func() interface{} { return master.config.ResyncPeriod.Duration },
				withTimeout, 2*time.Second, ShouldResemble, time.Duration(1)*time.Hour)

			// Re-creating config dir and file should change the config
			err = os.MkdirAll(configDir, 0755)
			So(err, ShouldBeNil)
			writeConfig(`
extraLabelNs: ["another.override.ns"]
resyncPeriod: '3m'
`)
			So(func() interface{} { return master.config.ExtraLabelNs },
				withTimeout, 2*time.Second, ShouldResemble, utils.StringSetVal{"another.override.ns": struct{}{}})
			So(func() interface{} { return master.config.ResyncPeriod.Duration },
				withTimeout, 2*time.Second, ShouldResemble, time.Duration(3)*time.Minute)
		})
	})
}

// withTimeout is a custom assertion for polling a value asynchronously
// actual is a function for getting the actual value
// expected[0] is a time.Duration value specifying the timeout
// expected[1] is  the "real" assertion function to be called
// expected[2:] are the arguments for the "real" assertion function
func withTimeout(actual interface{}, expected ...interface{}) string {
	getter, ok := actual.(func() interface{})
	if !ok {
		return "not getterFunc"
	}
	t, ok := expected[0].(time.Duration)
	if !ok {
		return "not time.Duration"
	}
	f, ok := expected[1].(func(interface{}, ...interface{}) string)
	if !ok {
		return "not an assert func"
	}
	timeout := time.After(t)
	for {
		result := f(getter(), expected[2:]...)
		if result == "" {
			return ""
		}
		select {
		case <-timeout:
			return result
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func jsonPatchMatcher(expected []apihelper.JsonPatch) func([]apihelper.JsonPatch) bool {
	return func(actual []apihelper.JsonPatch) bool {
		// We don't care about modifying the original slices
		ok, msg := assertions.So(sortJsonPatches(actual), ShouldResemble, sortJsonPatches(expected))
		if !ok {
			// We parse the cryptic string message for better readability
			var f assertions.FailureView
			if err := yaml.Unmarshal([]byte(msg), &f); err == nil {
				_, _ = Printf("%s\n", f.Message)
			} else {
				_, _ = Printf("%s\n", msg)
			}
		}
		return ok
	}
}

func sortJsonPatches(p []apihelper.JsonPatch) []apihelper.JsonPatch {
	sort.Slice(p, func(i, j int) bool { return p[i].Path < p[j].Path })
	return p
}

// Remove any labels having the given prefix
func removeLabelsWithPrefix(n *corev1.Node, search string) []apihelper.JsonPatch {
	var p []apihelper.JsonPatch

	for k := range n.Labels {
		if strings.HasPrefix(k, search) {
			p = append(p, apihelper.NewJsonPatch("remove", "/metadata/labels", k, ""))
		}
	}

	return p
}
