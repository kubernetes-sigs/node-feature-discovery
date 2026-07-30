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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	faketopologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned/fake"
	"github.com/k8stopologyawareschedwg/podfingerprint"
	. "github.com/smartystreets/goconvey/convey"

	"sigs.k8s.io/node-feature-discovery/pkg/resourcemonitor"
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

func TestUpdateNodeResourceTopologyApplies(t *testing.T) {
	Convey("When NodeResourceTopology is published", t, func() {
		topoClient := faketopologyclientset.NewSimpleClientset()
		topoClient.PrependReactor("patch", "noderesourcetopologies", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, &v1alpha2.NodeResourceTopology{}, nil
		})
		updater := newTestTopologyUpdater(topoClient)
		zones := v1alpha2.ZoneList{
			{
				Name: "node-0",
				Type: "Node",
			},
		}
		scanResponse := resourcemonitor.ScanResponse{
			Attributes: v1alpha2.AttributeList{
				{
					Name:  podfingerprint.Attribute,
					Value: "pfp0v001",
				},
			},
		}

		err := updater.updateNodeResourceTopology(zones, scanResponse)
		So(err, ShouldBeNil)

		actions := topoClient.Actions()
		So(actions, ShouldHaveLength, 1)
		patchAction := actions[0].(k8stesting.PatchActionImpl)

		applied := &v1alpha2.NodeResourceTopology{}
		err = json.Unmarshal(patchAction.GetPatch(), applied)
		So(err, ShouldBeNil)

		Convey("Then the missing object is created with server-side apply", func() {
			So(patchAction.GetVerb(), ShouldEqual, "patch")
			So(patchAction.GetPatchType(), ShouldEqual, types.ApplyPatchType)
		})

		Convey("Then the apply uses server-side apply", func() {
			So(patchAction.PatchOptions.FieldManager, ShouldEqual, nodeResourceTopologyFieldManager)
			So(*patchAction.PatchOptions.Force, ShouldBeTrue)
		})

		Convey("Then the apply payload contains the desired NRT fields", func() {
			So(applied.APIVersion, ShouldEqual, v1alpha2.SchemeGroupVersion.String())
			So(applied.Kind, ShouldEqual, "NodeResourceTopology")
			So(applied.Name, ShouldEqual, "test-node")
			So(applied.OwnerReferences, ShouldHaveLength, 1)
			So(applied.OwnerReferences[0].Name, ShouldEqual, "node-feature-discovery")
			So(applied.Zones, ShouldResemble, zones)
			So(applied.TopologyPolicies, ShouldNotBeEmpty)

			policy, err := findAttributeByName(applied.Attributes, TopologyManagerPolicyAttributeName)
			So(err, ShouldBeNil)
			So(policy.Value, ShouldEqual, "single-numa-node")

			scope, err := findAttributeByName(applied.Attributes, TopologyManagerScopeAttributeName)
			So(err, ShouldBeNil)
			So(scope.Value, ShouldEqual, "container")

			fingerprint, err := findAttributeByName(applied.Attributes, podfingerprint.Attribute)
			So(err, ShouldBeNil)
			So(fingerprint.Value, ShouldEqual, "pfp0v001")
		})
	})
}

func TestUpdateNodeResourceTopologyUsesCachedTopologyInfo(t *testing.T) {
	Convey("When NodeResourceTopology is refreshed after an interval update", t, func() {
		topoClient := faketopologyclientset.NewSimpleClientset(&v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		})
		updater := newTestTopologyUpdater(topoClient)
		kubeletConfigReads := 0
		cpuCoreReads := 0
		updater.kubeletConfigFunc = func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			kubeletConfigReads++
			return &kubeletconfigv1beta1.KubeletConfiguration{
				TopologyManagerPolicy: "single-numa-node",
				TopologyManagerScope:  "container",
			}, nil
		}
		updater.discoverCpuCoresFunc = func() v1alpha2.AttributeList {
			cpuCoreReads++
			return v1alpha2.AttributeList{
				{
					Name:  "cpu_performance",
					Value: "0-3",
				},
			}
		}
		zones := v1alpha2.ZoneList{
			{
				Name: "node-0",
				Type: "Node",
			},
		}

		err := updater.refreshTopologyManagerInfo(context.Background())
		So(err, ShouldBeNil)

		err = updater.updateNodeResourceTopology(zones, resourcemonitor.ScanResponse{
			Attributes: v1alpha2.AttributeList{
				{
					Name:  podfingerprint.Attribute,
					Value: "pfp0v001",
				},
			},
		})
		So(err, ShouldBeNil)

		err = updater.updateNodeResourceTopology(zones, resourcemonitor.ScanResponse{
			Attributes: v1alpha2.AttributeList{
				{
					Name:  podfingerprint.Attribute,
					Value: "pfp0v002",
				},
			},
		})

		Convey("Then cached topology-manager attributes are included without re-reading kubelet config or sysfs", func() {
			So(err, ShouldBeNil)
			So(kubeletConfigReads, ShouldEqual, 1)
			So(cpuCoreReads, ShouldEqual, 1)
			So(topoClient.Actions(), ShouldHaveLength, 2)

			tracked, err := topoClient.Tracker().Get(
				v1alpha2.SchemeGroupVersion.WithResource("noderesourcetopologies"),
				"",
				"test-node",
			)
			So(err, ShouldBeNil)
			applied := tracked.(*v1alpha2.NodeResourceTopology)

			policy, err := findAttributeByName(applied.Attributes, TopologyManagerPolicyAttributeName)
			So(err, ShouldBeNil)
			So(policy.Value, ShouldEqual, "single-numa-node")

			scope, err := findAttributeByName(applied.Attributes, TopologyManagerScopeAttributeName)
			So(err, ShouldBeNil)
			So(scope.Value, ShouldEqual, "container")

			cpuCores, err := findAttributeByName(applied.Attributes, "cpu_performance")
			So(err, ShouldBeNil)
			So(cpuCores.Value, ShouldEqual, "0-3")

			fingerprint, err := findAttributeByName(applied.Attributes, podfingerprint.Attribute)
			So(err, ShouldBeNil)
			So(fingerprint.Value, ShouldEqual, "pfp0v002")
		})
	})
}

func TestUpdateNodeResourceTopologyRetriesMissingTopologyInfo(t *testing.T) {
	Convey("Given topology-manager information is not cached", t, func() {
		topoClient := faketopologyclientset.NewSimpleClientset(&v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		})
		updater := newTestTopologyUpdater(topoClient)
		kubeletConfigReads := 0
		updater.kubeletConfigFunc = func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			kubeletConfigReads++
			if kubeletConfigReads == 1 {
				return nil, errors.New("kubelet unavailable")
			}
			return &kubeletconfigv1beta1.KubeletConfiguration{
				TopologyManagerPolicy: "single-numa-node",
				TopologyManagerScope:  "container",
			}, nil
		}

		err := updater.updateNodeResourceTopology(nil, resourcemonitor.ScanResponse{})
		So(errors.Is(err, errTopologyManagerInfoUnavailable), ShouldBeTrue)
		So(topoClient.Actions(), ShouldBeEmpty)

		err = updater.updateNodeResourceTopology(nil, resourcemonitor.ScanResponse{})
		So(err, ShouldBeNil)
		So(kubeletConfigReads, ShouldEqual, 2)
		So(topoClient.Actions(), ShouldHaveLength, 1)
	})
}

func TestTopologyManagerRefreshContextStops(t *testing.T) {
	Convey("Given the topology updater is stopped", t, func() {
		updater := &nfdTopologyUpdater{stop: make(chan struct{})}
		ctx, cancel := updater.newTopologyManagerRefreshContext()
		defer cancel()

		close(updater.stop)

		select {
		case <-ctx.Done():
			So(ctx.Err(), ShouldEqual, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("refresh context was not canceled")
		}
	})
}

func TestUpdateNodeResourceTopologyApplyErrors(t *testing.T) {
	const resource = "noderesourcetopologies"

	Convey("Given the NodeResourceTopology resource is unavailable", t, func() {
		topoClient := faketopologyclientset.NewSimpleClientset(&v1alpha2.NodeResourceTopology{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
		})
		notFoundErr := apierrors.NewNotFound(
			v1alpha2.SchemeGroupVersion.WithResource(resource).GroupResource(),
			"test-node",
		)
		topoClient.PrependReactor("patch", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, notFoundErr
		})

		err := newTestTopologyUpdater(topoClient).updateNodeResourceTopology(nil, resourcemonitor.ScanResponse{})

		So(apierrors.IsNotFound(err), ShouldBeTrue)
		So(err.Error(), ShouldContainSubstring, "The NodeResourceTopology CRD may not be installed")
	})

	Convey("Given applying the NodeResourceTopology fails", t, func() {
		topoClient := faketopologyclientset.NewSimpleClientset()
		applyErr := errors.New("apply failed")
		topoClient.PrependReactor("patch", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, applyErr
		})

		err := newTestTopologyUpdater(topoClient).updateNodeResourceTopology(nil, resourcemonitor.ScanResponse{})

		So(errors.Is(err, applyErr), ShouldBeTrue)
		So(err.Error(), ShouldContainSubstring, "failed to apply NodeResourceTopology")
	})
}

func newTestTopologyUpdater(topoClient topologyclientset.Interface) *nfdTopologyUpdater {
	return &nfdTopologyUpdater{
		nodeName:            "test-node",
		kubernetesNamespace: "node-feature-discovery",
		k8sClient: fakeclient.NewClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-feature-discovery",
				UID:  types.UID("namespace-uid"),
			},
		}),
		topoClient: topoClient,
		kubeletConfigFunc: func(context.Context) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
			return &kubeletconfigv1beta1.KubeletConfiguration{
				TopologyManagerPolicy: "single-numa-node",
				TopologyManagerScope:  "container",
			}, nil
		},
		discoverCpuCoresFunc: func() v1alpha2.AttributeList {
			return nil
		},
	}
}
