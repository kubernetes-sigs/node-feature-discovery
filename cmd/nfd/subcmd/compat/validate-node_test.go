/*
Copyright 2026 The Kubernetes Authors.

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

package compat

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateNodePreRun(t *testing.T) {
	Convey("When validating the registry credential flags", t, func() {
		Reset(func() {
			username, password, accessToken = "", "", ""
			readPassword, readAccessToken = false, false
		})

		preRun := func(args ...string) error {
			So(validateNodeCmd.ParseFlags(args), ShouldBeNil)
			return validateNodeCmd.PreRunE(validateNodeCmd, nil)
		}

		Convey("When --registry-password-stdin is set without --registry-username", func() {
			err := preRun("--registry-password-stdin")

			Convey("Then an error is returned", func() {
				So(err, ShouldBeError, "--registry-password-stdin requires --registry-username")
			})
		})

		Convey("When --registry-username is set without --registry-password-stdin", func() {
			err := preRun("--registry-username", "user")

			Convey("Then an error is returned", func() {
				So(err, ShouldBeError, "--registry-username requires --registry-password-stdin")
			})
		})

		Convey("When --registry-username is set with --registry-password-stdin", func() {
			err := preRun("--registry-username", "user", "--registry-password-stdin")

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When --registry-username is set with --registry-token-stdin", func() {
			err := preRun("--registry-username", "user", "--registry-token-stdin")

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
