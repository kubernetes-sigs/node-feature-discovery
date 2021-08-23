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

package worker

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	"github.com/vektra/errors"

	"sigs.k8s.io/node-feature-discovery/pkg/labeler"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/source/cpu"
	"sigs.k8s.io/node-feature-discovery/source/fake"
	"sigs.k8s.io/node-feature-discovery/source/kernel"
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

		labelWhiteList := utils.RegexpVal{Regexp: *regexp.MustCompile("^test")}

		Convey("When I successfully get the labels from the mock source", func() {
			mockFeatureSource.On("Name").Return(fakeFeatureSourceName)
			mockFeatureSource.On("Discover").Return(fakeFeatures, nil)

			returnedLabels, err := getFeatureLabels(fakeFeatureSource, labelWhiteList.Regexp)
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

			returnedLabels, err := getFeatureLabels(fakeFeatureSource, labelWhiteList.Regexp)
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
	for _, s := range w.realSources {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func TestConfigParse(t *testing.T) {
	Convey("When parsing configuration", t, func() {
		w, err := NewNfdWorker(&Args{})
		So(err, ShouldBeNil)
		worker := w.(*nfdWorker)
		overrides := `{"core": {"sources": ["fake"],"noPublish": true},"sources": {"cpu": {"cpuid": {"attributeBlacklist": ["foo","bar"]}}}}`

		Convey("and no core cmdline flags have been specified", func() {
			So(worker.configure("non-existing-file", overrides), ShouldBeNil)

			Convey("core overrides should be in effect", func() {
				So(worker.config.Core.Sources, ShouldResemble, []string{"fake"})
				So(worker.config.Core.NoPublish, ShouldBeTrue)
			})
		})
		Convey("and a non-accessible file, but core cmdline flags and some overrides are specified", func() {
			worker.args = Args{Overrides: ConfigOverrideArgs{Sources: &utils.StringSliceVal{"cpu", "kernel", "pci"}}}
			So(worker.configure("non-existing-file", overrides), ShouldBeNil)

			Convey("core cmdline flags should be in effect instead overrides", func() {
				So(worker.config.Core.Sources, ShouldResemble, []string{"cpu", "kernel", "pci"})
			})
			Convey("overrides should take effect", func() {
				So(worker.config.Core.NoPublish, ShouldBeTrue)

				c := worker.getSource("cpu").GetConfig().(*cpu.Config)
				So(c.Cpuid.AttributeBlacklist, ShouldResemble, []string{"foo", "bar"})
			})
		})
		// Create a temporary config file
		f, err := ioutil.TempFile("", "nfd-test-")
		defer os.Remove(f.Name())
		So(err, ShouldBeNil)
		_, err = f.WriteString(`
core:
  noPublish: false
  sources: ["system"]
  labelWhiteList: "foo"
  sleepInterval: "10s"
sources:
  kernel:
    configOpts:
      - "DMI"
  pci:
    deviceClassWhitelist:
      - "ff"`)
		f.Close()
		So(err, ShouldBeNil)

		Convey("and a proper config file is specified", func() {
			worker.args = Args{Overrides: ConfigOverrideArgs{Sources: &utils.StringSliceVal{"cpu", "kernel", "pci"}}}
			So(worker.configure(f.Name(), ""), ShouldBeNil)

			Convey("specified configuration should take effect", func() {
				// Verify core config
				So(worker.config.Core.NoPublish, ShouldBeFalse)
				So(worker.config.Core.Sources, ShouldResemble, []string{"cpu", "kernel", "pci"}) // from cmdline
				So(worker.config.Core.LabelWhiteList.String(), ShouldEqual, "foo")
				So(worker.config.Core.SleepInterval.Duration, ShouldEqual, 10*time.Second)

				// Verify feature source config
				So(err, ShouldBeNil)
				c := worker.getSource("kernel").GetConfig()
				So(c.(*kernel.Config).ConfigOpts, ShouldResemble, []string{"DMI"})
				c = worker.getSource("pci").GetConfig()
				So(c.(*pci.Config).DeviceClassWhitelist, ShouldResemble, []string{"ff"})
			})
		})

		Convey("and a proper config file and overrides are given", func() {
			sleepIntervalArg := 15 * time.Second
			worker.args = Args{Overrides: ConfigOverrideArgs{SleepInterval: &sleepIntervalArg}}
			overrides := `{"core": {"sources": ["fake"],"noPublish": true},"sources": {"pci": {"deviceClassWhitelist": ["03"]}}}`
			So(worker.configure(f.Name(), overrides), ShouldBeNil)

			Convey("overrides should take precedence over the config file", func() {
				// Verify core config
				So(worker.config.Core.NoPublish, ShouldBeTrue)
				So(worker.config.Core.Sources, ShouldResemble, []string{"fake"}) // from overrides
				So(worker.config.Core.LabelWhiteList.String(), ShouldEqual, "foo")
				So(worker.config.Core.SleepInterval.Duration, ShouldEqual, 15*time.Second) // from cmdline

				// Verify feature source config
				So(err, ShouldBeNil)
				c := worker.getSource("kernel").GetConfig()
				So(c.(*kernel.Config).ConfigOpts, ShouldResemble, []string{"DMI"})
				c = worker.getSource("pci").GetConfig()
				So(c.(*pci.Config).DeviceClassWhitelist, ShouldResemble, []string{"03"})
			})
		})
	})
}

func TestDynamicConfig(t *testing.T) {
	Convey("When running nfd-worker", t, func() {
		tmpDir, err := ioutil.TempDir("", "*.nfd-test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmpDir)

		// Create (temporary) dir for config
		configDir := filepath.Join(tmpDir, "subdir-1", "subdir-2", "worker.conf")
		err = os.MkdirAll(configDir, 0755)
		So(err, ShouldBeNil)

		// Create config file
		configFile := filepath.Join(configDir, "worker.conf")

		writeConfig := func(data string) {
			f, err := os.Create(configFile)
			So(err, ShouldBeNil)
			_, err = f.WriteString(data)
			So(err, ShouldBeNil)
			err = f.Close()
			So(err, ShouldBeNil)

		}
		writeConfig(`
core:
  labelWhiteList: "fake"
`)

		noPublish := true
		w, err := NewNfdWorker(&Args{
			ConfigFile: configFile,
			Overrides: ConfigOverrideArgs{
				Sources:   &utils.StringSliceVal{"fake"},
				NoPublish: &noPublish},
		})
		So(err, ShouldBeNil)
		worker := w.(*nfdWorker)

		Convey("config file updates should take effect", func() {
			go func() { _ = w.Run() }()
			defer w.Stop()

			// Check initial config
			So(func() interface{} { return worker.config.Core.LabelWhiteList.String() },
				withTimeout, 2*time.Second, ShouldEqual, "fake")

			// Update config and verify the effect
			writeConfig(`
core:
  labelWhiteList: "foo"
`)
			So(func() interface{} { return worker.config.Core.LabelWhiteList.String() },
				withTimeout, 2*time.Second, ShouldEqual, "foo")

			// Removing config file should get back our defaults
			err = os.RemoveAll(tmpDir)
			So(err, ShouldBeNil)
			So(func() interface{} { return worker.config.Core.LabelWhiteList.String() },
				withTimeout, 2*time.Second, ShouldEqual, "")

			// Re-creating config dir and file should change the config
			err = os.MkdirAll(configDir, 0755)
			So(err, ShouldBeNil)
			writeConfig(`
core:
  labelWhiteList: "bar"
`)
			So(func() interface{} { return worker.config.Core.LabelWhiteList.String() },
				withTimeout, 2*time.Second, ShouldEqual, "bar")
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

func TestNewNfdWorker(t *testing.T) {
	Convey("When creating new NfdWorker instance", t, func() {

		emptyRegexp := utils.RegexpVal{Regexp: *regexp.MustCompile("")}

		Convey("without any args specified", func() {
			args := &Args{}
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			So(worker.configure("", ""), ShouldBeNil)
			Convey("all sources should be enabled and the whitelist regexp should be empty", func() {
				So(len(worker.enabledSources), ShouldEqual, len(worker.realSources))
				So(worker.config.Core.LabelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("with non-empty Sources arg specified", func() {
			args := &Args{Overrides: ConfigOverrideArgs{Sources: &utils.StringSliceVal{"fake"}}}
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			So(worker.configure("", ""), ShouldBeNil)
			Convey("proper sources should be enabled", func() {
				So(len(worker.enabledSources), ShouldEqual, 1)
				So(worker.enabledSources[0], ShouldHaveSameTypeAs, &fake.Source{})
				So(worker.config.Core.LabelWhiteList, ShouldResemble, emptyRegexp)
			})
		})

		Convey("with valid LabelWhiteListStr arg specified", func() {
			args := &Args{Overrides: ConfigOverrideArgs{LabelWhiteList: &utils.RegexpVal{Regexp: *regexp.MustCompile(".*rdt.*")}}}
			w, err := NewNfdWorker(args)
			Convey("no error should be returned", func() {
				So(err, ShouldBeNil)
			})
			worker := w.(*nfdWorker)
			So(worker.configure("", ""), ShouldBeNil)
			expectRegexp := utils.RegexpVal{Regexp: *regexp.MustCompile(".*rdt.*")}
			Convey("proper labelWhiteList regexp should be produced", func() {
				So(worker.config.Core.LabelWhiteList, ShouldResemble, expectRegexp)
			})
		})
	})
}

func TestCreateFeatureLabels(t *testing.T) {
	Convey("When creating feature labels from the configured sources", t, func() {
		fakeFeatureSource := source.FeatureSource(new(fake.Source))
		fakeFeatureSource.SetConfig(fakeFeatureSource.NewConfig())
		sources := []source.FeatureSource{fakeFeatureSource}

		Convey("When fake feature source is configured", func() {
			emptyLabelWL := regexp.MustCompile("")
			labels := createFeatureLabels(sources, *emptyLabelWL)

			Convey("Proper fake labels are returned", func() {
				So(len(labels), ShouldEqual, 3)
				So(labels, ShouldContainKey, "fake-fakefeature1")
				So(labels, ShouldContainKey, "fake-fakefeature2")
				So(labels, ShouldContainKey, "fake-fakefeature3")
			})
		})
		Convey("When fake feature source is configured with a whitelist that doesn't match", func() {
			labels := createFeatureLabels(sources, *regexp.MustCompile(".*rdt.*"))

			Convey("fake labels are not returned", func() {
				So(len(labels), ShouldEqual, 0)
				So(labels, ShouldNotContainKey, "fake-fakefeature1")
				So(labels, ShouldNotContainKey, "fake-fakefeature2")
				So(labels, ShouldNotContainKey, "fake-fakefeature3")
			})
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
