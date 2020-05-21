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

package nfdworker

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"
	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/cpu"
	"sigs.k8s.io/node-feature-discovery/source/fake"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
	"sigs.k8s.io/node-feature-discovery/source/panic_fake"
	"sigs.k8s.io/node-feature-discovery/source/pci"
)

const fakeFeatureSourceName string = "testSource"

func TestDiscoveryWithMockSources(t *testing.T) {
	Convey("When I discover features from fake source and update the node using fake client", t, func() {
		mockFeatureSource := new(source.MockFeatureSource)
		allFeatureNames := []string{"testfeature1", "testfeature2", "test.ns/test", "test.ns/foo", "/no-ns-label", "invalid/test/feature"}
		whiteListFeatureNames := []string{"testfeature1", "testfeature2", "test.ns/test"}

		fakeFeatures, _ := makeFakeFeatures(allFeatureNames)
		_, fakeFeatureLabels := makeFakeFeatures(whiteListFeatureNames)

		fakeFeatureSource := source.FeatureSource(mockFeatureSource)

		labelWhiteList := regexp.MustCompile("^test")

		Convey("When I successfully get the labels from the mock source", func() {
			mockFeatureSource.On("Name").Return(fakeFeatureSourceName)
			mockFeatureSource.On("Discover").Return(fakeFeatures, nil)

			returnedLabels, err := getFeatureLabels(fakeFeatureSource, labelWhiteList)
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

			returnedLabels, err := getFeatureLabels(fakeFeatureSource, labelWhiteList)
			Convey("No label is returned", func() {
				So(returnedLabels, ShouldBeNil)
			})
			Convey("Error is produced", func() {
				So(err, ShouldEqual, expectedError)
			})
		})
	})
}

func makeFakeFeatures(names []string) (source.Features, Labels) {
	features := source.Features{}
	labels := Labels{}
	for _, f := range names {
		features[f] = true
		labelName := fakeFeatureSourceName + "-" + f
		if strings.IndexByte(f, '/') >= 0 {
			labelName = f
		}
		labels[labelName] = "true"
	}

	return features, labels
}

func (w *nfdWorker) getSource(name string) source.FeatureSource {
	for _, s := range w.sources {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func TestConfigParse(t *testing.T) {
	Convey("When parsing configuration", t, func() {
		w, err := NewNfdWorker(Args{Sources: []string{"cpu", "kernel", "pci"}})
		So(err, ShouldBeNil)
		worker := w.(*nfdWorker)
		Convey("and a non-accessible file and some overrides are specified", func() {
			overrides := `{"sources": {"cpu": {"cpuid": {"attributeBlacklist": ["foo","bar"]}}}}`
			worker.configure("non-existing-file", overrides)

			Convey("overrides should take effect", func() {
				c := worker.getSource("cpu").GetConfig().(*cpu.Config)
				So(c.Cpuid.AttributeBlacklist, ShouldResemble, []string{"foo", "bar"})
			})
		})
		// Create a temporary config file
		f, err := ioutil.TempFile("", "nfd-test-")
		defer os.Remove(f.Name())
		So(err, ShouldBeNil)
		_, err = f.WriteString(`sources:
  kernel:
    configOpts:
      - "DMI"
  pci:
    deviceClassWhitelist:
      - "ff"`)
		f.Close()
		So(err, ShouldBeNil)

		Convey("and a proper config file is specified", func() {
			worker.configure(f.Name(), "")

			Convey("specified configuration should take effect", func() {
				So(err, ShouldBeNil)
				c := worker.getSource("kernel").GetConfig()
				So(c.(*kernel.Config).ConfigOpts, ShouldResemble, []string{"DMI"})
				c = worker.getSource("pci").GetConfig()
				So(c.(*pci.Config).DeviceClassWhitelist, ShouldResemble, []string{"ff"})
			})
		})

		Convey("and a proper config file and overrides are given", func() {
			overrides := `{"sources": {"pci": {"deviceClassWhitelist": ["03"]}}}`
			worker.configure(f.Name(), overrides)

			Convey("overrides should take precedence over the config file", func() {
				So(err, ShouldBeNil)
				c := worker.getSource("kernel").GetConfig()
				So(c.(*kernel.Config).ConfigOpts, ShouldResemble, []string{"DMI"})
				c = worker.getSource("pci").GetConfig()
				So(c.(*pci.Config).DeviceClassWhitelist, ShouldResemble, []string{"03"})
			})
		})
	})
}

func TestNewNfdWorker(t *testing.T) {
	Convey("When creating new NfdWorker instance", t, func() {

		Convey("without any args specified", func() {
			args := Args{}
			emptyRegexp, _ := regexp.Compile("")
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			Convey("no sources should be enabled and the whitelist regexp should be empty", func() {
				So(len(worker.sources), ShouldEqual, 0)
				So(worker.labelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("with non-empty Sources arg specified", func() {
			args := Args{Sources: []string{"fake"}}
			emptyRegexp, _ := regexp.Compile("")
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			Convey("proper sources should be enabled", func() {
				So(len(worker.sources), ShouldEqual, 1)
				So(worker.sources[0], ShouldHaveSameTypeAs, &fake.Source{})
				So(worker.labelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("with invalid LabelWhiteList arg specified", func() {
			args := Args{LabelWhiteList: "*"}
			w, err := NewNfdWorker(args)
			worker := w.(*nfdWorker)
			Convey("an error should be returned", func() {
				So(len(worker.sources), ShouldEqual, 0)
				So(worker.labelWhiteList, ShouldBeNil)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("with valid LabelWhiteListStr arg specified", func() {
			args := Args{LabelWhiteList: ".*rdt.*"}
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			expectRegexp := regexp.MustCompile(".*rdt.*")
			Convey("proper labelWhiteList regexp should be produced", func() {
				So(len(worker.sources), ShouldEqual, 0)
				So(worker.labelWhiteList, ShouldResemble, expectRegexp)
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
		fakePanicFeatureSource := source.FeatureSource(new(panicfake.Source))

		returnedLabels, err := getFeatureLabels(fakePanicFeatureSource, regexp.MustCompile(""))
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
