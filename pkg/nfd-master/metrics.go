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
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
const (
	buildInfoQuery           = "nfd_master_build_info"
	nodeUpdatesQuery         = "nfd_node_updates_total"
	nodeUpdateFailuresQuery  = "nfd_node_update_failures_total"
	nfrProcessingTimeQuery   = "nfd_nodefeaturerule_processing_duration_seconds"
	nfrProcessingErrorsQuery = "nfd_nodefeaturerule_processing_errors_total"
)

var (
	srv *http.Server

	buildInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: buildInfoQuery,
		Help: "Version from which Node Feature Discovery was built.",
		ConstLabels: map[string]string{
			"version": version.Get(),
		},
	})
	nodeUpdates = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeUpdatesQuery,
		Help: "Number of nodes updated by the master.",
	})
	nodeUpdateFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: nodeUpdateFailuresQuery,
		Help: "Number of node update failures.",
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

// runMetricsServer starts a http server to expose metrics
func runMetricsServer(port int) {
	r := prometheus.NewRegistry()
	r.MustRegister(buildInfo,
		nodeUpdates,
		nodeUpdateFailures,
		nfrProcessingTime,
		nfrProcessingErrors)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	klog.InfoS("metrics server starting", "port", port)
	srv = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	klog.InfoS("metrics server stopped", "exitCode", srv.ListenAndServe())
}

// stopMetricsServer stops the metrics server
func stopMetricsServer() {
	if srv != nil {
		klog.InfoS("stopping metrics server", "port", srv.Addr)
		srv.Close()
	}
}
