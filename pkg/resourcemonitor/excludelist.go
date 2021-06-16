package resourcemonitor

import "k8s.io/apimachinery/pkg/util/sets"

// ToMapSet keeps the original keys, but replaces values with set.String types
func (r *ResourceExcludeList) ToMapSet() map[string]sets.String {
	asSet := make(map[string]sets.String)
	for k, v := range r.ExcludeList {
		asSet[k] = sets.NewString(v...)
	}
	return asSet
}
