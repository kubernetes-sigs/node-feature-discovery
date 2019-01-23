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

package main

import (
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

const (
	mockNodeName = "mock-node"
)

func init() {
	os.Setenv(NodeNameEnv, mockNodeName)
}

func TestUpdateNodeFeatures(t *testing.T) {
	Convey("When I update the node using fake client", t, func() {
		fakeFeatureLabels := map[string]string{"source-feature.1": "val1", "source-feature.2": "val2", "source-feature.3": "val3"}
		fakeAnnotations := map[string]string{"version": version.Get()}
		fakeFeatureLabelNames := make([]string, 0, len(fakeFeatureLabels))
		for k, _ := range fakeFeatureLabels {
			fakeFeatureLabelNames = append(fakeFeatureLabelNames, k)
		}
		fakeAnnotations["feature-labels"] = strings.Join(fakeFeatureLabelNames, ",")

		mockAPIHelper := new(apihelper.MockAPIHelpers)
		mockNode := &api.Node{}
		mockClient := &k8sclient.Clientset{}

		Convey("When I successfully update the node with feature labels", func() {
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil).Once()
			mockAPIHelper.On("AddLabels", mockNode, fakeFeatureLabels).Return().Once()
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, labelNs).Return().Once()
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, "node.alpha.kubernetes-incubator.io/nfd").Return().Once()
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, "node.alpha.kubernetes-incubator.io/node-feature-discovery").Return().Once()
			mockAPIHelper.On("AddAnnotations", mockNode, fakeAnnotations).Return().Once()
			mockAPIHelper.On("UpdateNode", mockClient, mockNode).Return(nil).Once()
			err := updateNodeFeatures(mockAPIHelper, mockNodeName, fakeFeatureLabels, fakeAnnotations)

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
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
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, labelNs).Return().Once()
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, "node.alpha.kubernetes-incubator.io/nfd").Return().Once()
			mockAPIHelper.On("RemoveLabelsWithPrefix", mockNode, "node.alpha.kubernetes-incubator.io/node-feature-discovery").Return().Once()
			mockAPIHelper.On("AddLabels", mockNode, fakeFeatureLabels).Return().Once()
			mockAPIHelper.On("AddAnnotations", mockNode, fakeAnnotations).Return().Once()
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
		mockNode := &api.Node{}
		Convey("When update operation succeeds", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, mockNodeName).Return(mockNode, nil)
			mockHelper.On("AddAnnotations", mockNode, map[string]string{"master.version": version.Get()})
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
			mockHelper.On("AddAnnotations", mock.Anything, mock.Anything)
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
		mockNode := &api.Node{}
		mockServer := labelerServer{args: Args{}, apiHelper: mockHelper}
		mockCtx := context.Background()
		mockReq := &labeler.SetLabelsRequest{NodeName: workerName, NfdVersion: workerVer, Labels: map[string]string{"feature-1": "val-1"}}

		Convey("When node update succeeds", func() {
			mockHelper.On("GetClient").Return(mockClient, nil)
			mockHelper.On("GetNode", mockClient, workerName).Return(mockNode, nil)
			mockHelper.On("RemoveLabelsWithPrefix", mockNode, mock.Anything).Return()
			mockHelper.On("AddLabels", mockNode, mock.Anything).Return()
			mockHelper.On("AddAnnotations", mockNode, map[string]string{"worker.version": workerVer, "feature-labels": "feature-1"})
			mockHelper.On("UpdateNode", mockClient, mockNode).Return(nil)
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("No error should be returned", func() {
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

		mockServer.args.noPublish = true
		Convey("With '--no-publish'", func() {
			_, err := mockServer.SetLabels(mockCtx, mockReq)
			Convey("Operation should succeed", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestArgsParse(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		Convey("When --no-publish and --oneshot flags are passed", func() {
			args, err := argsParse([]string{"--no-publish"})
			Convey("noPublish is set and args.sources is set to the default value", func() {
				So(args.noPublish, ShouldBeTrue)
				So(len(args.labelWhiteList.String()), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified", func() {
			args, err := argsParse([]string{"--label-whitelist=.*rdt.*", "--port=1234"})
			Convey("Argument parsing should succeed and args set to correct values", func() {
				So(args.noPublish, ShouldBeFalse)
				So(args.port, ShouldEqual, 1234)
				So(args.labelWhiteList.String(), ShouldResemble, ".*rdt.*")
				So(err, ShouldBeNil)
			})
		})
		Convey("When invalid --port is defined", func() {
			_, err := argsParse([]string{"--port=123a"})
			Convey("argsParse should fail", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}
