package resourcemonitor

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

// ExcludeResourceList contains a list of resources to ignore during resources scan
type ExcludeResourceList struct {
	excludeList sets.Set[string]
}

// NewExcludeResourceList returns new ExcludeList with values with set.String types
func NewExcludeResourceList(resMap map[string][]string, nodeName string) ExcludeResourceList {
	excludeList := make(sets.Set[string])

	for k, v := range resMap {
		if k == nodeName || k == "*" {
			excludeList.Insert(v...)
		}
	}
	return ExcludeResourceList{
		excludeList: excludeList,
	}
}

func (rl *ExcludeResourceList) IsExcluded(resource corev1.ResourceName) bool {
	if rl.excludeList.Has(string(resource)) {
		klog.V(5).InfoS("resource excluded", "resourceName", resource)
		return true
	}
	return false
}
