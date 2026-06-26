/*
Copyright 2026 The Kubernetes Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSliceValSet(t *testing.T) {
	cases := []struct {
		name string
		val  string
		want StringSliceVal
	}{
		{name: "simple", val: "cpu,pci", want: StringSliceVal{"cpu", "pci"}},
		{name: "surrounding whitespace is trimmed", val: "cpu, pci", want: StringSliceVal{"cpu", "pci"}},
		{name: "whitespace around every entry is trimmed", val: " pci.device , usb.* ", want: StringSliceVal{"pci.device", "usb.*"}},
		{name: "blank entries are dropped", val: "pci.device,,usb.*", want: StringSliceVal{"pci.device", "usb.*"}},
		{name: "single entry", val: "cpu", want: StringSliceVal{"cpu"}},
		{name: "empty string yields an empty slice", val: "", want: nil},
		{name: "whitespace-only yields an empty slice", val: "  ", want: nil},
		{name: "only blanks and whitespace yields an empty slice", val: ", ,\t,", want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got StringSliceVal
			assert.NoError(t, got.Set(tc.val))
			assert.Equal(t, tc.want, got)
		})
	}
}
