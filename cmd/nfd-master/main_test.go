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

	. "github.com/smartystreets/goconvey/convey"
)

func TestArgsParse(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		Convey("When --no-publish and --oneshot flags are passed", func() {
			args, err := argsParse([]string{"--no-publish"})
			Convey("noPublish is set and args.sources is set to the default value", func() {
				So(args.NoPublish, ShouldBeTrue)
				So(len(args.LabelWhiteList.String()), ShouldEqual, 0)
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified", func() {
			args, err := argsParse([]string{"--label-whitelist=.*rdt.*", "--port=1234", "--cert-file=crt", "--key-file=key", "--ca-file=ca"})
			Convey("Argument parsing should succeed and args set to correct values", func() {
				So(args.NoPublish, ShouldBeFalse)
				So(args.Port, ShouldEqual, 1234)
				So(args.CertFile, ShouldEqual, "crt")
				So(args.KeyFile, ShouldEqual, "key")
				So(args.CaFile, ShouldEqual, "ca")
				So(args.LabelWhiteList.String(), ShouldResemble, ".*rdt.*")
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
