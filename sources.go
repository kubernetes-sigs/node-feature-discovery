package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"

	"github.com/klauspost/cpuid"
	api "k8s.io/client-go/pkg/api/v1"
)

// FeatureSource represents a source of discovered node features.
type FeatureSource interface {
	// Returns a friendly name for this source of node features.
	Name() string

	// Returns discovered binary features for this node.
	Discover() ([]string, error)

	// Returns quantities of discovered opaque integer resources for this node.
	DiscoverResources() (api.ResourceList, error)
}

const (
	// RDTBin is the path to RDT detection helpers.
	RDTBin = "/go/src/github.com/kubernetes-incubator/node-feature-discovery/rdt-discovery"
)

////////////////////////////////////////////////////////////////////////////////
// CPUID Source

// Implements main.FeatureSource.
type cpuidSource struct{}

func (s cpuidSource) Name() string { return "cpuid" }
func (s cpuidSource) Discover() ([]string, error) {
	// Get the cpu features as strings
	return cpuid.CPU.Features.Strings(), nil
}
func (s cpuidSource) DiscoverResources() (api.ResourceList, error) {
	return nil, nil
}

////////////////////////////////////////////////////////////////////////////////
// RDT (Intel Resource Director Technology) Source

// Implements main.FeatureSource.
type rdtSource struct{}

func (s rdtSource) Name() string { return "rdt" }

// Returns feature names for CMT, MBM and CAT if suppported.
func (s rdtSource) Discover() ([]string, error) {
	features := []string{}

	cmd := exec.Command("bash", "-c", path.Join(RDTBin, "mon-discovery"))
	if err := cmd.Run(); err != nil {
		stderrLogger.Printf("support for RDT monitoring was not detected: %s", err.Error())
	} else {
		// RDT monitoring detected.
		features = append(features, "RDTMON")
	}

	cmd = exec.Command("bash", "-c", path.Join(RDTBin, "l3-alloc-discovery"))
	if err := cmd.Run(); err != nil {
		stderrLogger.Printf("support for RDT L3 allocation was not detected: %s", err.Error())
	} else {
		// RDT monitoring detected.
		features = append(features, "RDTL3CA")
	}

	cmd = exec.Command("bash", "-c", path.Join(RDTBin, "l2-alloc-discovery"))
	if err := cmd.Run(); err != nil {
		stderrLogger.Printf("support for RDT L2 allocation was not detected: %s", err.Error())
	} else {
		// RDT monitoring detected.
		features = append(features, "RDTL2CA")
	}

	return features, nil
}
func (s rdtSource) DiscoverResources() (api.ResourceList, error) {
	return nil, nil
}

////////////////////////////////////////////////////////////////////////////////
// PState Source

// Implements main.FeatureSource.
type pstateSource struct{}

func (s pstateSource) Name() string { return "pstate" }
func (s pstateSource) Discover() ([]string, error) {
	features := []string{}

	// Only looking for turbo boost for now...
	bytes, err := ioutil.ReadFile("/sys/devices/system/cpu/intel_pstate/no_turbo")
	if err != nil {
		return nil, fmt.Errorf("can't detect whether turbo boost is enabled: %s", err.Error())
	}
	if bytes[0] == byte('0') {
		// Turbo boost is enabled.
		features = append(features, "turbo")
	}

	return features, nil
}
func (s pstateSource) DiscoverResources() (api.ResourceList, error) {
	return nil, nil
}
