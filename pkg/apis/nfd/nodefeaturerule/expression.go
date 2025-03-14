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

package nodefeaturerule

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	strings "strings"

	"maps"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

const (
	// MatchedKeyName is the name of the matched flag/attribute element.
	MatchedKeyName = "Name"
	// MatchedKeyValue is the value of the matched attribute element.
	MatchedKeyValue = "Value"
)

var matchOps = map[nfdv1alpha1.MatchOp]struct{}{
	nfdv1alpha1.MatchAny:          {},
	nfdv1alpha1.MatchIn:           {},
	nfdv1alpha1.MatchNotIn:        {},
	nfdv1alpha1.MatchInRegexp:     {},
	nfdv1alpha1.MatchExists:       {},
	nfdv1alpha1.MatchDoesNotExist: {},
	nfdv1alpha1.MatchGt:           {},
	nfdv1alpha1.MatchGe:           {},
	nfdv1alpha1.MatchLt:           {},
	nfdv1alpha1.MatchLe:           {},
	nfdv1alpha1.MatchGtLt:         {},
	nfdv1alpha1.MatchGeLe:         {},
	nfdv1alpha1.MatchIsTrue:       {},
	nfdv1alpha1.MatchIsFalse:      {},
}

// evaluateMatchExpression evaluates the MatchExpression against a single input value.
func evaluateMatchExpression(m *nfdv1alpha1.MatchExpression, valid bool, value interface{}) (bool, error) {
	if _, ok := matchOps[m.Op]; !ok {
		return false, fmt.Errorf("invalid Op %q", m.Op)
	}

	switch m.Op {
	case nfdv1alpha1.MatchAny:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return true, nil
	case nfdv1alpha1.MatchExists:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return valid, nil
	case nfdv1alpha1.MatchDoesNotExist:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return !valid, nil
	}

	if valid && value != nil {
		value := fmt.Sprintf("%v", value)
		switch m.Op {
		case nfdv1alpha1.MatchIn:
			if len(m.Value) == 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be non-empty for Op %q", m.Op)
			}
			for _, v := range m.Value {
				if value == v {
					return true, nil
				}
			}
		case nfdv1alpha1.MatchNotIn:
			if len(m.Value) == 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be non-empty for Op %q", m.Op)
			}
			for _, v := range m.Value {
				if value == v {
					return false, nil
				}
			}
			return true, nil
		case nfdv1alpha1.MatchInRegexp:
			if len(m.Value) == 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be non-empty for Op %q", m.Op)
			}
			valueRe := make([]*regexp.Regexp, len(m.Value))
			for i, v := range m.Value {
				re, err := regexp.Compile(v)
				if err != nil {
					return false, fmt.Errorf("invalid expressiom, 'value' field must only contain valid regexps for Op %q (have %v)", m.Op, m.Value)
				}
				valueRe[i] = re
			}
			for _, re := range valueRe {
				if re.MatchString(value) {
					return true, nil
				}
			}
		case nfdv1alpha1.MatchGt, nfdv1alpha1.MatchGe, nfdv1alpha1.MatchLt, nfdv1alpha1.MatchLe:
			if len(m.Value) != 1 {
				return false, fmt.Errorf("invalid expression, 'value' field must contain exactly one element for Op %q (have %v)", m.Op, m.Value)
			}

			l, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("not a number %q", value)
			}
			r, err := strconv.Atoi(m.Value[0])
			if err != nil {
				return false, fmt.Errorf("not a number %q in %v", m.Value[0], m)
			}

			if (l < r && m.Op == nfdv1alpha1.MatchLt) || (l <= r && m.Op == nfdv1alpha1.MatchLe) ||
				(l > r && m.Op == nfdv1alpha1.MatchGt) || (l >= r && m.Op == nfdv1alpha1.MatchGe) {
				return true, nil
			}
		case nfdv1alpha1.MatchGtLt, nfdv1alpha1.MatchGeLe:
			if len(m.Value) != 2 {
				return false, fmt.Errorf("invalid expression, value' field must contain exactly two elements for Op %q (have %v)", m.Op, m.Value)
			}
			v, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("not a number %q", value)
			}
			lr := make([]int, 2)
			for i := range 2 {
				lr[i], err = strconv.Atoi(m.Value[i])
				if err != nil {
					return false, fmt.Errorf("not a number %q in %v", m.Value[i], m)
				}
			}
			if lr[0] >= lr[1] {
				return false, fmt.Errorf("invalid expression, value[0] must be less than Value[1] for Op %q (have %v)", m.Op, m.Value)
			}
			return (v > lr[0] && v < lr[1] && m.Op == nfdv1alpha1.MatchGtLt) ||
				(v >= lr[0] && v <= lr[1] && m.Op == nfdv1alpha1.MatchGeLe), nil
		case nfdv1alpha1.MatchIsTrue:
			if len(m.Value) != 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
			}
			return value == "true", nil
		case nfdv1alpha1.MatchIsFalse:
			if len(m.Value) != 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
			}
			return value == "false", nil
		default:
			return false, fmt.Errorf("unsupported Op %q", m.Op)
		}
	}
	return false, nil
}

// evaluateMatchExpressionKeys evaluates the MatchExpression against a set of keys.
func evaluateMatchExpressionKeys(m *nfdv1alpha1.MatchExpression, name string, keys map[string]nfdv1alpha1.Nil) (bool, error) {
	_, ok := keys[name]
	matched, err := evaluateMatchExpression(m, ok, nil)
	if err != nil {
		return false, err
	}

	if klogV := klog.V(3); klogV.Enabled() {
		klogV.InfoS("matched keys", "matchResult", matched, "matchKey", name, "matchOp", m.Op)
	} else if klogV := klog.V(4); klogV.Enabled() {
		k := slices.Collect(maps.Keys(keys))
		sort.Strings(k)
		klogV.InfoS("matched keys", "matchResult", matched, "matchKey", name, "matchOp", m.Op, "inputKeys", k)
	}
	return matched, nil
}

// evaluateMatchExpressionValues evaluates the MatchExpression against a set of key-value pairs.
func evaluateMatchExpressionValues(m *nfdv1alpha1.MatchExpression, name string, values map[string]string) (bool, error) {
	v, ok := values[name]
	matched, err := evaluateMatchExpression(m, ok, v)
	if err != nil {
		return false, err
	}

	if klogV := klog.V(3); klogV.Enabled() {
		klogV.InfoS("matched values", "matchResult", matched, "matchKey", name, "matchOp", m.Op, "matchValue", m.Value)
	} else if klogV := klog.V(4); klogV.Enabled() {
		klogV.InfoS("matched values", "matchResult", matched, "matchKey", name, "matchOp", m.Op, "matchValue", m.Value, "inputValues", values)
	}

	return matched, nil
}

// MatchKeyNames evaluates the MatchExpression against names of a set of key features.
func MatchKeyNames(m *nfdv1alpha1.MatchExpression, keys map[string]nfdv1alpha1.Nil) (bool, []MatchedElement, error) {
	ret := []MatchedElement{}

	for k := range keys {
		if match, err := evaluateMatchExpression(m, true, k); err != nil {
			return false, nil, err
		} else if match {
			ret = append(ret, MatchedElement{MatchedKeyName: k})
		}
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i][MatchedKeyName] < ret[j][MatchedKeyName] })

	if klogV3 := klog.V(3); klogV3.Enabled() {
		mk := make([]string, len(ret))
		for i, v := range ret {
			mk[i] = v[MatchedKeyName]
		}
		mkMsg := strings.Join(mk, ", ")

		if klogV4 := klog.V(4); klogV4.Enabled() {
			k := make([]string, 0, len(keys))
			for n := range keys {
				k = append(k, n)
			}
			sort.Strings(k)
			klogV3.InfoS("matched (key) names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value, "inputKeys", k)
		} else {
			klogV3.InfoS("matched (key) names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value)
		}
	}

	return len(ret) > 0, ret, nil
}

// MatchValueNames evaluates the MatchExpression against names of a set of value features.
func MatchValueNames(m *nfdv1alpha1.MatchExpression, values map[string]string) (bool, []MatchedElement, error) {
	ret := []MatchedElement{}

	for k, v := range values {
		if match, err := evaluateMatchExpression(m, true, k); err != nil {
			return false, nil, err
		} else if match {
			ret = append(ret, MatchedElement{MatchedKeyName: k, MatchedKeyValue: v})
		}
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i][MatchedKeyName] < ret[j][MatchedKeyName] })

	if klogV3 := klog.V(3); klogV3.Enabled() {
		mk := make([]string, len(ret))
		for i, v := range ret {
			mk[i] = v[MatchedKeyName]
		}
		mkMsg := strings.Join(mk, ", ")

		if klogV4 := klog.V(4); klogV4.Enabled() {
			klogV3.InfoS("matched (value) names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value, "inputValues", values)
		} else {
			klogV3.InfoS("matched (value) names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value)
		}
	}

	return len(ret) > 0, ret, nil
}

// MatchInstanceAttributeNames evaluates the MatchExpression against a set of
// instance features, matching against the names of their attributes.
func MatchInstanceAttributeNames(m *nfdv1alpha1.MatchExpression, instances []nfdv1alpha1.InstanceFeature) (bool, []MatchedElement, error) {
	ret := []MatchedElement{}

	for _, i := range instances {
		if match, _, err := MatchValueNames(m, i.Attributes); err != nil {
			return false, nil, err
		} else if match {
			ret = append(ret, i.Attributes)
		}
	}
	return len(ret) > 0, ret, nil
}

// MatchKeys evaluates the MatchExpressionSet against a set of keys.
func MatchKeys(m *nfdv1alpha1.MatchExpressionSet, keys map[string]nfdv1alpha1.Nil) (bool, error) {
	matched, _, _, err := MatchGetKeys(m, keys)
	return matched, err
}

// MatchedElement holds one matched Instance.
type MatchedElement map[string]string

// MatchGetKeys evaluates the MatchExpressionSet against a set of keys and
// returns all matched keys or nil if no match was found. Note that an empty
// MatchExpressionSet returns a match with an empty slice of matched features.
func MatchGetKeys(m *nfdv1alpha1.MatchExpressionSet, keys map[string]nfdv1alpha1.Nil) (bool, []MatchedElement, *nfdv1alpha1.MatchExpressionSet, error) {
	matchedElements := make([]MatchedElement, 0, len(*m))
	matchedExpressions := make(nfdv1alpha1.MatchExpressionSet)

	for n, e := range *m {
		match, err := evaluateMatchExpressionKeys(e, n, keys)
		if err != nil {
			return false, nil, nil, err
		}
		if !match {
			return false, nil, nil, nil
		}
		matchedElements = append(matchedElements, MatchedElement{MatchedKeyName: n})
		matchedExpressions[n] = e
	}
	// Sort for reproducible output
	sort.Slice(matchedElements, func(i, j int) bool { return matchedElements[i][MatchedKeyName] < matchedElements[j][MatchedKeyName] })
	return true, matchedElements, &matchedExpressions, nil
}

// MatchValues evaluates the MatchExpressionSet against a set of key-value pairs.
func MatchValues(m *nfdv1alpha1.MatchExpressionSet, values map[string]string, failFast bool) (bool, *nfdv1alpha1.MatchExpressionSet, error) {
	matched, _, matchedExpressions, err := MatchGetValues(m, values, failFast)
	return matched, matchedExpressions, err
}

// MatchGetValues evaluates the MatchExpressionSet against a set of key-value
// pairs and returns all matched key-value pairs. Note that an empty
// MatchExpressionSet returns a match with an empty slice of matched features.
func MatchGetValues(m *nfdv1alpha1.MatchExpressionSet, values map[string]string, failFast bool) (bool, []MatchedElement, *nfdv1alpha1.MatchExpressionSet, error) {
	matchedElements := make([]MatchedElement, 0, len(*m))
	matchedExpressions := make(nfdv1alpha1.MatchExpressionSet)
	isMatch := true

	for n, e := range *m {
		match, err := evaluateMatchExpressionValues(e, n, values)
		if err != nil {
			return false, nil, nil, err
		}
		if match {
			matchedElements = append(matchedElements, MatchedElement{MatchedKeyName: n, MatchedKeyValue: values[n]})
			matchedExpressions[n] = e
		} else {
			if failFast {
				return false, nil, nil, nil
			}
			isMatch = false
		}
	}
	// Sort for reproducible output
	sort.Slice(matchedElements, func(i, j int) bool { return matchedElements[i][MatchedKeyName] < matchedElements[j][MatchedKeyName] })
	return isMatch, matchedElements, &matchedExpressions, nil
}

// MatchInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes).
func MatchInstances(m *nfdv1alpha1.MatchExpressionSet, instances []nfdv1alpha1.InstanceFeature, failFast bool) (bool, error) {
	isMatch, _, _, err := MatchGetInstances(m, instances, failFast)
	return isMatch, err
}

// MatchGetInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes). Returns a boolean that reports whether the expression matched.
// Also, returns a slice containing all matching instances. An empty (non-nil)
// slice is returned if no matching instances were found.
func MatchGetInstances(m *nfdv1alpha1.MatchExpressionSet, instances []nfdv1alpha1.InstanceFeature, failFast bool) (bool, []MatchedElement, *nfdv1alpha1.MatchExpressionSet, error) {
	var (
		match         bool
		err           error
		expressionSet *nfdv1alpha1.MatchExpressionSet
	)
	matchedElements := []MatchedElement{}
	matchedExpressions := &nfdv1alpha1.MatchExpressionSet{}

	for _, i := range instances {
		if match, expressionSet, err = MatchValues(m, i.Attributes, failFast); err != nil {
			return false, nil, nil, err
		} else if match {
			matchedElements = append(matchedElements, i.Attributes)
		}
		if expressionSet != nil {
			for name, exp := range *expressionSet {
				(*matchedExpressions)[name] = exp
			}
		}
	}
	return len(matchedElements) > 0, matchedElements, matchedExpressions, nil
}

// MatchMulti evaluates a MatchExpressionSet against key, value and instance
// features all at once. Key and values features are evaluated together so that
// a match in either (or both) of them is accepted as success. Instances are
// handled separately as the way of evaluating match expressions is different.
// This function is written to handle "multi-type" features where one feature
// (say "cpu.cpuid") contains multiple types (flag, attribute and/or instance).
func MatchMulti(m *nfdv1alpha1.MatchExpressionSet, keys map[string]nfdv1alpha1.Nil, values map[string]string, instances []nfdv1alpha1.InstanceFeature, failFast bool) (bool, []MatchedElement, *nfdv1alpha1.MatchExpressionSet, error) {
	matchedElems := []MatchedElement{}
	matchedExpressions := nfdv1alpha1.MatchExpressionSet{}
	isMatch := false

	// Keys and values are handled as a union, it is enough to find a match in
	// either of them
	if keys != nil || values != nil {
		// Handle the special case of empty match expression
		isMatch = true
	}
	for n, e := range *m {
		var (
			matchK bool
			matchV bool
			err    error
		)
		if keys != nil {
			matchK, err = evaluateMatchExpressionKeys(e, n, keys)
			if err != nil {
				return false, nil, nil, err
			}
			if matchK {
				matchedElems = append(matchedElems, MatchedElement{MatchedKeyName: n})
				matchedExpressions[n] = e
			} else if e.Op == nfdv1alpha1.MatchDoesNotExist {
				// DoesNotExist is special in that both "keys" and "values" should match (i.e. the name is not found in either of them).
				isMatch = false
				if !failFast {
					continue
				}
				matchedElems = []MatchedElement{}
				matchedExpressions = nfdv1alpha1.MatchExpressionSet{}
				break
			}
		}

		if values != nil {
			matchV, err = evaluateMatchExpressionValues(e, n, values)
			if err != nil {
				return false, nil, nil, err
			}
			if matchV {
				matchedElems = append(matchedElems, MatchedElement{MatchedKeyName: n, MatchedKeyValue: values[n]})
				matchedExpressions[n] = e
			} else if e.Op == nfdv1alpha1.MatchDoesNotExist {
				// DoesNotExist is special in that both "keys" and "values" should match (i.e. the name is not found in either of them).
				isMatch = false
				if !failFast {
					continue
				}
				matchedElems = []MatchedElement{}
				matchedExpressions = nfdv1alpha1.MatchExpressionSet{}
				break
			}
		}

		if !matchK && !matchV {
			isMatch = false
			if !failFast {
				continue
			}
			matchedElems = []MatchedElement{}
			matchedExpressions = nfdv1alpha1.MatchExpressionSet{}
			break
		}
	}
	// Sort for reproducible output
	sort.Slice(matchedElems, func(i, j int) bool { return matchedElems[i][MatchedKeyName] < matchedElems[j][MatchedKeyName] })

	// Instances are handled separately as the logic is fundamentally different
	// from keys and values and cannot be combined with them. We want to find
	// instance(s) that match all match expressions. I.e. the set of all match
	// expressions are evaluated against every instance separately.
	ma, melems, mexps, err := MatchGetInstances(m, instances, failFast)
	if err != nil {
		return false, nil, nil, err
	}
	isMatch = isMatch || ma
	matchedElems = append(matchedElems, melems...)
	if mexps != nil {
		for k, v := range *mexps {
			matchedExpressions[k] = v
		}
	}

	return isMatch, matchedElems, &matchedExpressions, nil
}

// MatchNamesMulti evaluates the MatchExpression against the names of key,
// value and attributes of instance features all at once. It is meant to handle
// "multi-type" features where one feature (say "cpu.cpuid") contains multiple
// types (flag, attribute and/or instance).
func MatchNamesMulti(m *nfdv1alpha1.MatchExpression, keys map[string]nfdv1alpha1.Nil, values map[string]string, instances []nfdv1alpha1.InstanceFeature) (bool, []MatchedElement, error) {
	ret := []MatchedElement{}
	for k := range keys {
		if match, err := evaluateMatchExpression(m, true, k); err != nil {
			return false, nil, err
		} else if match {
			ret = append(ret, MatchedElement{MatchedKeyName: k})
		}
	}

	for k, v := range values {
		if match, err := evaluateMatchExpression(m, true, k); err != nil {
			return false, nil, err
		} else if match {
			ret = append(ret, MatchedElement{MatchedKeyName: k, MatchedKeyValue: v})
		}
	}

	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i][MatchedKeyName] < ret[j][MatchedKeyName] })

	_, me, err := MatchInstanceAttributeNames(m, instances)
	if err != nil {
		return false, nil, err
	}
	ret = append(ret, me...)

	if klogV3 := klog.V(3); klogV3.Enabled() {
		mk := make([]string, len(ret))
		for i, v := range ret {
			mk[i] = v[MatchedKeyName]
		}
		mkMsg := strings.Join(mk, ", ")

		if klogV4 := klog.V(4); klogV4.Enabled() {
			k := make([]string, 0, len(keys))
			for n := range keys {
				k = append(k, n)
			}
			sort.Strings(k)
			klogV3.InfoS("matched names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value, "inputKeys", k)
		} else {
			klogV3.InfoS("matched names", "matchResult", mkMsg, "matchOp", m.Op, "matchValue", m.Value)
		}
	}

	return len(ret) > 0, ret, nil
}
