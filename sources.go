package main

import (
	"bytes"
	"fmt"
	"github.com/klauspost/cpuid"
	"io/ioutil"
	"net"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// FeatureSource represents a source of discovered node features.
type FeatureSource interface {
	// Returns a friendly name for this source of node features.
	Name() string

	// Returns discovered features for this node.
	Discover() ([]string, error)
}

const (
	// RDTBin is the path to RDT detection helpers.
	RDTBin = "/go/src/github.com/kubernetes-incubator/node-feature-discovery/rdt-discovery"

	// path to network interfaces in sysfs
	pathNet = "/sys/class/net/"

	// path suffix to obtain number of virtual functions for a network interface
	deviceSriovNumvfs = "/device/sriov_numvfs"
)

////////////////////////////////////////////////////////////////////////////////
// CPUID Source

// Implements main.FeatureSource.
type cpuidSource struct{}

func (s cpuidSource) Name() string { return "cpuid" }

// Returns feature names for all the supported CPU features.
func (s cpuidSource) Discover() ([]string, error) {
	// Get the cpu features as strings
	return cpuid.CPU.Features.Strings(), nil
}

////////////////////////////////////////////////////////////////////////////////
// RDT (Intel Resource Director Technology) Source

// Implements main.FeatureSource.
type rdtSource struct{}

func (s rdtSource) Name() string { return "rdt" }

// Returns feature names for CMT and CAT if suppported.
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
		// RDT L3 cache allocation detected.
		features = append(features, "RDTL3CA")
	}

	cmd = exec.Command("bash", "-c", path.Join(RDTBin, "l2-alloc-discovery"))
	if err := cmd.Run(); err != nil {
		stderrLogger.Printf("support for RDT L2 allocation was not detected: %s", err.Error())
	} else {
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

// Returns feature names for p-state related features such as turbo boost.
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

////////////////////////////////////////////////////////////////////////////////
// Network Source

// Implements main.FeatureSource.
type networkSource struct{}

func (s networkSource) Name() string { return "network" }

// reading the network card details from sysfs and determining if SR-IOV is enabled for each of the network interfaces
func (s networkSource) Discover() ([]string, error) {
	features := []string{}
	netInterfaces, err := net.Interfaces()

	if err != nil {
		return nil, fmt.Errorf("can't obtain the network interfaces from sysfs: %s", err.Error())
	}
	// iterating through network interfaces to obtain their respective number of virtual functions
	for _, netInterface := range netInterfaces {
		if strings.Contains(netInterface.Flags.String(), "up") {
			netInterfaceVfPath := pathNet + netInterface.Name + deviceSriovNumvfs
			bytes_received, err := ioutil.ReadFile(netInterfaceVfPath)
			if err != nil {
				stderrLogger.Printf("SR-IOV not supported for network interface: %s", netInterface.Name)
				continue
			}
			num := bytes.TrimSpace(bytes_received)
			n, err := strconv.Atoi(string(num))
			if err != nil {
				stderrLogger.Printf("Error in obtaining the number of virtual functions for network interface: %s", netInterface.Name)
				continue
			}
			if n > 0 {
				stdoutLogger.Printf("%d virtual functions configured on network interface: %s", n, netInterface.Name)
				features = append(features, "SRIOV")
				break
			}
		}
	}
	return features, nil
}
