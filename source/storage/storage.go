/*
Copyright 2018 The Kubernetes Authors.

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

package storage

import (
	"fmt"
	"io/ioutil"

	"github.com/kubernetes-incubator/node-feature-discovery/source"
)

// Source implements FeatureSource.
type Source struct{}

// Name returns an identifier string for this feature source.
func (s Source) Name() string { return "storage" }

// Discover returns feature names for storage: nonrotationaldisk if any SSD drive present.
func (s Source) Discover() (source.Features, error) {
	features := source.Features{}

	// Check if there is any non-rotational block devices attached to the node
	blockdevices, err := ioutil.ReadDir("/sys/block/")
	if err == nil {
		for _, bdev := range blockdevices {
			fname := "/sys/block/" + bdev.Name() + "/queue/rotational"
			bytes, err := ioutil.ReadFile(fname)
			if err != nil {
				return nil, fmt.Errorf("can't read rotational status: %s", err.Error())
			}
			if bytes[0] == byte('0') {
				// Non-rotational storage is present, add label.
				features["nonrotationaldisk"] = true
				break
			}
		}
	}
	return features, nil
}
