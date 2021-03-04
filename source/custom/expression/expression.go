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

package expression

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
)

// MatchExpressionSet contains a set of MatchExpressions, each of which is
// evaluated against a set of input values.
type MatchExpressionSet map[string]*MatchExpression

// MatchExpression specifies an expression to evaluate against a set of input
// values. It contains an operator that is applied when matching the input and
// an array of values that the operator evaluates the input against.
// NB: CreateMatchExpression or MustCreateMatchExpression() should be used for
//     creating new instances.
// NB: Validate() must be called if Op or Value fields are modified or if a new
//     instance is created from scratch without using the helper functions.
type MatchExpression struct {
	// Op is the operator to be applied.
	Op MatchOp

	// Value is the list of values that the operand evaluates the input
	// against. Value should be empty if the operator is Exists, DoesNotExist,
	// IsTrue or IsFalse. Value should contain exactly one element if the
	// operator is Gt or Lt. In other cases Value should contain at least one
	// element.
	Value MatchValue `json:",omitempty"`

	// valueRe caches compiled regexps for "InRegexp" operator
	valueRe []*regexp.Regexp
}

// MatchOp is the match operator that is applied on values when evaluating a
// MatchExpression.
type MatchOp string

// MatchValue is the list of values associated with a MatchExpression.
type MatchValue []string

const (
	// MatchAny returns always true.
	MatchAny MatchOp = ""
	// MatchIn returns true if any of the values stored in the expression is
	// equal to the input.
	MatchIn MatchOp = "In"
	// MatchIn returns true if none of the values in the expression are equal
	// to the input.
	MatchNotIn MatchOp = "NotIn"
	// MatchInRegexp treats values of the expression as regular expressions and
	// returns true if any of them matches the input.
	MatchInRegexp MatchOp = "InRegexp"
	// MatchExists returns true if the input is valid. The expression must not
	// have any values.
	MatchExists MatchOp = "Exists"
	// MatchDoesNotExist returns true if the input is not valid. The expression
	// must not have any values.
	MatchDoesNotExist MatchOp = "DoesNotExist"
	// MatchGt returns true if the input is greater than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchGt MatchOp = "Gt"
	// MatchLt returns true if the input is less  than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchLt MatchOp = "Lt"
	// MatchIsTrue returns true if the input holds the value "true". The
	// expression must not have any values.
	MatchIsTrue MatchOp = "IsTrue"
	// MatchIsTrue returns true if the input holds the value "false". The
	// expression must not have any values.
	MatchIsFalse MatchOp = "IsFalse"
)

var matchOps = map[MatchOp]struct{}{
	MatchAny:          struct{}{},
	MatchIn:           struct{}{},
	MatchNotIn:        struct{}{},
	MatchInRegexp:     struct{}{},
	MatchExists:       struct{}{},
	MatchDoesNotExist: struct{}{},
	MatchGt:           struct{}{},
	MatchLt:           struct{}{},
	MatchIsTrue:       struct{}{},
	MatchIsFalse:      struct{}{},
}

// CreateMatchExpression creates a new MatchExpression instance. Returns an
// error if validation fails.
func CreateMatchExpression(op MatchOp, values ...string) (*MatchExpression, error) {
	m := newMatchExpression(op, values...)
	return m, m.Validate()
}

// MustCreateMatchExpression creates a new MatchExpression instance. Panics if
// validation fails.
func MustCreateMatchExpression(op MatchOp, values ...string) *MatchExpression {
	m, err := CreateMatchExpression(op, values...)
	if err != nil {
		panic(err)
	}
	return m
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
	m.valueRe = nil

	if _, ok := matchOps[m.Op]; !ok {
		return fmt.Errorf("invalid Op %q", m.Op)
	}
	switch m.Op {
	case MatchExists, MatchDoesNotExist, MatchIsTrue, MatchIsFalse, MatchAny:
		if len(m.Value) != 0 {
			return fmt.Errorf("Value must be empty for Op %q (have %v)", m.Op, m.Value)
		}
	case MatchGt, MatchLt:
		if len(m.Value) != 1 {
			return fmt.Errorf("Value must contain exactly one element for Op %q (have %v)", m.Op, m.Value)
		}
		if _, err := strconv.Atoi(m.Value[0]); err != nil {
			return fmt.Errorf("Value must be an integer for Op %q (have %v)", m.Op, m.Value[0])
		}
	case MatchInRegexp:
		if len(m.Value) == 0 {
			return fmt.Errorf("Value must be non-empty for Op %q", m.Op)
		}
		m.valueRe = make([]*regexp.Regexp, len(m.Value))
		for i, v := range m.Value {
			re, err := regexp.Compile(v)
			if err != nil {
				return fmt.Errorf("Value must only contain valid regexps for Op %q (have %v)", m.Op, m.Value)
			}
			m.valueRe[i] = re
		}
	default:
		if len(m.Value) == 0 {
			return fmt.Errorf("Value must be non-empty for Op %q", m.Op)
		}
	}
	return nil
}

// Match evaluates the MatchExpression against a single input value.
func (m *MatchExpression) Match(valid bool, value interface{}) (bool, error) {
	switch m.Op {
	case MatchAny:
		return true, nil
	case MatchExists:
		return valid, nil
	case MatchDoesNotExist:
		return !valid, nil
	}

	if valid {
		value := fmt.Sprintf("%v", value)
		switch m.Op {
		case MatchIn:
			for _, v := range m.Value {
				if value == v {
					return true, nil
				}
			}
		case MatchNotIn:
			for _, v := range m.Value {
				if value == v {
					return false, nil
				}
			}
			return true, nil
		case MatchInRegexp:
			if m.valueRe == nil {
				return false, fmt.Errorf("BUG: MatchExpression has not been initialized properly, regexps missing")
			}
			for _, re := range m.valueRe {
				if re.MatchString(value) {
					return true, nil
				}
			}
		case MatchGt, MatchLt:
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
		case MatchIsTrue:
			return value == "true", nil
		case MatchIsFalse:
			return value == "false", nil
		default:
			return false, fmt.Errorf("unsupported Op %q", m.Op)
		}
	}
	return false, nil
}

// MatchKeys evaluates the MatchExpression against a set of keys.
func (m *MatchExpression) MatchKeys(name string, keys map[string]feature.Nil) (bool, error) {
	klog.V(3).Infof("matching %q %q against %v", name, m.Op, keys)

	_, ok := keys[name]
	switch m.Op {
	case MatchAny:
		return true, nil
	case MatchExists:
		return ok, nil
	case MatchDoesNotExist:
		return !ok, nil
	default:
		return false, fmt.Errorf("invalid Op %q when matching keys", m.Op)
	}
}

// MatchValues evaluates the MatchExpression against a set of key-value pairs.
func (m *MatchExpression) MatchValues(name string, values map[string]string) (bool, error) {
	klog.V(3).Infof("matching %q %q %v against %v", name, m.Op, m.Value, values)
	v, ok := values[name]
	return m.Match(ok, v)
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
func (m *MatchExpressionSet) MatchKeys(keys map[string]feature.Nil) (bool, error) {
	for n, e := range *m {
		match, err := e.MatchKeys(n, keys)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// MatchValues evaluates the MatchExpressionSet against a set of key-value pairs.
func (m *MatchExpressionSet) MatchValues(values map[string]string) (bool, error) {
	for n, e := range *m {
		match, err := e.MatchValues(n, values)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// MatchInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes).
func (m *MatchExpressionSet) MatchInstances(instances []feature.InstanceFeature) (bool, error) {
	for _, i := range instances {
		if match, err := m.MatchValues(i.Attributes); err != nil {
			return false, err
		} else if match {
			return true, nil
		}
	}
	return false, nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchExpressionSet) UnmarshalJSON(data []byte) error {
	*m = make(MatchExpressionSet)

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
		} else {
			for k, v := range expressions {
				if v != nil {
					(*m)[k] = v
				} else {
					(*m)[k] = newMatchExpression(MatchExists)
				}
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
