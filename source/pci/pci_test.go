/*
Copyright 2021 The Kubernetes Authors.

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

package pci

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

var packagePath string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)
}

func TestSingletonPciSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err, err)
	assert.Empty(t, l)
}

func TestPciSource(t *testing.T) {
	// create a reader from the tarball
	tarballPath := filepath.Join(packagePath, "testdata.tar.gz")
	tarball, err := os.Open(tarballPath)
	assert.Nil(t, err, err)
	defer tarball.Close()

	err = untar(tarball, packagePath)
	assert.Nil(t, err, err)

	// Specify expected "raw" features. These are always the same for the same
	// mocked sysfs.
	expectedFeatures := map[string]*nfdv1alpha1.Features{
		"rootfs-empty": {
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances:  map[string]nfdv1alpha1.InstanceFeatureSet{},
		},
		"rootfs-1": {
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
				"device": {
					Elements: []nfdv1alpha1.InstanceFeature{
						{
							Attributes: map[string]string{
								"class":            "0880",
								"device":           "2021",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "ff00",
								"device":           "a1ed",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0106",
								"device":           "a1d2",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "1180",
								"device":           "a1b1",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0780",
								"device":           "a1ba",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0604",
								"device":           "a193",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0c80",
								"device":           "a1a4",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0300",
								"device":           "2000",
								"subsystem_device": "2000",
								"subsystem_vendor": "1a03",
								"vendor":           "1a03",
							},
						},
						{
							Attributes: map[string]string{
								"class":                     "0b40",
								"device":                    "37c8",
								"iommu/intel-iommu/version": "1:0",
								"iommu_group/type":          "identity",
								"sriov_totalvfs":            "16",
								"subsystem_device":          "35cf",
								"subsystem_vendor":          "8086",
								"vendor":                    "8086",
							},
						},
						{
							Attributes: map[string]string{
								"class":            "0200",
								"device":           "37d2",
								"sriov_totalvfs":   "32",
								"subsystem_device": "35cf",
								"subsystem_vendor": "8086",
								"vendor":           "8086",
							},
						},
					},
				},
			},
		},
	}

	// Specify test cases
	tests := []struct {
		name           string
		config         *Config
		rootfs         string
		expectErr      bool
		expectedLabels source.FeatureLabels
	}{
		{
			name:   "detect with default config",
			rootfs: "rootfs-1",
			expectedLabels: source.FeatureLabels{
				"0300_1a03.present":       true,
				"0b40_8086.present":       true,
				"0b40_8086.sriov.capable": true,
			},
		},
		{
			name:   "test config with empty DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with empty DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with only invalid DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{"foo", "bar"},
			},
			expectedLabels: source.FeatureLabels{
				"0c80_8086.present": true,
			},
		},
		{
			name:   "test config with some invalid DeviceLabelFields",
			rootfs: "rootfs-1",
			config: &Config{
				DeviceClassWhitelist: []string{"0c"},
				DeviceLabelFields:    []string{"foo", "class"},
			},
			expectedLabels: source.FeatureLabels{
				"0c80.present": true,
			},
		},
		{
			name:           "test empty sysfs",
			rootfs:         "rootfs-empty",
			expectErr:      true,
			expectedLabels: source.FeatureLabels{},
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hostpath.SysfsDir = hostpath.HostDir(filepath.Join(packagePath, "testdata", tc.rootfs, "sys"))

			config := tc.config
			if config == nil {
				config = newDefaultConfig()
			}
			testSrc := pciSource{config: config}

			// Discover mock PCI devices
			err := testSrc.Discover()
			if tc.expectErr {
				assert.NotNil(t, err, err)
			} else {
				assert.Nil(t, err, err)
			}

			// Check features
			f := testSrc.GetFeatures()
			assert.Equal(t, expectedFeatures[tc.rootfs], f)

			// Check labels
			l, err := testSrc.GetLabels()
			assert.Nil(t, err, err)
			assert.Equal(t, tc.expectedLabels, l)
		})
	}
}

// untar reads the gzip-compressed tar file from r and writes it into dir.
// from golang.org/x/build/internal/untar/internal/untar/untar.go
// TODO: remove this function once the upstream untar package is published
func untar(r io.Reader, dir string) (err error) {
	t0 := time.Now()
	nFiles := 0
	madeDir := map[string]bool{}
	defer func() {
		td := time.Since(t0)
		if err == nil {
			log.Printf("extracted tarball into %s: %d files, %d dirs (%v)", dir, nFiles, len(madeDir), td)
		} else {
			log.Printf("error extracting tarball into %s after %d files, %d dirs, %v: %v", dir, nFiles, len(madeDir), td, err)
		}
	}()
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("requires gzip-compressed body: %v", err)
	}
	tr := tar.NewReader(zr)
	loggedChtimesError := false
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("tar reading error: %v", err)
			return fmt.Errorf("tar error: %v", err)
		}
		if !validRelPath(f.Name) {
			return fmt.Errorf("tar contained invalid name error %q", f.Name)
		}
		rel := filepath.FromSlash(f.Name)
		abs := filepath.Join(dir, rel)

		mode := f.FileInfo().Mode()
		switch f.Typeflag {
		case tar.TypeReg:
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			if runtime.GOOS == "darwin" && mode&0111 != 0 {
				err := os.Remove(abs)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			n, err := io.Copy(wf, tr)
			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			modTime := f.ModTime
			if modTime.After(t0) {
				modTime = t0
			}
			if !modTime.IsZero() {
				if err := os.Chtimes(abs, modTime, modTime); err != nil && !loggedChtimesError {
					log.Printf("error changing modtime: %v (further Chtimes errors suppressed)", err)
					loggedChtimesError = true // once is enough
				}
			}
			nFiles++
		case tar.TypeDir:
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			madeDir[abs] = true
		case tar.TypeXGlobalHeader:
			// git archive generates these. Ignore them.
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}
