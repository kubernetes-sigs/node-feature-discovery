package main

import (
	"fmt"
	"testing"

	// "github.com/intelsdi-x/dbi-iafeature-discovery/mocks"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/vektra/errors"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

func TestDiscoveryWithMockSources(t *testing.T) {
	Convey("When I discover features from fake source and update the node using fake client", t, func() {
		mockFeatureSource := new(MockFeatureSource)
		fakeFeatureSourceName := string("testSource")
		fakeFeatures := []string{"testfeature1", "testfeature2", "testfeature3"}
		fakeFeatureLabels := Labels{}
		for _, f := range fakeFeatures {
			fakeFeatureLabels[fmt.Sprintf("%s-testSource-%s", prefix, f)] = "true"
		}
		fakeFeatureSource := FeatureSource(mockFeatureSource)

		Convey("When I successfully get the labels from the mock source", func() {
			mockFeatureSource.On("Name").Return(fakeFeatureSourceName)
			mockFeatureSource.On("Discover").Return(fakeFeatures, nil)

			returnedLabels, err := getFeatureLabels(fakeFeatureSource)
			Convey("Proper label is returned", func() {
				So(returnedLabels, ShouldResemble, fakeFeatureLabels)
			})
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When I fail to get the labels from the mock source", func() {
			expectedError := errors.New("fake error")
			mockFeatureSource.On("Discover").Return(nil, expectedError)

			returnedLabels, err := getFeatureLabels(fakeFeatureSource)
			Convey("No label is returned", func() {
				So(returnedLabels, ShouldBeNil)
			})
			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		mockAPIHelper := new(MockAPIHelpers)
		testHelper := APIHelpers(mockAPIHelper)
		var mockClient *client.Client
		var mockNode *api.Node

		Convey("When I successfully advertise feature labels to a node", func() {
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient).Return(mockNode, nil).Once()
			mockAPIHelper.On("AddLabels", mockNode, fakeFeatureLabels).Return().Once()
			mockAPIHelper.On("UpdateNode", mockClient, mockNode).Return(nil).Once()
			err := advertiseFeatureLabels(testHelper, fakeFeatureLabels)

			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When I fail to get a mock client while advertising feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(nil, expectedError)
			err := advertiseFeatureLabels(testHelper, fakeFeatureLabels)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to get a mock node while advertising feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient).Return(nil, expectedError).Once()
			err := advertiseFeatureLabels(testHelper, fakeFeatureLabels)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

		Convey("When I fail to update a mock node while advertising feature labels", func() {
			expectedError := errors.New("fake error")
			mockAPIHelper.On("GetClient").Return(mockClient, nil)
			mockAPIHelper.On("GetNode", mockClient).Return(mockNode, nil).Once()
			mockAPIHelper.On("AddLabels", mockNode, fakeFeatureLabels).Return().Once()
			mockAPIHelper.On("UpdateNode", mockClient, mockNode).Return(expectedError).Once()
			err := advertiseFeatureLabels(testHelper, fakeFeatureLabels)

			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})

	})
}
