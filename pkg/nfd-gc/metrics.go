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

package nfdgarbagecollector

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
const (
	buildInfoQuery          = "nfd_gc_build_info"
	objectsDeletedQuery     = "nfd_gc_objects_deleted_total"
	objectDeleteErrorsQuery = "nfd_gc_object_delete_failures_total"
)

var (
	buildInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: buildInfoQuery,
		Help: "Version from which Node Feature Discovery was built.",
		ConstLabels: map[string]string{
			"version": version.Get(),
		},
	})
	objectsDeleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: objectsDeletedQuery,
		Help: "Number of NodeFeature and NodeResourceTopology objects garbage collected."},
		[]string{"kind"},
	)
	objectDeleteErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: objectDeleteErrorsQuery,
		Help: "Number of errors in deleting NodeFeature and NodeResourceTopology objects."},
		[]string{"kind"},
	)
)

// registerVersion exposes the Operator build version.
func registerVersion(version string) {
	buildInfo.SetToCurrentTime()
}
