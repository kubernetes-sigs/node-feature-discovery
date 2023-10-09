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

package utils

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

type MetricsServer struct {
	srv *http.Server
}

// RunMetricsServer starts a new http server to expose metrics.
func CreateMetricsServer(port int, cs ...prometheus.Collector) *MetricsServer {
	r := prometheus.NewRegistry()
	r.MustRegister(cs...)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	return &MetricsServer{srv: &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}}
}

// Run runs the metrics server.
func (s *MetricsServer) Run() {
	klog.InfoS("metrics server starting", "port", s.srv.Addr)
	klog.InfoS("metrics server stopped", "exitCode", s.srv.ListenAndServe())
}

// Stop stops the metrics server.
func (s *MetricsServer) Stop() {
	if s.srv != nil {
		klog.InfoS("stopping metrics server", "port", s.srv.Addr)
		s.srv.Close()
	}
}
