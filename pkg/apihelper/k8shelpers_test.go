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

package apihelper_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	api "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/node-feature-discovery/pkg/apihelper"
)

func TestAddLabels(t *testing.T) {
	Convey("When adding labels", t, func() {
		labelNs := "test.nfd/"
		helper := apihelper.K8sHelpers{LabelNs: labelNs}
		labels := map[string]string{}
		n := &api.Node{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: map[string]string{},
			},
		}

		Convey("If no labels are passed", func() {
			helper.AddLabels(n, labels)

			Convey("None should be added", func() {
				So(len(n.Labels), ShouldEqual, 0)
			})
		})

		Convey("They should be added to the node.Labels", func() {
			test1 := "test1"
			labels[test1] = "true"
			helper.AddLabels(n, labels)
			So(n.Labels, ShouldContainKey, labelNs+test1)
		})
	})
}

func TestRemoveLabelsWithPrefix(t *testing.T) {
	Convey("When removing labels", t, func() {
		helper := apihelper.K8sHelpers{}
		n := &api.Node{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: map[string]string{
					"single-label": "123",
					"multiple_A":   "a",
					"multiple_B":   "b",
				},
			},
		}

		Convey("a unique label should be removed", func() {
			helper.RemoveLabelsWithPrefix(n, "single")
			So(len(n.Labels), ShouldEqual, 2)
			So(n.Labels, ShouldNotContainKey, "single")
		})

		Convey("a non-unique search string should remove all matching keys", func() {
			helper.RemoveLabelsWithPrefix(n, "multiple")
			So(len(n.Labels), ShouldEqual, 1)
			So(n.Labels, ShouldNotContainKey, "multiple_A")
			So(n.Labels, ShouldNotContainKey, "multiple_B")
		})

		Convey("a search string with no matches should not alter labels", func() {
			helper.RemoveLabelsWithPrefix(n, "i")
			So(n.Labels, ShouldContainKey, "single-label")
			So(n.Labels, ShouldContainKey, "multiple_A")
			So(n.Labels, ShouldContainKey, "multiple_B")
			So(len(n.Labels), ShouldEqual, 3)
		})
	})
}
