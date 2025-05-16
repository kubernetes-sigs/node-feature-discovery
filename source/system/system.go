/*
Copyright 2018-2021 The Kubernetes Authors.

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

package system

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

var osReleaseFields = [...]string{
	"ID",
	"VERSION_ID",
	"VERSION_ID.major",
	"VERSION_ID.minor",
}

// Name of this feature source
const Name = "system"

const (
	OsReleaseFeature = "osrelease"
	NameFeature      = "name"
	DmiIdFeature     = "dmiid"
	LibcFeature      = "libc"
)

// systemSource implements the FeatureSource and LabelSource interfaces.
type systemSource struct {
	features *nfdv1alpha1.Features
}

// Singleton source instance
var (
	src systemSource
	_   source.FeatureSource = &src
	_   source.LabelSource   = &src
)

func (s *systemSource) Name() string { return Name }

// Priority method of the LabelSource interface
func (s *systemSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *systemSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for _, key := range osReleaseFields {
		if value, exists := features.Attributes[OsReleaseFeature].Elements[key]; exists {
			feature := "os_release." + key
			labels[feature] = value
		}
	}
	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *systemSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()

	// Get node name
	s.features.Attributes[NameFeature] = nfdv1alpha1.NewAttributeFeatures(nil)
	s.features.Attributes[NameFeature].Elements["nodename"] = utils.NodeName()

	// Get os-release information
	release, err := parseOSRelease()
	if err != nil {
		klog.ErrorS(err, "failed to get os-release")
	} else {
		s.features.Attributes[OsReleaseFeature] = nfdv1alpha1.NewAttributeFeatures(release)

		if v, ok := release["VERSION_ID"]; ok {
			versionComponents := splitVersion(v)
			for subKey, subValue := range versionComponents {
				if subValue != "" {
					s.features.Attributes[OsReleaseFeature].Elements["VERSION_ID."+subKey] = subValue
				}
			}
		}
	}

	// Get DMI ID attributes
	dmiIDAttributeNames := []string{"sys_vendor", "product_name"}
	dmiAttrs := make(map[string]string)
	for _, name := range dmiIDAttributeNames {
		val, err := getDmiIDAttribute(name)
		if err != nil {
			klog.ErrorS(err, "failed to get DMI entry", "attributeName", name)
		} else {
			dmiAttrs[name] = val
		}
	}

	if len(dmiAttrs) > 0 {
		s.features.Attributes[DmiIdFeature] = nfdv1alpha1.NewAttributeFeatures(dmiAttrs)
	}

	libcAttrs, err := getLibcAttributes()
	if err != nil {
		klog.ErrorS(err, "failed to detect libc version")
	} else {
		s.features.Attributes[LibcFeature] = nfdv1alpha1.NewAttributeFeatures(libcAttrs)
	}

	klog.V(3).InfoS("discovered features", "featureSource", s.Name(), "features", utils.DelayedDumper(s.features))

	return nil
}

// GetFeatures method of the FeatureSource Interface
func (s *systemSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

// Read and parse os-release file
func parseOSRelease() (map[string]string, error) {
	release := map[string]string{}

	f, err := os.Open(hostpath.EtcDir.Path("os-release"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^(?P<key>\w+)=(?P<value>.+)`)

	// Read line-by-line
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			release[m[1]] = strings.Trim(m[2], `"'`)
		}
	}

	return release, nil
}

// Split version number into sub-components. Verifies that they are numerical
// so that they can be fully utilized in k8s nodeAffinity
func splitVersion(version string) map[string]string {
	components := map[string]string{}
	// Currently, split out just major and minor version
	re := regexp.MustCompile(`^(?P<major>\d+)(\.(?P<minor>\d+))?(\..*)?$`)
	if m := re.FindStringSubmatch(version); m != nil {
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				components[name] = m[i]
			}
		}
	}
	return components
}

// Read /sys/devices/virtual/dmi/id attribute
func getDmiIDAttribute(name string) (string, error) {
	s, err := os.ReadFile(hostpath.SysfsDir.Path("devices/virtual/dmi/id/", name))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(s)), nil
}

func init() {
	source.Register(&src)
}
