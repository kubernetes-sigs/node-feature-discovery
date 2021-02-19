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

package main

import (
	"flag"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

func TestParseArgs(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		flags := flag.NewFlagSet(ProgramName, flag.ExitOnError)

		Convey("When no override args are specified", func() {
			args := parseArgs(flags, "--oneshot")

			Convey("overrides should be nil", func() {
				So(args.Oneshot, ShouldBeTrue)
				So(args.Overrides.NoPublish, ShouldBeNil)
				So(args.Overrides.LabelWhiteList, ShouldBeNil)
				So(args.Overrides.SleepInterval, ShouldBeNil)
				So(args.Overrides.Sources, ShouldBeNil)
			})
		})

		Convey("When all override args are specified", func() {
			args := parseArgs(flags,
				"--no-publish",
				"-label-whitelist=.*rdt.*",
				"-sources=fake1,fake2,fake3",
				"-sleep-interval=30s")

			Convey("args.sources is set to appropriate values", func() {
				So(args.Oneshot, ShouldBeFalse)
				So(*args.Overrides.NoPublish, ShouldBeTrue)
				So(*args.Overrides.SleepInterval, ShouldEqual, 30*time.Second)
				So(*args.Overrides.Sources, ShouldResemble, utils.StringSliceVal{"fake1", "fake2", "fake3"})
				So(args.Overrides.LabelWhiteList.Regexp.String(), ShouldResemble, ".*rdt.*")
			})
		})
	})
}
