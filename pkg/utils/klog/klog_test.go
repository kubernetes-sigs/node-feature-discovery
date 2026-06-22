/*
Copyright 2023 The Kubernetes Authors.

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

package klog

import (
	"flag"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestKlogConfigOptName(t *testing.T) {
	Convey("When converting names of klog command line flags", t, func() {
		tcs := map[string]string{
			"":                    "",
			"a":                   "a",
			"an_arg":              "anArg",
			"arg_with_many_parts": "argWithManyParts",
		}
		Convey("resulting config option names should be as expected", func() {
			for input, expected := range tcs {
				So(klogConfigOptName(input), ShouldEqual, expected)
			}
		})
	})
}

func TestApplyStderrThresholdDefaults(t *testing.T) {
	Convey("When applyStderrThresholdDefaults is called", t, func() {
		// Use a local flagset to avoid mutating klog's global config.
		flagset := flag.NewFlagSet("test-defaults", flag.ContinueOnError)
		flagset.String("legacy_stderr_threshold_behavior", "true", "")
		flagset.String("stderrthreshold", "2", "")

		err := applyStderrThresholdDefaults(flagset)

		Convey("it should not return an error", func() {
			So(err, ShouldBeNil)
		})

		Convey("legacy_stderr_threshold_behavior value and DefValue should be false", func() {
			f := flagset.Lookup("legacy_stderr_threshold_behavior")
			So(f, ShouldNotBeNil)
			So(f.DefValue, ShouldEqual, "false")
			So(f.Value.String(), ShouldEqual, "false")
		})

		Convey("stderrthreshold DefValue should be 0 (INFO)", func() {
			f := flagset.Lookup("stderrthreshold")
			So(f, ShouldNotBeNil)
			So(f.DefValue, ShouldEqual, "0")
			So(f.Value.String(), ShouldEqual, "INFO")
		})
	})

	Convey("When applyStderrThresholdDefaults is called on a flagset without the flags", t, func() {
		flagset := flag.NewFlagSet("test-missing-flags", flag.ContinueOnError)

		err := applyStderrThresholdDefaults(flagset)

		Convey("it should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestInitKlogFlags_MergePreservesDefaults(t *testing.T) {
	Convey("When InitKlogFlags is called and MergeKlogConfiguration runs with empty config", t, func() {
		flagset := flag.NewFlagSet("test-merge", flag.ContinueOnError)
		klogFlags := InitKlogFlags(flagset)

		Convey("legacy_stderr_threshold_behavior should be disabled", func() {
			f := flagset.Lookup("legacy_stderr_threshold_behavior")
			So(f, ShouldNotBeNil)
			So(f.Value.String(), ShouldEqual, "false")
		})

		Convey("stderrthreshold should be INFO (0)", func() {
			f := flagset.Lookup("stderrthreshold")
			So(f, ShouldNotBeNil)
			// klog represents severity as numeric level: 0=INFO
			So(f.Value.String(), ShouldEqual, "0")
		})

		Convey("the flags should NOT be marked as set from command line", func() {
			kfv, ok := klogFlags["legacyStderrThresholdBehavior"]
			So(ok, ShouldBeTrue)
			// The DefValue approach does not set isSetFromCmdLine,
			// allowing config-file overrides to still take effect.
			So(kfv.IsSetFromCmdline(), ShouldBeFalse)

			kfv2, ok := klogFlags["stderrthreshold"]
			So(ok, ShouldBeTrue)
			So(kfv2.IsSetFromCmdline(), ShouldBeFalse)
		})

		Convey("MergeKlogConfiguration with empty config should preserve the defaults", func() {
			err := MergeKlogConfiguration(klogFlags, KlogConfigOpts{})
			So(err, ShouldBeNil)
			f := flagset.Lookup("legacy_stderr_threshold_behavior")
			So(f.Value.String(), ShouldEqual, "false")
			f2 := flagset.Lookup("stderrthreshold")
			// klog represents severity as numeric level: 0=INFO
			So(f2.Value.String(), ShouldEqual, "0")
		})

		Convey("MergeKlogConfiguration should allow config-file override of legacyStderrThresholdBehavior", func() {
			err := MergeKlogConfiguration(klogFlags, KlogConfigOpts{
				"legacyStderrThresholdBehavior": "true",
			})
			So(err, ShouldBeNil)
			f := flagset.Lookup("legacy_stderr_threshold_behavior")
			So(f.Value.String(), ShouldEqual, "true")
		})

		Convey("MergeKlogConfiguration should allow config-file override of stderrthreshold", func() {
			err := MergeKlogConfiguration(klogFlags, KlogConfigOpts{
				"stderrthreshold": "WARNING",
			})
			So(err, ShouldBeNil)
			f := flagset.Lookup("stderrthreshold")
			// klog represents severity as numeric level: 1=WARNING
			So(f.Value.String(), ShouldEqual, "1")
		})
	})
}
