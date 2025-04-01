/*
Copyright 2020-2022 The Kubernetes Authors.

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

package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	"github.com/onsi/gomega"
	nfdtopologyupdater "sigs.k8s.io/node-feature-discovery/pkg/nfd-topology-updater"
	"sigs.k8s.io/node-feature-discovery/pkg/topologypolicy"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
)

func init() {
	// make golangci-lint happy
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
}

// NewNodeResourceTopologies makes a CRD golang object representing NodeResourceTopology definition
func NewNodeResourceTopologies() (*apiextensionsv1.CustomResourceDefinition, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("cannot retrieve manifests directory")
	}

	baseDir := filepath.Dir(file)
	crdPath := filepath.Clean(filepath.Join(baseDir, "..", "..", "..", "deployment", "base", "noderesourcetopologies-crd", "noderesourcetopologies.yaml"))

	data, err := os.ReadFile(crdPath)
	if err != nil {
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("unexpected type, got %t", obj)
	}
	return crd, nil
}

// CreateNodeResourceTopologies creates the NodeResourceTopology in the cluster if the CRD doesn't exists already.
// Returns the CRD golang object present in the cluster.
func CreateNodeResourceTopologies(ctx context.Context, extClient extclient.Interface) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd, err := NewNodeResourceTopologies()
	if err != nil {
		return nil, err
	}

	// Delete existing CRD (if any) with this we also get rid of stale objects
	err = extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete NodeResourceTopology CRD: %w", err)
	}

	// It takes time for the delete operation, wait until the CRD completely gone
	if err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err = extClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
		if err == nil {
			return false, nil
		}

		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return nil, fmt.Errorf("failed to get NodeResourceTopology CRD: %w", err)
	}
	return extClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
}

// CreateNodeResourceTopology creates a dummy NodeResourceTopology object for a node
func CreateNodeResourceTopology(ctx context.Context, topologyClient *topologyclientset.Clientset, nodeName string) error {
	nrt := &v1alpha2.NodeResourceTopology{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Zones:      v1alpha2.ZoneList{},
	}
	_, err := topologyClient.TopologyV1alpha2().NodeResourceTopologies().Create(ctx, nrt, metav1.CreateOptions{})
	return err
}

// GetNodeTopology returns the NodeResourceTopology data for the node identified by `nodeName`.
func GetNodeTopology(ctx context.Context, topologyClient *topologyclientset.Clientset, nodeName string) *v1alpha2.NodeResourceTopology {
	var nodeTopology *v1alpha2.NodeResourceTopology
	var err error
	gomega.EventuallyWithOffset(1, func() bool {
		nodeTopology, err = topologyClient.TopologyV1alpha2().NodeResourceTopologies().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("failed to get the node topology resource: %v", err)
			return false
		}
		return true
	}).WithPolling(5 * time.Second).WithTimeout(1 * time.Minute).Should(gomega.BeTrue())
	return nodeTopology
}

// AllocatableResourceListFromNodeResourceTopology extract the map zone:allocatableResources from the given NodeResourceTopology instance.
func AllocatableResourceListFromNodeResourceTopology(nodeTopo *v1alpha2.NodeResourceTopology) map[string]corev1.ResourceList {
	allocRes := make(map[string]corev1.ResourceList)
	for _, zone := range nodeTopo.Zones {
		if zone.Type != "Node" {
			continue
		}
		resList := make(corev1.ResourceList)
		for _, res := range zone.Resources {
			resList[corev1.ResourceName(res.Name)] = res.Allocatable.DeepCopy()
		}
		if len(resList) == 0 {
			continue
		}
		allocRes[zone.Name] = resList
	}
	return allocRes
}

// CompareAllocatableResources compares `expected` and `got` map zone:allocatableResources respectively (see: AllocatableResourceListFromNodeResourceTopology),
// and informs the caller if the maps are equal. Here `equal` means the same zoneNames with the same resources, where the resources are equal if they have
// the same resources with the same quantities. Returns the name of the different zone, the name of the different resources within the zone,
// the comparison result (same semantic as strings.Compare) and a boolean that reports if the resourceLists are consistent. See `CompareResourceList`.
func CompareAllocatableResources(expected, got map[string]corev1.ResourceList) (string, string, int, bool) {
	if len(got) != len(expected) {
		framework.Logf("-> expected=%v (len=%d) got=%v (len=%d)", expected, len(expected), got, len(got))
		return "", "", 0, false
	}
	for expZoneName, expResList := range expected {
		gotResList, ok := got[expZoneName]
		if !ok {
			return expZoneName, "", 0, false
		}
		if resName, cmp, ok := CompareResourceList(expResList, gotResList); !ok || cmp != 0 {
			return expZoneName, resName, cmp, ok
		}
	}
	return "", "", 0, true
}

// CompareResourceList compares `expected` and `got` ResourceList respectively, and informs the caller if the two ResourceList
// are equal. Here `equal` means the same resources with the same quantities. Returns the different resource,
// the comparison result (same semantic as strings.Compare) and a boolean that reports if the resourceLists are consistent.
// The ResourceLists are consistent only if the represent the same resource set (all the resources listed in one are
// also present in the another; no ResourceList is a superset nor a subset of the other)
func CompareResourceList(expected, got corev1.ResourceList) (string, int, bool) {
	if len(got) != len(expected) {
		framework.Logf("-> expected=%v (len=%d) got=%v (len=%d)", expected, len(expected), got, len(got))
		return "", 0, false
	}
	for expResName, expResQty := range expected {
		gotResQty, ok := got[expResName]
		if !ok {
			return string(expResName), 0, false
		}
		if cmp := gotResQty.Cmp(expResQty); cmp != 0 {
			framework.Logf("-> resource=%q cmp=%d expected=%v got=%v", expResName, cmp, expResQty, gotResQty)
			return string(expResName), cmp, true
		}
	}
	return "", 0, true
}

// IsValidNodeTopology checks the provided NodeResourceTopology object if it is well-formad, internally consistent and
// consistent with the given kubelet config object. Returns true if the NodeResourceTopology object is consistent and well
// formet, false otherwise; if return false, logs the failure reason.
func IsValidNodeTopology(nodeTopology *v1alpha2.NodeResourceTopology, kubeletConfig *kubeletconfig.KubeletConfiguration) bool {
	if nodeTopology == nil || len(nodeTopology.TopologyPolicies) == 0 {
		framework.Logf("failed to get topology policy from the node topology resource")
		return false
	}

	tmPolicy := string(topologypolicy.DetectTopologyPolicy(kubeletConfig.TopologyManagerPolicy, kubeletConfig.TopologyManagerScope))
	if nodeTopology.TopologyPolicies[0] != tmPolicy {
		framework.Logf("topology policy mismatch got %q expected %q", nodeTopology.TopologyPolicies[0], tmPolicy)
		return false
	}

	expectedPolicyAttribute := v1alpha2.AttributeInfo{
		Name:  nfdtopologyupdater.TopologyManagerPolicyAttributeName,
		Value: kubeletConfig.TopologyManagerPolicy,
	}
	if !containsAttribute(nodeTopology.Attributes, expectedPolicyAttribute) {
		framework.Logf("topology policy attributes don't have correct topologyManagerPolicy attribute expected %v attributeList %v", expectedPolicyAttribute, nodeTopology.Attributes)
		return false
	}

	expectedScopeAttribute := v1alpha2.AttributeInfo{
		Name:  nfdtopologyupdater.TopologyManagerScopeAttributeName,
		Value: kubeletConfig.TopologyManagerScope,
	}
	if !containsAttribute(nodeTopology.Attributes, expectedScopeAttribute) {
		framework.Logf("topology policy attributes don't have correct topologyManagerScope attribute expected %v attributeList %v", expectedScopeAttribute, nodeTopology.Attributes)
		return false
	}

	if len(nodeTopology.Zones) == 0 {
		framework.Logf("failed to get topology zones from the node topology resource")
		return false
	}

	foundNodes := 0
	for _, zone := range nodeTopology.Zones {
		// TODO constant not in the APIs
		if !strings.HasPrefix(strings.ToUpper(zone.Type), "NODE") {
			continue
		}
		foundNodes++

		if !isValidCostList(zone.Name, zone.Costs) {
			framework.Logf("invalid cost list for zone %q", zone.Name)
			return false
		}

		if !isValidResourceList(zone.Name, zone.Resources) {
			framework.Logf("invalid resource list for zone %q", zone.Name)
			return false
		}
	}
	return foundNodes > 0
}

func isValidCostList(zoneName string, costs v1alpha2.CostList) bool {
	if len(costs) == 0 {
		framework.Logf("failed to get topology costs for zone %q from the node topology resource", zoneName)
		return false
	}

	// TODO cross-validate zone names
	for _, cost := range costs {
		if cost.Name == "" || cost.Value < 0 {
			framework.Logf("malformed cost %v for zone %q", cost, zoneName)
		}
	}
	return true
}

func isValidResourceList(zoneName string, resources v1alpha2.ResourceInfoList) bool {
	if len(resources) == 0 {
		framework.Logf("failed to get topology resources for zone %q from the node topology resource", zoneName)
		return false
	}
	foundCpu := false
	for _, resource := range resources {
		// TODO constant not in the APIs
		if strings.ToUpper(resource.Name) == "CPU" {
			foundCpu = true
		}
		allocatable, ok1 := resource.Allocatable.AsInt64()
		capacity, ok2 := resource.Capacity.AsInt64()
		if (!ok1 || !ok2) || ((allocatable < 0 || capacity < 0) || (capacity < allocatable)) {
			framework.Logf("malformed resource %v for zone %q", resource, zoneName)
			return false
		}
	}
	return foundCpu
}

func containsAttribute(attributes v1alpha2.AttributeList, attribute v1alpha2.AttributeInfo) bool {
	for _, attr := range attributes {
		if reflect.DeepEqual(attr, attribute) {
			return true
		}
	}
	return false
}
