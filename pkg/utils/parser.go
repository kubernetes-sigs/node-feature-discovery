/*
Copyright 2025 The Kubernetes Authors.

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

package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ExtractSemVer extracts semantic version from string (e.g., "1.2.3")
func ExtractSemVer(s string) (string, error) {
	var major, minor, patch int

	_, err := fmt.Sscanf(s, "%d.%d.%d", &major, &minor, &patch)
	if err == nil {
		return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
	}

	_, err = fmt.Sscanf(s, "%d.%d", &major, &minor)
	if err == nil {
		return fmt.Sprintf("%d.%d", major, minor), nil
	}

	_, err = fmt.Sscanf(s, "%d", &major)
	if err == nil {
		return fmt.Sprintf("%d", major), nil
	}

	return "", fmt.Errorf("unable to extract semantic version from value: %s", s)
}

// CompareVersions compare to semantic versions (e.g., "1.2.3")
// Returns:
// -1 if v1 < v2
// 0 if v1 == v2
// 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")

	maxLen := max(len(p1), len(p2))

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(p1) {
			num1, _ = strconv.Atoi(p1[i])
		}
		if i < len(p2) {
			num2, _ = strconv.Atoi(p2[i])
		}
		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}
	return 0
}
