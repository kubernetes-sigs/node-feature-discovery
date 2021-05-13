/*
Copyright 2020 The Kubernetes Authors.

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

func TestArgsParse(t *testing.T) {
	Convey("When parsing command line arguments", t, func() {
		Convey("When --no-publish and --oneshot flags are passed", func() {
			args, finderArgs, err := argsParse([]string{"--no-publish", "--oneshot"})

			Convey("noPublish is set and args.sources is set to the default value", func() {
				So(args.NoPublish, ShouldBeTrue)
				So(args.Oneshot, ShouldBeTrue)
				So(finderArgs.SleepInterval, ShouldEqual, 60*time.Second)
				So(finderArgs.SysfsRoot, ShouldEqual, "/host")
				So(finderArgs.KubeletConfigFile, ShouldEqual, "/podresources/config.yaml")
				So(finderArgs.PodResourceSocketPath, ShouldEqual, "/podresources/kubelet.sock")
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified for --kubelet-config-file and --sleep-interval,", func() {
			args, finderArgs, err := argsParse([]string{"--kubelet-config-file=/path/testconfig.yaml", "--sleep-interval=30s"})

			Convey("args.sources is set to appropriate values", func() {
				So(args.NoPublish, ShouldBeFalse)
				So(args.Oneshot, ShouldBeFalse)
				So(finderArgs.SleepInterval, ShouldEqual, 30*time.Second)
				So(finderArgs.SysfsRoot, ShouldEqual, "/host")
				So(finderArgs.KubeletConfigFile, ShouldEqual, "/path/testconfig.yaml")
				So(finderArgs.PodResourceSocketPath, ShouldEqual, "/podresources/kubelet.sock")
				So(err, ShouldBeNil)
			})
		})

		Convey("When valid args are specified for --podresources-socket flag and --sleep-interval is specified", func() {
			args, finderArgs, err := argsParse([]string{"--podresources-socket=/path/testkubelet.sock", "--sleep-interval=30s"})

			Convey("args.sources is set to appropriate values", func() {
				So(args.NoPublish, ShouldBeFalse)
				So(args.Oneshot, ShouldBeFalse)
				So(finderArgs.SleepInterval, ShouldEqual, 30*time.Second)
				So(finderArgs.SysfsRoot, ShouldEqual, "/host")
				So(finderArgs.KubeletConfigFile, ShouldEqual, "/podresources/config.yaml")
				So(finderArgs.PodResourceSocketPath, ShouldEqual, "/path/testkubelet.sock")
				So(err, ShouldBeNil)
			})
		})
		Convey("When valid args are specified for--sysfs and --sleep-inteval is specified", func() {
			args, finderArgs, err := argsParse([]string{"--sysfs=/host/sysfs-root", "--sleep-interval=30s"})

			Convey("args.sources is set to appropriate values", func() {
				So(args.NoPublish, ShouldBeFalse)
				So(args.Oneshot, ShouldBeFalse)
				So(finderArgs.SleepInterval, ShouldEqual, 30*time.Second)
				So(finderArgs.SysfsRoot, ShouldEqual, "/host/sysfs-root")
				So(finderArgs.KubeletConfigFile, ShouldEqual, "/podresources/config.yaml")
				So(finderArgs.PodResourceSocketPath, ShouldEqual, "/podresources/kubelet.sock")
				So(err, ShouldBeNil)
			})
		})

		Convey("When All valid args are specified", func() {
			args, finderArgs, err := argsParse([]string{"--no-publish", "--sleep-interval=30s", "--sysfs=/host/sysfs-root", "--kubelet-config-file=/path/testconfig.yaml", "--podresources-socket=/path/testkubelet.sock", "--ca-file=ca", "--cert-file=crt", "--key-file=key"})

			Convey("--no-publish is set and args.sources is set to appropriate values", func() {
				So(args.NoPublish, ShouldBeTrue)
				So(args.CaFile, ShouldEqual, "ca")
				So(args.CertFile, ShouldEqual, "crt")
				So(args.KeyFile, ShouldEqual, "key")
				So(finderArgs.SleepInterval, ShouldEqual, 30*time.Second)
				So(finderArgs.SysfsRoot, ShouldEqual, "/host/sysfs-root")
				So(finderArgs.KubeletConfigFile, ShouldEqual, "/path/testconfig.yaml")
				So(finderArgs.PodResourceSocketPath, ShouldEqual, "/path/testkubelet.sock")
				So(err, ShouldBeNil)
			})
		})
	})
}
