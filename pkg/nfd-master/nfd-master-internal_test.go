/*
Copyright 2019 The Kubernetes Authors.

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
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/smartystreets/assertions"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
	"sigs.k8s.io/yaml"
)

const (
	mockNodeName = "mock-node"
)

func init() {
	nodeName = mockNodeName
}

func newMockNode() *api.Node {
	n := api.Node{}
	n.Name = mockNodeName
	n.Labels = map[string]string{}
	n.Annotations = map[string]string{}
	n.Status.Capacity = api.ResourceList{}
	return &n
}

func TestUpdateNodeFeatures(t *testing.T) {
	Convey("When I update the node using fake client", t, func() {
		fakeFeatureLabels := map[string]string{LabelNs + "/source-feature.1": "1", LabelNs + "/source-feature.2": "2", LabelNs + "/source-feature.3": "val3"}
		fakeAnnotations := map[string]string{"my-annotation": "my-val"}
		fakeExtResources := ExtendedResources{LabelNs + "/source-feature.1": "1", LabelNs + "/source-feature.2": "2"}

		fakeFeatureLabelNames := make([]string, 0, len(fakeFeatureLabels))
		for k := range fakeFeatureLabels {
			fakeFeatureLabelNames = append(fakeFeatureLabelNames, strings.TrimPrefix(k, LabelNs+"/"))
		}
		sort.Strings(fakeFeatureLabelNames)

		fakeExtResourceNames := make([]string, 0, len(fakeExtResources))
		for k := range fakeExtResources {
			fakeExtResourceNames = append(fakeExtResourceNames, strings.TrimPrefix(k, LabelNs+"/"))
		}
		sort.Strings(fakeExtResourceNames)

		mockAPIHelper := new(apihelper.MockAPIHelpers)
		mockClient := &k8sclient.Clientset{}
		// Mock node with old features
		mockNode := newMockNode()
		mockNode.Labels[LabelNs+"/old-feature"] = "old-value"
		mockNode.Annotations[AnnotationNs+"/feature-labels"] = "old-feature"

		Convey("When I successfully update the node with feature labels", func() {
			// Create a list of expected node metadata patches
			metadataPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("replace", "/metadata/annotations", AnnotationNs+"/feature-labels", strings.Join(fakeFeatureLabelNames, ",")),
				apihelper.NewJsonPatch("add", "/metadata/annotations", AnnotationNs+"/extended-resources", strings.Join(fakeExtResourceNames, ",")),
				apihelper.NewJsonPatch("remove", "/metadata/labels", LabelNs+"/old-feature", ""),
			}
			for k, v := range fakeFeatureLabels {
				metadataPatches = append(metadataPatches, apihelper.NewJsonPatch("add", "/metadata/labels", k, v))
			}
			for k, v := range fakeAnnotations {
				metadataPatches = append(metadataPatches, apihelper.NewJsonPatch("add", "/metadata/annotations", k, v))
			}

			// Create a list of expected node status patches
			statusPatches := []apihelper.JsonPatch{}
			for k, v := range fakeExtResources {
				statusPatches = append(statusPatches, apihelper.NewJsonPatch("add", "/status/capacity", k, v))
			}

			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Once()
			mockAPIHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(metadataPatches))).Return(nil)
			mockAPIHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(statusPatches))).Return(nil)
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources)

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When I fail to update the node with feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to get a mock client while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to get a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(nil, expectedError).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to update a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Once()
			mockAPIHelper.On("PatchNode", mockClient, mockNodeName, mock.Anything).Return(expectedError).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations, fakeExtResources)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

	})
}

func TestUpdateMasterNode(t *testing.T) {
	Convey("When updating the nfd-master node", t, func() {
		mockHelper := &apihelper.MockAPIHelpers{}
		mockClient := &k8sclient.Clientset{}
		mockNode := newMockNode()
		Convey("When update operation succeeds", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", AnnotationNs+"/master.version", version.Get())}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			err := updateMasterNode(mockHelper)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})

		mockErr := errors.New("mock-error")
		Convey("When getting API client fails", func() {
			mockHelper.On("GetClient").Return(mockClient, mockErr)
			err := updateMasterNode(mockHelper)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		Convey("When getting API node object fails", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, mockErr)
			err := updateMasterNode(mockHelper)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		Convey("When updating node object fails", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.Anything).Return(mockErr)
			err := updateMasterNode(mockHelper)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})
	})
}

func TestAddingExtResources(t *testing.T) {
	Convey("When adding extended resources", t, func() {
		Convey("When there are no matching labels", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{}
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When there are matching labels", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{"feature-1": "1", "feature-2": "2"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/status/capacity", "feature-1", "1"),
				apihelper.NewJsonPatch("add", "/status/capacity", "feature-2", "2"),
			}
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})

		Convey("When the resource already exists", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockResourceLabels := ExtendedResources{LabelNs + "/feature-1": "1"}
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When the resource already exists but its capacity has changed", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[api.ResourceName("feature-1")] = *resource.NewQuantity(2, resource.BinarySI)
			mockResourceLabels := ExtendedResources{"feature-1": "1"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("replace", "/status/capacity", "feature-1", "1"),
				apihelper.NewJsonPatch("replace", "/status/allocatable", "feature-1", "1"),
			}
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})
	})
}

func TestRemovingExtResources(t *testing.T) {
	Convey("When removing extended resources", t, func() {
		Convey("When none are removed", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{LabelNs + "/feature-1": "1", LabelNs + "/feature-2": "2"}
			mockNode.Annotations[AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldEqual, 0)
		})
		Convey("When the related label is gone", func() {
			mockNode := newMockNode()
			mockResourceLabels := ExtendedResources{LabelNs + "/feature-4": "", LabelNs + "/feature-2": "2"}
			mockNode.Annotations[AnnotationNs+"/extended-resources"] = "feature-4,feature-2"
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-4")] = *resource.NewQuantity(4, resource.BinarySI)
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
		Convey("When the extended resource is no longer wanted", func() {
			mockNode := newMockNode()
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			mockNode.Status.Capacity[api.ResourceName(LabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			mockResourceLabels := ExtendedResources{LabelNs + "/feature-2": "2"}
			mockNode.Annotations[AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			patches := createExtendedResourcePatches(mockNode, mockResourceLabels)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
	})
}

func TestSetLabels(t *testing.T) {
	Convey("When servicing SetLabels request", t, func() {
		const workerName = "mock-worker"
		const workerVer = "0.1-test"
		mockHelper := &apihelper.MockAPIHelpers{}
		mockClient := &k8sclient.Clientset{}
		mockNode := newMockNode()
		mockServer := labelerServer{args: Args{LabelWhiteList: regexp.MustCompile("")}, apiHelper: mockHelper}
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
				apihelper.NewJsonPatch("add", "/metadata/annotations", workerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", featureLabelAnnotation, strings.Join(mockLabelNames, ",")),
				apihelper.NewJsonPatch("add", "/metadata/annotations", extendedResourceAnnotation, ""),
			}
			for k, v := range mockLabels {
				expectedPatches = append(expectedPatches, apihelper.NewJsonPatch("add", "/metadata/labels", LabelNs+"/"+k, v))
			}

			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When --label-whitelist is specified", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", workerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", featureLabelAnnotation, "feature-2"),
				apihelper.NewJsonPatch("add", "/metadata/annotations", extendedResourceAnnotation, ""),
				apihelper.NewJsonPatch("add", "/metadata/labels", LabelNs+"/feature-2", mockLabels["feature-2"]),
			}

			mockServer.args.LabelWhiteList = regexp.MustCompile("^f.*2$")
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When --extra-label-ns is specified", func() {
			// In the gRPC request the label names may omit the default ns
			mockLabels := map[string]string{"feature-1": "val-1",
				"valid.ns/feature-2":   "val-2",
				"invalid.ns/feature-3": "val-3"}
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", workerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", featureLabelAnnotation, "feature-1,valid.ns/feature-2"),
				apihelper.NewJsonPatch("add", "/metadata/annotations", extendedResourceAnnotation, ""),
				apihelper.NewJsonPatch("add", "/metadata/labels", LabelNs+"/feature-1", mockLabels["feature-1"]),
				apihelper.NewJsonPatch("add", "/metadata/labels", "valid.ns/feature-2", mockLabels["valid.ns/feature-2"]),
			}

			mockServer.args.ExtraLabelNs = map[string]struct{}{"valid.ns": struct{}{}}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: mockLabels}
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})

		})

		Convey("When --resource-labels is specified", func() {
			expectedPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/metadata/annotations", workerVersionAnnotation, workerVer),
				apihelper.NewJsonPatch("add", "/metadata/annotations", featureLabelAnnotation, "feature-2"),
				apihelper.NewJsonPatch("add", "/metadata/annotations", extendedResourceAnnotation, "feature-1,feature-3"),
				apihelper.NewJsonPatch("add", "/metadata/labels", LabelNs+"/feature-2", mockLabels["feature-2"]),
			}
			expectedStatusPatches := []apihelper.JsonPatch{
				apihelper.NewJsonPatch("add", "/status/capacity", LabelNs+"/feature-1", mockLabels["feature-1"]),
				apihelper.NewJsonPatch("add", "/status/capacity", LabelNs+"/feature-3", mockLabels["feature-3"]),
			}

			mockServer.args.ResourceLabels = []string{"feature-3", "feature-1"}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("PatchNode", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedPatches))).Return(nil)
			mockHelper.On("PatchNodeStatus", mockClient, mockNodeName, mock.MatchedBy(jsonPatchMatcher(expectedStatusPatches))).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		mockErr := errors.New("mock-error")
		Convey("When node update fails", func() {
			mockHelper.On("GetClient").Return(mockClient, mockErr)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})

		mockServer.args.NoPublish = true
		Convey("With '--no-publish'", func() {
			_, err := mockServer.SetLabels(mockCtx, mockReq)
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
		n := &api.Node{
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

func jsonPatchMatcher(expected []apihelper.JsonPatch) func([]apihelper.JsonPatch) bool {
	return func(actual []apihelper.JsonPatch) bool {
		// We don't care about modifying the original slices
		ok, msg := assertions.So(sortJsonPatches(actual), ShouldResemble, sortJsonPatches(expected))
		if !ok {
			// We parse the cryptic string message for better readability
			var f assertions.FailureView
			if err := yaml.Unmarshal([]byte(msg), &f); err == nil {
				Printf("%s\n", f.Message)
			} else {
				Printf("%s\n", msg)
			}
		}
		return ok
	}
}

func sortJsonPatches(p []apihelper.JsonPatch) []apihelper.JsonPatch {
	sort.Slice(p, func(i, j int) bool { return p[i].Path < p[j].Path })
	return p
}
