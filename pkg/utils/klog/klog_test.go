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
