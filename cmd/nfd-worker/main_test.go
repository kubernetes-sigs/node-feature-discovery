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
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/fake"
	"sigs.k8s.io/node-feature-discovery/source/panic_fake"
)

func TestDiscoveryWithMockSources(t *testing.T) {
	Convey("When I discover features from fake source and update the node using fake client", t, func() {
		mockFeatureSource := new(source.MockFeatureSource)
		fakeFeatureSourceName := string("testSource")
		fakeFeatureNames := []string{"testfeature1", "testfeature2", "testfeature3"}
		fakeFeatures := source.Features{}
		fakeFeatureLabels := Labels{}
		fakeFeatureLabelNames := make([]string, 0, len(fakeFeatureNames))
		for _, f := range fakeFeatureNames {
			fakeFeatures[f] = true
			labelName := fakeFeatureSourceName + "-" + f
			fakeFeatureLabels[labelName] = "true"
			fakeFeatureLabelNames = append(fakeFeatureLabelNames, labelName)
		}
		fakeFeatureSource := source.FeatureSource(mockFeatureSource)

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
	})
}

func TestArgsParse(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		Convey("When --no-publish and --oneshot flags are passed", func() {
			args, err := argsParse([]string{"--no-publish", "--oneshot"})

			Convey("noPublish is set and args.sources is set to the default value", func() {
				So(args.sleepInterval, ShouldEqual, 60*time.Second)
				So(args.noPublish, ShouldBeTrue)
				So(args.oneshot, ShouldBeTrue)
				So(args.sources, ShouldResemble, []string{"cpu", "cpuid", "iommu", "kernel", "local", "memory", "network", "pci", "pstate", "rdt", "storage", "system"})
				So(len(args.labelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When --sources flag is passed and set to some values, --sleep-inteval is specified", func() {
			args, err := argsParse([]string{"--sources=fake1,fake2,fake3", "--sleep-interval=30s"})

			Convey("args.sources is set to appropriate values", func() {
				So(args.sleepInterval, ShouldEqual, 30*time.Second)
				So(args.noPublish, ShouldBeFalse)
				So(args.oneshot, ShouldBeFalse)
				So(args.sources, ShouldResemble, []string{"fake1", "fake2", "fake3"})
				So(len(args.labelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When --label-whitelist flag is passed and set to some value", func() {
			args, err := argsParse([]string{"--label-whitelist=.*rdt.*"})

			Convey("args.labelWhiteList is set to appropriate value and args.sources is set to default value", func() {
				So(args.noPublish, ShouldBeFalse)
				So(args.sources, ShouldResemble, []string{"cpu", "cpuid", "iommu", "kernel", "local", "memory", "network", "pci", "pstate", "rdt", "storage", "system"})
				So(args.labelWhiteList, ShouldResemble, ".*rdt.*")
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified", func() {
			args, err := argsParse([]string{"--no-publish", "--sources=fake1,fake2,fake3", "--ca-file=ca", "--cert-file=crt", "--key-file=key"})

			Convey("--no-publish is set and args.sources is set to appropriate values", func() {
				So(args.noPublish, ShouldBeTrue)
				So(args.caFile, ShouldEqual, "ca")
				So(args.certFile, ShouldEqual, "crt")
				So(args.keyFile, ShouldEqual, "key")
				So(args.sources, ShouldResemble, []string{"fake1", "fake2", "fake3"})
				So(len(args.labelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When one of --cert-file, --key-file or --ca-file is missing", func() {
			_, err := argsParse([]string{"--cert-file=crt", "--key-file=key"})
			_, err2 := argsParse([]string{"--key-file=key", "--ca-file=ca"})
			_, err3 := argsParse([]string{"--cert-file=crt", "--ca-file=ca"})
			Convey("Argument parsing should fail", func() {
				So(err, ShouldNotBeNil)
				So(err2, ShouldNotBeNil)
				So(err3, ShouldNotBeNil)
			})
		})
	})
}

func TestConfigParse(t *testing.T) {
	Convey("When parsing configuration file", t, func() {
		Convey("When non-accessible file is given", func() {
			err := configParse("non-existing-file", "")

			Convey("Should return error", func() {
				So(err, ShouldNotBeNil)
			})
		})
		// Create a temporary config file
		f, err := ioutil.TempFile("", "nfd-test-")
		defer os.Remove(f.Name())
		So(err, ShouldBeNil)
		f.WriteString(`sources:
  kernel:
    configOpts:
      - "DMI"
  pci:
    deviceClassWhitelist:
      - "ff"`)
		f.Close()

		Convey("When proper config file is given", func() {
			err := configParse(f.Name(), "")

			Convey("Should return error", func() {
				So(err, ShouldBeNil)
				So(config.Sources.Kernel.ConfigOpts, ShouldResemble, []string{"DMI"})
				So(config.Sources.Pci.DeviceClassWhitelist, ShouldResemble, []string{"ff"})
			})
		})
	})
}

func TestConfigureParameters(t *testing.T) {
	Convey("When configuring parameters for node feature discovery", t, func() {

		Convey("When no sourcesWhiteList and labelWhiteListStr are passed", func() {
			sourcesWhiteList := []string{}
			labelWhiteListStr := ""
			emptyRegexp, _ := regexp.Compile("")
			enabledSources, labelWhiteList, err := configureParameters(sourcesWhiteList, labelWhiteListStr)

			Convey("Error should not be produced", func() {
				So(err, ShouldBeNil)
			})
			Convey("No sourcesWhiteList or labelWhiteList are returned", func() {
				So(len(enabledSources), ShouldEqual, 0)
				So(labelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("When sourcesWhiteList is passed", func() {
			sourcesWhiteList := []string{"fake"}
			labelWhiteListStr := ""
			emptyRegexp, _ := regexp.Compile("")
			enabledSources, labelWhiteList, err := configureParameters(sourcesWhiteList, labelWhiteListStr)

			Convey("Error should not be produced", func() {
				So(err, ShouldBeNil)
			})
			Convey("Proper sourcesWhiteList are returned", func() {
				So(len(enabledSources), ShouldEqual, 1)
				So(enabledSources[0], ShouldHaveSameTypeAs, fake.Source{})
				So(labelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("When invalid labelWhiteListStr is passed", func() {
			sourcesWhiteList := []string{""}
			labelWhiteListStr := "*"
			enabledSources, labelWhiteList, err := configureParameters(sourcesWhiteList, labelWhiteListStr)

			Convey("Error is produced", func() {
				So(enabledSources, ShouldBeNil)
				So(labelWhiteList, ShouldBeNil)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When valid labelWhiteListStr is passed", func() {
			sourcesWhiteList := []string{""}
			labelWhiteListStr := ".*rdt.*"
			expectRegexp, err := regexp.Compile(".*rdt.*")
			enabledSources, labelWhiteList, err := configureParameters(sourcesWhiteList, labelWhiteListStr)

			Convey("Error should not be produced", func() {
				So(err, ShouldBeNil)
			})
			Convey("Proper labelWhiteList is returned", func() {
				So(len(enabledSources), ShouldEqual, 0)
				So(labelWhiteList, ShouldResemble, expectRegexp)
			})
		})
	})
}

func TestCreateFeatureLabels(t *testing.T) {
	Convey("When creating feature labels from the configured sources", t, func() {
		Convey("When fake feature source is configured", func() {
			emptyLabelWL, _ := regexp.Compile("")
			fakeFeatureSource := source.FeatureSource(new(fake.Source))
			sources := []source.FeatureSource{}
			sources = append(sources, fakeFeatureSource)
			labels := createFeatureLabels(sources, emptyLabelWL)

			Convey("Proper fake labels are returned", func() {
				So(len(labels), ShouldEqual, 3)
				So(labels, ShouldContainKey, "fake-fakefeature1")
				So(labels, ShouldContainKey, "fake-fakefeature2")
				So(labels, ShouldContainKey, "fake-fakefeature3")
			})
		})
		Convey("When fake feature source is configured with a whitelist that doesn't match", func() {
			emptyLabelWL, _ := regexp.Compile(".*rdt.*")
			fakeFeatureSource := source.FeatureSource(new(fake.Source))
			sources := []source.FeatureSource{}
			sources = append(sources, fakeFeatureSource)
			labels := createFeatureLabels(sources, emptyLabelWL)

			Convey("fake labels are not returned", func() {
				So(len(labels), ShouldEqual, 0)
				So(labels, ShouldNotContainKey, "fake-fakefeature1")
				So(labels, ShouldNotContainKey, "fake-fakefeature2")
				So(labels, ShouldNotContainKey, "fake-fakefeature3")
			})
		})
	})
}

func TestGetFeatureLabels(t *testing.T) {
	Convey("When I get feature labels and panic occurs during discovery of a feature source", t, func() {
		fakePanicFeatureSource := source.FeatureSource(new(panic_fake.Source))

		returnedLabels, err := getFeatureLabels(fakePanicFeatureSource)
		Convey("No label is returned", func() {
			So(len(returnedLabels), ShouldEqual, 0)
		})
		Convey("Error is produced and panic error is returned", func() {
			So(err, ShouldResemble, fmt.Errorf("fake panic error"))
		})

	})
}

func TestAdvertiseFeatureLabels(t *testing.T) {
	Convey("When advertising labels", t, func() {
		mockClient := &labeler.MockLabelerClient{}
		labels := map[string]string{"feature-1": "value-1"}

		Convey("Correct labeling request is sent", func() {
			mockClient.On("SetLabels", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*labeler.SetLabelsRequest")).Return(&labeler.SetLabelsReply{}, nil)
			err := advertiseFeatureLabels(mockClient, labels)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
		})
		Convey("Labeling request fails", func() {
			mockErr := errors.New("mock-error")
			mockClient.On("SetLabels", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("*labeler.SetLabelsRequest")).Return(&labeler.SetLabelsReply{}, mockErr)
			err := advertiseFeatureLabels(mockClient, labels)
			Convey("An error should be returned", func() {
				So(err, ShouldEqual, mockErr)
			})
		})
	})
}
