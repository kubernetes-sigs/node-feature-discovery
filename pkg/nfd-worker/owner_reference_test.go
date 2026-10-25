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

package nfdworker

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func TestOwnerRefSourcesSet(t *testing.T) {
	testCases := []struct {
		name    string
		value   string
		want    OwnerRefSources
		wantErr bool
	}{
		{name: "single", value: "node", want: OwnerRefSources{"node"}},
		{name: "canonical order", value: "ds, node,pod", want: OwnerRefSources{"node", "pod", "ds"}},
		{name: "empty", value: "", want: OwnerRefSources{}},
		{name: "unknown", value: "job", wantErr: true},
		{name: "duplicate", value: "pod,pod", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var got OwnerRefSources
			err := got.Set(tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Set() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestOwnerRefSourcesUnmarshalJSON(t *testing.T) {
	var got OwnerRefSources
	if err := json.Unmarshal([]byte(`["ds", "node"]`), &got); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}
	want := OwnerRefSources{"node", "ds"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() = %#v, want %#v", got, want)
	}

	if err := json.Unmarshal([]byte(`["invalid"]`), &got); err == nil {
		t.Fatal("expected invalid source to be rejected")
	}
}

func TestDefaultOwnerRefs(t *testing.T) {
	got := newDefaultConfig().Core.OwnerRefs
	if got == nil {
		t.Fatal("default ownerRefs is nil")
	}
	want := OwnerRefSources{"pod", "ds"}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("default ownerRefs = %#v, want %#v", *got, want)
	}
}

func TestResolveOwnerReferences(t *testing.T) {
	t.Setenv("NODE_NAME", "fake-node")
	t.Setenv("POD_NAME", "fake-worker")
	t.Setenv("POD_UID", "pod-uid")

	controller := true
	nodeRef := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Node",
		Name:               "fake-node",
		UID:                types.UID("node-uid"),
		BlockOwnerDeletion: new(false),
	}
	podRef := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               "fake-worker",
		UID:                types.UID("pod-uid"),
		BlockOwnerDeletion: new(false),
	}
	dsRef := metav1.OwnerReference{
		APIVersion:         "apps/v1",
		Kind:               "DaemonSet",
		Name:               "nfd-worker",
		UID:                types.UID("ds-uid"),
		Controller:         &controller,
		BlockOwnerDeletion: new(false),
	}

	client := fakeclient.NewSimpleClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "fake-node", UID: types.UID("node-uid")}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-worker",
				Namespace: "fake-ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "DaemonSet",
						Name:               "nfd-worker",
						UID:                "ds-uid",
						Controller:         &controller,
						BlockOwnerDeletion: new(true),
					},
				},
			},
		},
	)
	worker := &nfdWorker{k8sClient: client, kubernetesNamespace: "fake-ns"}

	testCases := []struct {
		name    string
		sources OwnerRefSources
		want    []metav1.OwnerReference
	}{
		{name: "none", sources: OwnerRefSources{}, want: []metav1.OwnerReference{}},
		{name: "node", sources: OwnerRefSources{"node"}, want: []metav1.OwnerReference{nodeRef}},
		{name: "pod", sources: OwnerRefSources{"pod"}, want: []metav1.OwnerReference{podRef}},
		{name: "daemonset", sources: OwnerRefSources{"ds"}, want: []metav1.OwnerReference{dsRef}},
		{name: "node and pod", sources: OwnerRefSources{"node", "pod"}, want: []metav1.OwnerReference{nodeRef, podRef}},
		{name: "node and daemonset", sources: OwnerRefSources{"node", "ds"}, want: []metav1.OwnerReference{nodeRef, dsRef}},
		{name: "pod and daemonset", sources: OwnerRefSources{"pod", "ds"}, want: []metav1.OwnerReference{podRef, dsRef}},
		{name: "all", sources: OwnerRefSources{"node", "pod", "ds"}, want: []metav1.OwnerReference{nodeRef, podRef, dsRef}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := worker.resolveOwnerReferences(tc.sources)
			if err != nil {
				t.Fatalf("resolveOwnerReferences() returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolveOwnerReferences() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestResolveNodeOwnerReferenceRequiresNode(t *testing.T) {
	t.Setenv("NODE_NAME", "fake-node")
	worker := &nfdWorker{k8sClient: fakeclient.NewSimpleClientset()}
	if _, err := worker.resolveOwnerReferences(OwnerRefSources{"node"}); err == nil {
		t.Fatal("expected missing Node to return an error")
	}
}
