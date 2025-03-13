/*
Copyright 2023 The Kubernetes Authors.

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

package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
		for i := range 2 {
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
