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

	. "github.com/smartystreets/goconvey/convey"
	"github.com/vektra/errors"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	mockNodeName = "mock-node"
)

func init() {
	nodeName = mockNodeName
}

func newMockNode() *api.Node {
	n := api.Node{}
	n.Labels = map[string]string{}
	n.Annotations = map[string]string{}
	return &n
}

func TestUpdateNodeFeatures(t *testing.T) {
	Convey("When I update the node using fake client", t, func() {
		fakeFeatureLabels := map[string]string{"source-feature.1": "val1", "source-feature.2": "val2", "source-feature.3": "val3"}
		fakeAnnotations := map[string]string{"version": version.Get()}
		fakeFeatureLabelNames := make([]string, 0, len(fakeFeatureLabels))
		for k, _ := range fakeFeatureLabels {
			fakeFeatureLabelNames = append(fakeFeatureLabelNames, k)
		}
		sort.Strings(fakeFeatureLabelNames)
		fakeAnnotations["feature-labels"] = strings.Join(fakeFeatureLabelNames, ",")

		mockAPIHelper := new(apihelper.MockAPIHelpers)
		mockClient := &k8sclient.Clientset{}
		// Mock node with old features
		mockNode := newMockNode()
		mockNode.Labels[labelNs+"old-feature"] = "old-value"
		mockNode.Annotations[annotationNs+"feature-labels"] = "old-feature"

		Convey("When I successfully update the node with feature labels", func() {
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Once()
			mockAPIHelper.On("UpdateNode", mockClient, mockNode).Return(nil).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Node object should have updated with labels and annotations", func() {
				So(len(mockNode.Labels), ShouldEqual, len(fakeFeatureLabels))
				for k, v := range fakeFeatureLabels {
					So(mockNode.Labels[labelNs+k], ShouldEqual, v)
				}
				So(len(mockNode.Annotations), ShouldEqual, len(fakeAnnotations))
				for k, v := range fakeAnnotations {
					So(mockNode.Annotations[annotationNs+k], ShouldEqual, v)
				}
			})
		})

		Convey("When I fail to update the node with feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to get a mock client while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to get a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(nil, expectedError).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to update a mock node while updating feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Once()
			mockAPIHelper.On("UpdateNode", mockClient, mockNode).Return(expectedError).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

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
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(nil)
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
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(mockErr)
			err := updateMasterNode(mockHelper)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
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
		mockLabels := map[string]string{"feature-1": "val-1", "feature-2": "val-2", "feature-3": "val-3"}
		mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: mockLabels}

		mockLabelNames := make([]string, 0, len(mockLabels))
		for k := range mockLabels {
			mockLabelNames = append(mockLabelNames, k)
		}
		sort.Strings(mockLabelNames)
		expectedAnnotations := map[string]string{"worker.version": workerVer}
		expectedAnnotations["feature-labels"] = strings.Join(mockLabelNames, ",")

		Convey("When node update succeeds", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})
			Convey("Node object should have updated with labels and annotations", func() {
				So(len(mockNode.Labels), ShouldEqual, len(mockLabels))
				for k, v := range mockLabels {
					So(mockNode.Labels[labelNs+k], ShouldEqual, v)
				}
				So(len(mockNode.Annotations), ShouldEqual, len(expectedAnnotations))
				for k, v := range expectedAnnotations {
					So(mockNode.Annotations[annotationNs+k], ShouldEqual, v)
				}
			})
		})

		Convey("When --label-whitelist is specified", func() {
			mockServer.args.LabelWhiteList = regexp.MustCompile("^f.*2$")
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Node object should only have whitelisted labels", func() {
				So(len(mockNode.Labels), ShouldEqual, 1)
				So(mockNode.Labels, ShouldResemble, map[string]string{labelNs + "feature-2": "val-2"})

				a := map[string]string{annotationNs + "worker.version": workerVer, annotationNs + "feature-labels": "feature-2"}
				So(len(mockNode.Annotations), ShouldEqual, len(a))
				So(mockNode.Annotations, ShouldResemble, a)
			})
		})

		Convey("When --extra-label-ns is specified", func() {
			mockServer.args.ExtraLabelNs = []string{"valid.ns"}
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(nil)
			mockLabels := map[string]string{"feature-1": "val-1",
				"valid.ns/feature-2":   "val-2",
				"invalid.ns/feature-3": "val-3"}
			mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: mockLabels}
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Node object should only have allowed label namespaces", func() {
				So(len(mockNode.Labels), ShouldEqual, 2)
				So(mockNode.Labels, ShouldResemble, map[string]string{labelNs + "feature-1": "val-1", "valid.ns/feature-2": "val-2"})

				a := map[string]string{annotationNs + "worker.version": workerVer, annotationNs + "feature-labels": "feature-1,valid.ns/feature-2"}
				So(len(mockNode.Annotations), ShouldEqual, len(a))
				So(mockNode.Annotations, ShouldResemble, a)
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

func TestAddLabels(t *testing.T) {
	Convey("When adding labels", t, func() {
		labels := map[string]string{}
		n := &api.Node{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: map[string]string{},
			},
		}

		Convey("If no labels are passed", func() {
			addLabels(n, labels)

			Convey("None should be added", func() {
				So(len(n.Labels), ShouldEqual, 0)
			})
		})

		Convey("They should be added to the node.Labels", func() {
			test1 := "test1"
			labels[test1] = "true"
			addLabels(n, labels)
			So(n.Labels, ShouldContainKey, labelNs+test1)
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
			removeLabelsWithPrefix(n, "single")
			So(len(n.Labels), ShouldEqual, 2)
			So(n.Labels, ShouldNotContainKey, "single")
		})

		Convey("a non-unique search string should remove all matching keys", func() {
			removeLabelsWithPrefix(n, "multiple")
			So(len(n.Labels), ShouldEqual, 1)
			So(n.Labels, ShouldNotContainKey, "multiple_A")
			So(n.Labels, ShouldNotContainKey, "multiple_B")
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
