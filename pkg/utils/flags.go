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

package utils

import (
	"regexp"
	"sort"
	"strings"
)

// RegexpVal is a wrapper for regexp command line flags
type RegexpVal struct {
	regexp.Regexp
}

// Set implements the flag.Value interface
func (a *RegexpVal) Set(val string) error {
	r, err := regexp.Compile(val)
	a.Regexp = *r
	return err
}

// StringSetVal is a Value encapsulating a set of comma-separated strings
type StringSetVal map[string]struct{}

// Set implements the flag.Value interface
func (a *StringSetVal) Set(val string) error {
	m := map[string]struct{}{}
	for _, n := range strings.Split(val, ",") {
		m[n] = struct{}{}
	}
	*a = m
	return nil
}

// String implements the flag.Value interface
func (a *StringSetVal) String() string {
	if *a == nil {
		return ""
	}

	vals := make([]string, len(*a), 0)
	for val := range *a {
		vals = append(vals, val)
	}
	sort.Strings(vals)
	return strings.Join(vals, ",")
}
