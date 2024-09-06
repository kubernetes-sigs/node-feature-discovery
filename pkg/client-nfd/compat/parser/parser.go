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

package parser

import (
	"fmt"
	"strings"
)

func Parse(labelKey string, labelValue interface{}) (EntryGetter, error) {
	var err error
	entry := &CompatibilityEntry{
		keyRaw: labelKey,
		value: CompatibilityValue{
			raw: labelValue,
		},
	}

	entry.source, entry.feature, entry.option, err = parseKey(entry.keyRaw)
	if err != nil {
		return nil, err
	}

	entry.value.kind, entry.value.intervals, err = parseValue(entry.value.raw)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func parseKey(key string) (source, feature, option string, err error) {
	sourceSplit := strings.SplitN(key, "-", 2)
	if len(sourceSplit) <= 1 {
		return "", "", "", fmt.Errorf("invalid label %q", key)
	}
	source = sourceSplit[0]

	featureSplit := strings.SplitN(sourceSplit[1], ".", 2)
	if len(featureSplit) <= 1 {
		return "", "", "", fmt.Errorf("invalid feature %q -> %q", key, sourceSplit[1])
	}
	feature = featureSplit[0]
	option = featureSplit[1]

	return
}

func parseValue(value interface{}) (valueType ValueType, intervals []string, err error) {
	switch value := value.(type) {
	case bool:
		valueType = LiteralValueType
		return
	case string:
		vals := strings.Split(value, ";")
		if len(vals) > 1 || (strings.HasPrefix(value, ">") || strings.HasPrefix(value, "<")) {
			valueType = RangeValueType
			intervals = vals
		} else {
			valueType = LiteralValueType
		}
		return
	default:
		err = fmt.Errorf("unspported value type: %T. Type must be either string or boolean", value)
		return
	}
}
