/*
Copyright 2019 The Kubernetes Authors.

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

package cpu_power

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"sigs.k8s.io/node-feature-discovery/source"
)

var (
	cpuinfo     = "/host-proc/cpuinfo"
	sysdevcpu   = "/host-sys/devices/system/cpu/cpu"
	cpubasefreq = "/cpufreq/base_frequency"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "cpu_power" }

type NoBasefreqSupport struct {
	message string
}

func (b *NoBasefreqSupport) Error() string {
	return string(b.message)
}

func getcpuBaseFrequency() int {
	baseFreq := DiscoverBaseFreq()
	return int(baseFreq)
}

func getcpuCount() (int, error) {
	files, err := ioutil.ReadDir("/host-sys/devices/system/cpu")
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile("cpu([0-9])")
	var i int
	for _, f := range files {
		if re.MatchString(f.Name()) {
			i++
		}
	}

	return i, nil
}

func getBaseFrequency(cpu int) (int, error) {
	var baseFreq int

	baseFile := fmt.Sprintf("%s%d%s", sysdevcpu, cpu, cpubasefreq)
	if _, err := os.Lstat(baseFile); err != nil {
		return baseFreq, &NoBasefreqSupport{"no base frequency support"}
	}

	data, err := ioutil.ReadFile(baseFile)
	if err != nil {
		return baseFreq, fmt.Errorf("failed to read the %q err: %v", baseFile, err)
	}

	if len(data) == 0 {
		return baseFreq, fmt.Errorf("no data in the file %q", baseFile)
	}

	rawbaseFreq := strings.TrimSpace(string(data))
	baseFreq, err = strconv.Atoi(rawbaseFreq)
	if err != nil {
		return baseFreq, fmt.Errorf("failed to convert baseFreqs(byte value) of %q: %v", baseFile, err)
	}

	baseFreq = baseFreq / 1000
	return baseFreq, nil
}

func checkpbfCores() (bool, error) {
	var pbfenabled bool

	cpucount, err := getcpuCount()
	if err != nil {
		return pbfenabled, err
	}

	P1 := getcpuBaseFrequency()

	for cpuid := 0; cpuid < cpucount; cpuid++ {
		base, err := getBaseFrequency(cpuid)
		if err != nil {
			return pbfenabled, err
		}

		if base > P1 {
			pbfenabled = true
			break
		}
	}

	return pbfenabled, nil
}

// Discover returns feature names for pbf related features
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	pbfenabled, err := checkpbfCores()
	if err != nil {
		if _, ok := err.(*NoBasefreqSupport); ok {
			features["sst_bf.enabled"] = false
		} else {
			return nil, fmt.Errorf("can't detect whether pbf is enabled: %s", err.Error())
		}
	}

	if pbfenabled != false {
		features["sst_bf.enabled"] = true
	}

	return features, nil
}
