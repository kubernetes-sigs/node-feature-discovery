/*
Copyright 2021. The Kubernetes Authors.

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

package main

import (
	"flag"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	nfdkubernetesiov1 "github.com/kubernetes-sigs/node-feature-discovery-operator/api/v1"
	"github.com/kubernetes-sigs/node-feature-discovery-operator/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	// scheme holds a new scheme for the operator
	scheme = runtime.NewScheme()

	// setupLog will be used for logging the operator "setup" process so that users know
	// what parts of the logging are associated with the setup of the manager and
	// controller
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	//Set up the Go client and NFD schemes. Panic on errors.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(nfdkubernetesiov1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	// Setup CLI arguments
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the Prometheus "+
		"metric endpoint binds to for scraping NFD resource usage data.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe "+
		"endpoint binds to for determining liveness, readiness, and configuration of"+
		"operator pods.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// opts is created using zap to set the operator's logging
	opts := zap.Options{
		// This mode makes DPanic-level logs panic instead of just logging error events as
		// errors. The settings are then bound to the CLI flag args and the flag args are
		// then parsed.
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Create a new manager to manage the operator
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "39f5e5c3.nodefeaturediscoveries.nfd.kubernetes.io",
	})

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.NodeFeatureDiscoveryReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NodeFeatureDiscovery"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeFeatureDiscovery")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// Next, add a Healthz checker to the manager. Healthz is a health and liveness package
	// that the operator will use to periodically check the health of its pods, etc.
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// Now add a ReadyZ checker to the manager as well. It is important to ensure that the
	// API server's readiness is checked when the operator is installed and running.
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Register signal handler for SIGINT and SIGTERM to terminate the manager
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
