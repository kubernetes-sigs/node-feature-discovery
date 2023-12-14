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

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
)

var matchOps = map[MatchOp]struct{}{
	MatchAny:          {},
	MatchIn:           {},
	MatchNotIn:        {},
	MatchInRegexp:     {},
	MatchExists:       {},
	MatchDoesNotExist: {},
	MatchGt:           {},
	MatchLt:           {},
	MatchGtLt:         {},
	MatchIsTrue:       {},
	MatchIsFalse:      {},
}

// newMatchExpression returns a new MatchExpression instance.
func newMatchExpression(op MatchOp, values ...string) *MatchExpression {
	return &MatchExpression{
		Op:    op,
		Value: values,
	}
}

// Validate validates the expression.
func (m *MatchExpression) Validate() error {
	if _, ok := matchOps[m.Op]; !ok {
		return fmt.Errorf("invalid Op %q", m.Op)
	}
	switch m.Op {
	case MatchExists, MatchDoesNotExist, MatchIsTrue, MatchIsFalse, MatchAny:
		if len(m.Value) != 0 {
			return fmt.Errorf("value must be empty for Op %q (have %v)", m.Op, m.Value)
		}
	case MatchGt, MatchLt:
		if len(m.Value) != 1 {
			return fmt.Errorf("value must contain exactly one element for Op %q (have %v)", m.Op, m.Value)
		}
		if _, err := strconv.Atoi(m.Value[0]); err != nil {
			return fmt.Errorf("value must be an integer for Op %q (have %v)", m.Op, m.Value[0])
		}
	case MatchGtLt:
		if len(m.Value) != 2 {
			return fmt.Errorf("value must contain exactly two elements for Op %q (have %v)", m.Op, m.Value)
		}
		var err error
		v := make([]int, 2)
		for i := 0; i < 2; i++ {
			if v[i], err = strconv.Atoi(m.Value[i]); err != nil {
				return fmt.Errorf("value must contain integers for Op %q (have %v)", m.Op, m.Value)
			}
		}
		if v[0] >= v[1] {
			return fmt.Errorf("value[0] must be less than Value[1] for Op %q (have %v)", m.Op, m.Value)
		}
	case MatchInRegexp:
		if len(m.Value) == 0 {
			return fmt.Errorf("value must be non-empty for Op %q", m.Op)
		}
		for _, v := range m.Value {
			_, err := regexp.Compile(v)
			if err != nil {
				return fmt.Errorf("value must only contain valid regexps for Op %q (have %v)", m.Op, m.Value)
			}
		}
	default:
		if len(m.Value) == 0 {
			return fmt.Errorf("value must be non-empty for Op %q", m.Op)
		}
	}
	return nil
}

// Match evaluates the MatchExpression against a single input value.
func (m *MatchExpression) Match(valid bool, value interface{}) (bool, error) {
	if _, ok := matchOps[m.Op]; !ok {
		return false, fmt.Errorf("invalid Op %q", m.Op)
	}

	switch m.Op {
	case MatchAny:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return true, nil
	case MatchExists:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return valid, nil
	case MatchDoesNotExist:
		if len(m.Value) != 0 {
			return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
		}
		return !valid, nil
	}

	if valid {
		value := fmt.Sprintf("%v", value)
		switch m.Op {
		case MatchIn:
			if len(m.Value) == 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be non-empty for Op %q", m.Op)
			}
			for _, v := range m.Value {
				if value == v {
					return true, nil
				}
			}
		case MatchNotIn:
			if len(m.Value) == 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be non-empty for Op %q", m.Op)
			}
			for _, v := range m.Value {
				if value == v {
					return false, nil
				}
			}
			return true, nil
		case MatchInRegexp:
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
		case MatchGt, MatchLt:
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

			if (l < r && m.Op == MatchLt) || (l > r && m.Op == MatchGt) {
				return true, nil
			}
		case MatchGtLt:
			if len(m.Value) != 2 {
				return false, fmt.Errorf("invalid expression, value' field must contain exactly two elements for Op %q (have %v)", m.Op, m.Value)
			}
			v, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("not a number %q", value)
			}
			lr := make([]int, 2)
			for i := 0; i < 2; i++ {
				lr[i], err = strconv.Atoi(m.Value[i])
				if err != nil {
					return false, fmt.Errorf("not a number %q in %v", m.Value[i], m)
				}
			}
			if lr[0] >= lr[1] {
				return false, fmt.Errorf("invalid expression, value[0] must be less than Value[1] for Op %q (have %v)", m.Op, m.Value)
			}
			return v > lr[0] && v < lr[1], nil
		case MatchIsTrue:
			if len(m.Value) != 0 {
				return false, fmt.Errorf("invalid expression, 'value' field must be empty for Op %q (have %v)", m.Op, m.Value)
			}
			return value == "true", nil
		case MatchIsFalse:
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

// MatchKeys evaluates the MatchExpression against a set of keys.
func (m *MatchExpression) MatchKeys(name string, keys map[string]Nil) (bool, error) {
	matched := false

	_, ok := keys[name]
	switch m.Op {
	case MatchAny:
		matched = true
	case MatchExists:
		matched = ok
	case MatchDoesNotExist:
		matched = !ok
	default:
		return false, fmt.Errorf("invalid Op %q when matching keys", m.Op)
	}

	if klogV := klog.V(3); klogV.Enabled() {
		klogV.InfoS("matched keys", "matchResult", matched, "matchKey", name, "matchOp", m.Op)
	} else if klogV := klog.V(4); klogV.Enabled() {
		k := maps.Keys(keys)
		sort.Strings(k)
		klogV.InfoS("matched keys", "matchResult", matched, "matchKey", name, "matchOp", m.Op, "inputKeys", k)
	}
	return matched, nil
}

// MatchValues evaluates the MatchExpression against a set of key-value pairs.
func (m *MatchExpression) MatchValues(name string, values map[string]string) (bool, error) {
	v, ok := values[name]
	matched, err := m.Match(ok, v)
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

// matchExpression is a helper type for unmarshalling MatchExpression
type matchExpression MatchExpression

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json"
func (m *MatchExpression) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	switch v := (*raw).(type) {
	case string:
		*m = *newMatchExpression(MatchIn, v)
	case bool:
		*m = *newMatchExpression(MatchIn, strconv.FormatBool(v))
	case float64:
		*m = *newMatchExpression(MatchIn, strconv.FormatFloat(v, 'f', -1, 64))
	case []interface{}:
		values := make([]string, len(v))
		for i, value := range v {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid value %v in %v", value, v)
			}
			values[i] = str
		}
		*m = *newMatchExpression(MatchIn, values...)
	case map[string]interface{}:
		helper := &matchExpression{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		*m = *newMatchExpression(helper.Op, helper.Value...)
	default:
		return fmt.Errorf("invalid rule '%v' (%T)", v, v)
	}

	return m.Validate()
}

// MatchKeys evaluates the MatchExpressionSet against a set of keys.
func (m *MatchExpressionSet) MatchKeys(keys map[string]Nil) (bool, error) {
	matched, _, err := m.MatchGetKeys(keys)
	return matched, err
}

// MatchedElement holds one matched Instance.
// +k8s:deepcopy-gen=false
type MatchedElement map[string]string

// MatchGetKeys evaluates the MatchExpressionSet against a set of keys and
// returns all matched keys or nil if no match was found. Note that an empty
// MatchExpressionSet returns a match with an empty slice of matched features.
func (m *MatchExpressionSet) MatchGetKeys(keys map[string]Nil) (bool, []MatchedElement, error) {
	ret := make([]MatchedElement, 0, len(*m))

	for n, e := range *m {
		match, err := e.MatchKeys(n, keys)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
		ret = append(ret, MatchedElement{"Name": n})
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i]["Name"] < ret[j]["Name"] })
	return true, ret, nil
}

// MatchValues evaluates the MatchExpressionSet against a set of key-value pairs.
func (m *MatchExpressionSet) MatchValues(values map[string]string) (bool, error) {
	matched, _, err := m.MatchGetValues(values)
	return matched, err
}

// MatchGetValues evaluates the MatchExpressionSet against a set of key-value
// pairs and returns all matched key-value pairs. Note that an empty
// MatchExpressionSet returns a match with an empty slice of matched features.
func (m *MatchExpressionSet) MatchGetValues(values map[string]string) (bool, []MatchedElement, error) {
	ret := make([]MatchedElement, 0, len(*m))

	for n, e := range *m {
		match, err := e.MatchValues(n, values)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
		ret = append(ret, MatchedElement{"Name": n, "Value": values[n]})
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i]["Name"] < ret[j]["Name"] })
	return true, ret, nil
}

// MatchInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes).
func (m *MatchExpressionSet) MatchInstances(instances []InstanceFeature) (bool, error) {
	v, err := m.MatchGetInstances(instances)
	return len(v) > 0, err
}

// MatchGetInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes). A slice containing all matching instances is returned. An
// empty (non-nil) slice is returned if no matching instances were found.
func (m *MatchExpressionSet) MatchGetInstances(instances []InstanceFeature) ([]MatchedElement, error) {
	ret := []MatchedElement{}

	for _, i := range instances {
		if match, err := m.MatchValues(i.Attributes); err != nil {
			return nil, err
		} else if match {
			ret = append(ret, i.Attributes)
		}
	}
	return ret, nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchExpressionSet) UnmarshalJSON(data []byte) error {
	*m = MatchExpressionSet{}

	names := make([]string, 0)
	if err := json.Unmarshal(data, &names); err == nil {
		// Simplified slice form
		for _, name := range names {
			split := strings.SplitN(name, "=", 2)
			if len(split) == 1 {
				(*m)[split[0]] = newMatchExpression(MatchExists)
			} else {
				(*m)[split[0]] = newMatchExpression(MatchIn, split[1])
			}
		}
	} else {
		// Unmarshal the full map form
		expressions := make(map[string]*MatchExpression)
		if err := json.Unmarshal(data, &expressions); err != nil {
			return err
		}
		for k, v := range expressions {
			if v != nil {
				(*m)[k] = v
			} else {
				(*m)[k] = newMatchExpression(MatchExists)
			}
		}
	}

	return nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchOp) UnmarshalJSON(data []byte) error {
	var raw string

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if _, ok := matchOps[MatchOp(raw)]; !ok {
		return fmt.Errorf("invalid Op %q", raw)
	}
	*m = MatchOp(raw)
	return nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchValue) UnmarshalJSON(data []byte) error {
	var raw interface{}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case string:
		*m = []string{v}
	case bool:
		*m = []string{strconv.FormatBool(v)}
	case float64:
		*m = []string{strconv.FormatFloat(v, 'f', -1, 64)}
	case []interface{}:
		values := make([]string, len(v))
		for i, value := range v {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid value %v in %v", value, v)
			}
			values[i] = str
		}
		*m = values
	default:
		return fmt.Errorf("invalid values '%v' (%T)", v, v)
	}

	return nil
}
