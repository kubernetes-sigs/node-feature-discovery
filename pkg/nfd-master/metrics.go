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

package nfdmaster

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
const (
	buildInfoQuery                      = "nfd_master_build_info"
	nodeUpdateRequestsQuery             = "nfd_node_update_requests_total"
	nodeUpdatesQuery                    = "nfd_node_updates_total"
	nodeFeatureGroupUpdateRequestsQuery = "nfd_node_feature_group_update_requests_total"
	nodeUpdateFailuresQuery             = "nfd_node_update_failures_total"
	nodeLabelsRejectedQuery             = "nfd_node_labels_rejected_total"
	nodeERsRejectedQuery                = "nfd_node_extendedresources_rejected_total"
	nodeTaintsRejectedQuery             = "nfd_node_taints_rejected_total"
	nfrProcessingTimeQuery              = "nfd_nodefeaturerule_processing_duration_seconds"
	nfrProcessingErrorsQuery            = "nfd_nodefeaturerule_processing_errors_total"
)

var (
	buildInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: buildInfoQuery,
		Help: "Version from which Node Feature Discovery was built.",
		ConstLabels: map[string]string{
			"version": version.Get(),
		},
	})
	nodeUpdateRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeUpdateRequestsQuery,
		Help: "Number of node update requests processed by the master.",
	})
	nodeFeatureGroupUpdateRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeFeatureGroupUpdateRequestsQuery,
		Help: "Number of cluster feature update requests processed by the master.",
	})
	nodeUpdates = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeUpdatesQuery,
		Help: "Number of nodes updated by the master.",
	})
	nodeUpdateFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeUpdateFailuresQuery,
		Help: "Number of node update failures.",
	})
	nodeLabelsRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeLabelsRejectedQuery,
		Help: "Number of node labels that were rejected by nfd-master.",
	})
	nodeERsRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeERsRejectedQuery,
		Help: "Number of node extended resources that were rejected by nfd-master.",
	})
	nodeTaintsRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeTaintsRejectedQuery,
		Help: "Number of node taints that were rejected by nfd-master.",
	})
	nfrProcessingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    nfrProcessingTimeQuery,
			Help:    "Time processing time of NodeFeatureRule objects.",
			Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01},
		},
		[]string{
			"name",
			"node",
		},
	)
	nfrProcessingErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nfrProcessingErrorsQuery,
		Help: "Number of errors encountered while processing NodeFeatureRule objects.",
	})
)

// registerVersion exposes the Operator build version.
func registerVersion(version string) {
	buildInfo.SetToCurrentTime()
}
