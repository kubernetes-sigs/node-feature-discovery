//go:build linux && s390x

/*
Copyright 2024 The Kubernetes Authors.

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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// cardAttrs is the list of sysfs attributes read from each card directory.
// https://github.com/torvalds/linux/blob/master/drivers/s390/crypto/ap_card.c
var cardAttrs = []string{"type", "online", "hwtype", "depth", "ap_functions", "config"}

// modeFromType extracts the operational mode from a CEX type string.
// CEX type strings end with a mode suffix: A=Accelerator, C=CCA, P=EP11.
func modeFromType(cexType string) string {
	if len(cexType) == 0 {
		return ""
	}
	switch cexType[len(cexType)-1] {
	case 'A':
		return "accelerator"
	case 'C':
		return "cca"
	case 'P':
		return "ep11"
	default:
		return ""
	}
}

// detectCexCards detects IBM CEX cryptographic cards on s390x systems.
// It scans /sys/bus/ap/devices/ for card entries and reads their attributes.
func detectCexCards() ([]nfdv1alpha1.InstanceFeature, error) {
	sysfsBasePath := hostpath.SysfsDir.Path("bus/ap/devices")

	if _, err := os.Stat(sysfsBasePath); os.IsNotExist(err) {
		klog.V(3).InfoS("AP bus not found, no CEX cards detected", "path", sysfsBasePath)
		return nil, nil
	}

	devices, err := os.ReadDir(sysfsBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list AP devices: %w", err)
	}

	cards := make([]nfdv1alpha1.InstanceFeature, 0)
	for _, device := range devices {
		deviceName := device.Name()

		if !strings.HasPrefix(deviceName, "card") {
			continue
		}

		cardPath := filepath.Join(sysfsBasePath, deviceName)
		cardInfo := readCexCardInfo(cardPath, deviceName)

		if len(cardInfo.Attributes) > 1 {
			cards = append(cards, *cardInfo)
		}
	}

	klog.V(3).InfoS("detected CEX cards", "count", len(cards))
	return cards, nil
}

// readCexCardInfo reads attributes for a single CEX card from its sysfs directory.
func readCexCardInfo(cardPath, cardName string) *nfdv1alpha1.InstanceFeature {
	attrs := map[string]string{"name": cardName}

	for _, attrName := range cardAttrs {
		attrPath := filepath.Join(cardPath, attrName)
		data, err := os.ReadFile(attrPath)
		if err != nil {
			klog.V(4).InfoS("failed to read card attribute", "card", cardName, "attribute", attrName, "error", err)
			continue
		}
		attrs[attrName] = strings.TrimSpace(string(data))
	}

	// Derive the operational mode from the type string
	if cardType, ok := attrs["type"]; ok {
		if mode := modeFromType(cardType); mode != "" {
			attrs["mode"] = mode
		}
	}

	queues, err := enumerateCardQueues(cardPath, cardName)
	if err != nil {
		klog.V(4).InfoS("failed to enumerate queues", "card", cardName, "error", err)
	} else if len(queues) > 0 {
		attrs["queue_count"] = fmt.Sprintf("%d", len(queues))
		attrs["queues"] = strings.Join(queues, ",")
	}

	return nfdv1alpha1.NewInstanceFeature(attrs)
}

// enumerateCardQueues finds AP queues (APQNs) associated with a card.
// Queue entries in sysfs have the format "<cardnum>.<domain>" (e.g., "00.0014").
func enumerateCardQueues(cardPath, cardName string) ([]string, error) {
	cardNum := strings.TrimPrefix(cardName, "card")

	parentPath := filepath.Dir(cardPath)
	devices, err := os.ReadDir(parentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read devices directory: %w", err)
	}

	queues := make([]string, 0)
	queuePrefix := cardNum + "."

	for _, device := range devices {
		deviceName := device.Name()
		if strings.HasPrefix(deviceName, queuePrefix) {
			queuePath := filepath.Join(parentPath, deviceName)
			if _, err := os.Stat(filepath.Join(queuePath, "online")); err == nil {
				queues = append(queues, deviceName)
			}
		}
	}

	return queues, nil
}
