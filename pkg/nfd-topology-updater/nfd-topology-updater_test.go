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

package nfdtopologyupdater

import (
	"fmt"
	"testing"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTopologyUpdater(t *testing.T) {

	Convey("Given a list of Attributes", t, func() {

		attr_two := v1alpha2.AttributeInfo{
			Name:  "attr_two_name",
			Value: "attr_two_value",
		}

		attrList := v1alpha2.AttributeList{
			v1alpha2.AttributeInfo{
				Name:  "attr_one_name",
				Value: "attr_one_value",
			},
			attr_two,
			v1alpha2.AttributeInfo{
				Name:  "attr_three_name",
				Value: "attr_three_value",
			},
		}
		attrListLen := len(attrList)
		attrNames := getListOfNames(attrList)

		Convey("When an existing attribute is updated", func() {

			updatedAttribute := v1alpha2.AttributeInfo{
				Name:  attr_two.Name,
				Value: attr_two.Value + "_new",
			}
			updateAttribute(&attrList, updatedAttribute)

			Convey("Then list should have the same number of elements", func() {
				So(attrList, ShouldHaveLength, attrListLen)
			})
			Convey("Then the order of the elemens should be the same", func() {
				So(attrNames, ShouldResemble, getListOfNames(attrList))
			})
			Convey("Then Attribute value in the list should be updated", func() {
				attr, err := findAttributeByName(attrList, attr_two.Name)
				So(err, ShouldBeNil)
				So(attr.Value, ShouldEqual, updatedAttribute.Value)
			})
		})

		Convey("When a non existing attribute is updated", func() {
			completelyNewAttribute := v1alpha2.AttributeInfo{
				Name:  "NonExistingAttribute_Name",
				Value: "NonExistingAttribute_Value",
			}
			_, err := findAttributeByName(attrList, completelyNewAttribute.Name)
			So(err, ShouldNotBeNil)

			updateAttribute(&attrList, completelyNewAttribute)

			Convey("Then list should have the one more element", func() {
				So(attrList, ShouldHaveLength, attrListLen+1)
			})

			Convey("Then new Attribute should be added at the end of the list", func() {
				So(attrList[len(attrList)-1], ShouldResemble, completelyNewAttribute)
			})

			Convey("Then the order of the elemens should be the same", func() {
				So(attrNames, ShouldResemble, getListOfNames(attrList[:len(attrList)-1]))
			})
		})
	})
}

func getListOfNames(attrList v1alpha2.AttributeList) []string {
	ret := make([]string, len(attrList))

	for idx, attr := range attrList {
		ret[idx] = attr.Name
	}
	return ret
}

func findAttributeByName(attrList v1alpha2.AttributeList, name string) (v1alpha2.AttributeInfo, error) {
	for _, attr := range attrList {
		if attr.Name == name {
			return attr, nil
		}
	}
	return v1alpha2.AttributeInfo{}, fmt.Errorf("Attribute Not Found name:=%s", name)
}
