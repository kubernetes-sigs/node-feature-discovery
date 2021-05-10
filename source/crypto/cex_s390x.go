// +build s390x

/*
Copyright 2021 The Kubernetes Authors

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

package crypto

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/node-feature-discovery/source"
)

type CEX struct {
	Name string
	Type string
}

func discoverCEX() ([]*CEX, error) {
	cards := []*CEX{}
	sysfsBasePath := source.SysfsDir.Path("bus/ap/devices")

	devices, err := ioutil.ReadDir(sysfsBasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*CEX{}, nil
		} else {
			return nil, err
		}
	}

	// Iterate over devices
	for _, device := range devices {
		if !strings.HasPrefix(device.Name(), "card") {
			continue
		}
		card, err := readCex(sysfsBasePath, device.Name())
		if err != nil {
			klog.Errorf("Failed to read Crypto Express %s: %v", device.Name(), err)
			continue
		}
		cards = append(cards, card)
	}

	return cards, nil
}

func readCex(basePath string, name string) (*CEX, error) {
	card := new(CEX)
	cardPath := path.Join(basePath, name)

	card.Name = name
	data, err := ioutil.ReadFile(cardPath + "/type")
	if err != nil {
		return nil, err
	}
	cardType := string(data)
	card.Type = strings.Trim(cardType, "\n")
	return card, nil
}
