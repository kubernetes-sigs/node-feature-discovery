/*
Copyright 2015 The Kubernetes Authors.

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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
)

var (
	dockerRepo = flag.String("nfd.repo", "gcr.io/k8s-staging-nfd/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag  = flag.String("nfd.tag", "master", "Docker tag to use")
)

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()
}

// must be called after flags are parsed
func dockerImage() string {
	return fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
}

func TestMain(m *testing.M) {
	// Register test flags, then parse flags.
	handleFlags()

	framework.AfterReadingAllFlags(&framework.TestContext)

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	runE2ETests(t)
}

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
// If a "report directory" is specified, one or more JUnit test reports will be
// generated in this directory, and cluster logs will also be saved.
// This function is called on each Ginkgo node in parallel mode.
func runE2ETests(t *testing.T) {
	// InitLogs disables contextual logging, without a way to enable it again
	// in the E2E test suite because it has no feature gates. It used to have a
	// misleading --feature-gates parameter but that didn't do what users
	// and developers expected (define which features the cluster supports)
	// and therefore got removed.
	//
	// Because contextual logging is useful and should get tested, it gets
	// re-enabled here unconditionally.
	logs.InitLogs()
	defer logs.FlushLogs()
	klog.EnableContextualLogging(true)

	gomega.RegisterFailHandler(framework.Fail)

	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	suiteConfig, reporterConfig := framework.CreateGinkgoConfig()
	klog.Infof("Starting e2e run %q on Ginkgo node %d", framework.RunID, suiteConfig.ParallelProcess)
	ginkgo.RunSpecs(t, "Kubernetes e2e suite", suiteConfig, reporterConfig)
}
