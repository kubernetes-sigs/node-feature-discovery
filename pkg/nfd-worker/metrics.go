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

package nfdworker

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
// When adding metric names, see https://prometheus.io/docs/practices/naming/#metric-names
const (
	buildInfoQuery                = "nfd_worker_build_info"
	featureDiscoveryDurationQuery = "nfd_feature_discovery_duration_seconds"
)

var (
	srv *http.Server

	featureDiscoveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: featureDiscoveryDurationQuery,
			Help: "Time taken to discover features",
		},
		[]string{"NodeName"},
	)
	buildInfo = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: buildInfoQuery,
		Help: "Version from which Node Feature Discovery was built.",
		ConstLabels: map[string]string{
			"version": version.Get(),
		},
	})
)

// registerVersion exposes the Operator build version.
func registerVersion(version string) {
	buildInfo.SetToCurrentTime()
}

// runMetricsServer starts a http server to expose metrics
func runMetricsServer(port int) {
	r := prometheus.NewRegistry()
	r.MustRegister(featureDiscoveryDuration)
	r.MustRegister(buildInfo)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	klog.InfoS("metrics server starting", "port", port)
	srv = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	klog.InfoS("metrics server stopped", "exit code", srv.ListenAndServe())
}

// stopMetricsServer stops the metrics server
func stopMetricsServer() {
	if srv != nil {
		klog.InfoS("stopping metrics server", "port", srv.Addr)
		srv.Close()
	}
}
