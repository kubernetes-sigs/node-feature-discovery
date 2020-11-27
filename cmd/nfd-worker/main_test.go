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
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

var allSources = []string{"all"}

func TestArgsParse(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		Convey("When --no-publish and --oneshot flags are passed", func() {
			args, err := argsParse([]string{"--no-publish", "--oneshot"})

			Convey("noPublish is set and args.sources is set to the default value", func() {
				So(args.SleepInterval, ShouldEqual, 60*time.Second)
				So(*args.NoPublish, ShouldBeTrue)
				So(args.Oneshot, ShouldBeTrue)
				So(args.Sources, ShouldResemble, allSources)
				So(len(args.LabelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When --sources flag is passed and set to some values, --sleep-inteval is specified", func() {
			args, err := argsParse([]string{"--sources=fake1,fake2,fake3", "--sleep-interval=30s"})

			Convey("args.sources is set to appropriate values", func() {
				So(args.SleepInterval, ShouldEqual, 30*time.Second)
				So(args.NoPublish, ShouldBeNil)
				So(args.Oneshot, ShouldBeFalse)
				So(args.Sources, ShouldResemble, []string{"fake1", "fake2", "fake3"})
				So(len(args.LabelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When --label-whitelist flag is passed and set to some value", func() {
			args, err := argsParse([]string{"--label-whitelist=.*rdt.*"})

			Convey("args.labelWhiteList is set to appropriate value and args.sources is set to default value", func() {
				So(args.NoPublish, ShouldBeNil)
				So(args.Sources, ShouldResemble, allSources)
				So(args.LabelWhiteList, ShouldResemble, ".*rdt.*")
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified", func() {
			args, err := argsParse([]string{"--no-publish", "--sources=fake1,fake2,fake3", "--ca-file=ca", "--cert-file=crt", "--key-file=key"})

			Convey("--no-publish is set and args.sources is set to appropriate values", func() {
				So(*args.NoPublish, ShouldBeTrue)
				So(args.CaFile, ShouldEqual, "ca")
				So(args.CertFile, ShouldEqual, "crt")
				So(args.KeyFile, ShouldEqual, "key")
				So(args.Sources, ShouldResemble, []string{"fake1", "fake2", "fake3"})
				So(len(args.LabelWhiteList), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})
	})
}
