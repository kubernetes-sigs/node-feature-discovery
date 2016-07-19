package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"

	"github.com/klauspost/cpuid"
)

// FeatureSource represents a source of discovered node features.
type FeatureSource interface {
	// Returns a friendly name for this source of node features.
	Name() string

	// Returns discovered features for this node.
	Discover() ([]string, error)
}

const (
	// DETECTED is compared with stdout for RDT detection helper programs.
	DETECTED = "DETECTED"

	// RDTBin is the path to RDT detection helpers.
	RDTBin = "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery"
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

////////////////////////////////////////////////////////////////////////////////
// RDT (Intel Resource Director Technology) Source

type rdtSource struct{}

func (s rdtSource) Name() string { return "rdt" }

// Returns feature names for CMT, MBM and CAT if suppported.
func (s rdtSource) Discover() ([]string, error) {
	features := []string{}

	out, err := exec.Command("bash", "-c", path.Join(RDTBin, "mon-discovery")).Output()
	if err != nil {
		return nil, fmt.Errorf("can't detect support for RDT monitoring: %v", err)
	}
	if string(out[:]) == DETECTED {
		// RDT monitoring detected.
		features = append(features, "RDTMON")
	}

	out, err = exec.Command("bash", "-c", path.Join(RDTBin, "l3-alloc-discovery")).Output()
	if err != nil {
		return nil, fmt.Errorf("can't detect support for RDT L3 allocation: %v", err)
	}
	if string(out[:]) == DETECTED {
		// RDT L3 cache allocation detected.
		features = append(features, "RDTL3CA")
	}

	out, err = exec.Command("bash", "-c", path.Join(RDTBin, "l2-alloc-discovery")).Output()
	if err != nil {
		return nil, fmt.Errorf("can't detect support for RDT L2 allocation: %v", err)
	}
	if string(out[:]) == DETECTED {
		// RDT L2 cache allocation detected.
		features = append(features, "RDTL2CA")
	}

	return features, nil
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
