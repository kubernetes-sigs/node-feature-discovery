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

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	api "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
)

type BoolAssertionFunc func(assert.TestingT, bool, ...interface{}) bool

type ValueAssertionFunc func(assert.TestingT, interface{}, ...interface{}) bool

func TestMatchExpressionValidate(t *testing.T) {
	type V = api.MatchValue
	type TC struct {
		name   string
		op     api.MatchOp
		values V
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", op: api.MatchAny, err: assert.Nil}, // #0
		{name: "2", op: api.MatchAny, values: V{"1"}, err: assert.NotNil},

		{name: "3", op: api.MatchIn, err: assert.NotNil},
		{name: "4", op: api.MatchIn, values: V{"1"}, err: assert.Nil},
		{name: "5", op: api.MatchIn, values: V{"1", "2", "3", "4"}, err: assert.Nil},

		{name: "6", op: api.MatchNotIn, err: assert.NotNil},
		{name: "7", op: api.MatchNotIn, values: V{"1"}, err: assert.Nil},
		{name: "8", op: api.MatchNotIn, values: V{"1", "2"}, err: assert.Nil},

		{name: "9", op: api.MatchInRegexp, err: assert.NotNil},
		{name: "10", op: api.MatchInRegexp, values: V{"1"}, err: assert.Nil},
		{name: "11", op: api.MatchInRegexp, values: V{"()", "2", "3"}, err: assert.Nil},
		{name: "12", op: api.MatchInRegexp, values: V{"("}, err: assert.NotNil},

		{name: "13", op: api.MatchExists, err: assert.Nil},
		{name: "14", op: api.MatchExists, values: V{"1"}, err: assert.NotNil},

		{name: "15", op: api.MatchDoesNotExist, err: assert.Nil},
		{name: "16", op: api.MatchDoesNotExist, values: V{"1"}, err: assert.NotNil},

		{name: "17", op: api.MatchGt, err: assert.NotNil},
		{name: "18", op: api.MatchGt, values: V{"1"}, err: assert.Nil},
		{name: "19", op: api.MatchGt, values: V{"-10"}, err: assert.Nil},
		{name: "20", op: api.MatchGt, values: V{"1", "2"}, err: assert.NotNil},
		{name: "21", op: api.MatchGt, values: V{""}, err: assert.NotNil},

		{name: "22", op: api.MatchLt, err: assert.NotNil},
		{name: "23", op: api.MatchLt, values: V{"1"}, err: assert.Nil},
		{name: "24", op: api.MatchLt, values: V{"-1"}, err: assert.Nil},
		{name: "25", op: api.MatchLt, values: V{"1", "2", "3"}, err: assert.NotNil},
		{name: "26", op: api.MatchLt, values: V{"a"}, err: assert.NotNil},

		{name: "27", op: api.MatchGtLt, err: assert.NotNil},
		{name: "28", op: api.MatchGtLt, values: V{"1"}, err: assert.NotNil},
		{name: "29", op: api.MatchGtLt, values: V{"1", "2"}, err: assert.Nil},
		{name: "30", op: api.MatchGtLt, values: V{"2", "1"}, err: assert.NotNil},
		{name: "31", op: api.MatchGtLt, values: V{"1", "2", "3"}, err: assert.NotNil},
		{name: "32", op: api.MatchGtLt, values: V{"a", "2"}, err: assert.NotNil},

		{name: "33", op: api.MatchIsTrue, err: assert.Nil},
		{name: "34", op: api.MatchIsTrue, values: V{"1"}, err: assert.NotNil},

		{name: "35", op: api.MatchIsFalse, err: assert.Nil},
		{name: "36", op: api.MatchIsFalse, values: V{"1", "2"}, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := api.MatchExpression{Op: tc.op, Value: tc.values}
			err := me.Validate()
			tc.err(t, err)
		})
	}
}

func TestMatch(t *testing.T) {
	type V = api.MatchValue
	type TC struct {
		name   string
		op     api.MatchOp
		values V
		input  interface{}
		valid  bool
		result BoolAssertionFunc
	}

	tcs := []TC{
		{name: "MatchAny-1", op: api.MatchAny, result: assert.True},
		{name: "MatchAny-2", op: api.MatchAny, input: "2", valid: false, result: assert.True},

		{name: "MatchIn-1", op: api.MatchIn, values: V{"1"}, input: "2", valid: false, result: assert.False},
		{name: "MatchIn-2", op: api.MatchIn, values: V{"1"}, input: "2", valid: true, result: assert.False},
		{name: "MatchIn-3", op: api.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.False},
		{name: "MatchIn-4", op: api.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.True},

		{name: "MatchNotIn-1", op: api.MatchNotIn, values: V{"2"}, input: 2, valid: false, result: assert.False},
		{name: "MatchNotIn-2", op: api.MatchNotIn, values: V{"1"}, input: 2, valid: true, result: assert.True},
		{name: "MatchNotIn-3", op: api.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.False},
		{name: "MatchNotIn-4", op: api.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.False},

		{name: "MatchInRegexp-1", op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: false, result: assert.False},
		{name: "MatchInRegexp-2", op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: true, result: assert.True},
		{name: "MatchInRegexp-3", op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-12", valid: true, result: assert.False},
		{name: "MatchInRegexp-4", op: api.MatchInRegexp, values: V{"val-[0-9]$", "al-[1-9]"}, input: "val-12", valid: true, result: assert.True},

		{name: "MatchExists-1", op: api.MatchExists, input: nil, valid: false, result: assert.False},
		{name: "MatchExists-2", op: api.MatchExists, input: nil, valid: true, result: assert.True},

		{name: "MatchDoesNotExist-1", op: api.MatchDoesNotExist, input: false, valid: false, result: assert.True},
		{name: "MatchDoesNotExist-2", op: api.MatchDoesNotExist, input: false, valid: true, result: assert.False},

		{name: "MatchGt-1", op: api.MatchGt, values: V{"2"}, input: 3, valid: false, result: assert.False},
		{name: "MatchGt-2", op: api.MatchGt, values: V{"2"}, input: 2, valid: true, result: assert.False},
		{name: "MatchGt-3", op: api.MatchGt, values: V{"2"}, input: 3, valid: true, result: assert.True},
		{name: "MatchGt-4", op: api.MatchGt, values: V{"-10"}, input: -3, valid: true, result: assert.True},

		{name: "MatchLt-1", op: api.MatchLt, values: V{"2"}, input: "1", valid: false, result: assert.False},
		{name: "MatchLt-2", op: api.MatchLt, values: V{"2"}, input: "2", valid: true, result: assert.False},
		{name: "MatchLt-3", op: api.MatchLt, values: V{"-10"}, input: -3, valid: true, result: assert.False},
		{name: "MatchLt-4", op: api.MatchLt, values: V{"2"}, input: "1", valid: true, result: assert.True},

		{name: "MatchGtLt-1", op: api.MatchGtLt, values: V{"1", "10"}, input: "1", valid: false, result: assert.False},
		{name: "MatchGtLt-2", op: api.MatchGtLt, values: V{"1", "10"}, input: "1", valid: true, result: assert.False},
		{name: "MatchGtLt-3", op: api.MatchGtLt, values: V{"1", "10"}, input: "10", valid: true, result: assert.False},
		{name: "MatchGtLt-4", op: api.MatchGtLt, values: V{"1", "10"}, input: "2", valid: true, result: assert.True},

		{name: "MatchIsTrue-1", op: api.MatchIsTrue, input: true, valid: false, result: assert.False},
		{name: "MatchIsTrue-2", op: api.MatchIsTrue, input: true, valid: true, result: assert.True},
		{name: "MatchIsTrue-3", op: api.MatchIsTrue, input: false, valid: true, result: assert.False},

		{name: "MatchIsFalse-1", op: api.MatchIsFalse, input: "false", valid: false, result: assert.False},
		{name: "MatchIsFalse-2", op: api.MatchIsFalse, input: "false", valid: true, result: assert.True},
		{name: "MatchIsFalse-3", op: api.MatchIsFalse, input: "true", valid: true, result: assert.False},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := api.MatchExpression{Op: tc.op, Value: tc.values}
			res, err := me.Match(tc.valid, tc.input)
			tc.result(t, res)
			assert.Nil(t, err)
		})
	}

	// Error cases
	tcs = []TC{
		{name: "MatchAny-err-1", op: api.MatchAny, values: V{"1"}, input: "val"},

		{name: "MatchIn-err-1", op: api.MatchIn, input: "val"},

		{name: "MatchNotIn-err-1", op: api.MatchNotIn, input: "val"},

		{name: "MatchInRegexp-err-1", op: api.MatchInRegexp, input: "val"},
		{name: "MatchInRegexp-err-2", op: api.MatchInRegexp, values: V{"("}, input: "val"},

		{name: "MatchExists-err-1", op: api.MatchExists, values: V{"1"}},

		{name: "MatchDoesNotExist-err-1", op: api.MatchDoesNotExist, values: V{"1"}},

		{name: "MatchGt-err-1", op: api.MatchGt, input: "1"},
		{name: "MatchGt-err-2", op: api.MatchGt, values: V{"1", "2"}, input: "1"},
		{name: "MatchGt-err-3", op: api.MatchGt, values: V{""}, input: "1"},
		{name: "MatchGt-err-4", op: api.MatchGt, values: V{"2"}, input: "3a"},

		{name: "MatchLt-err-1", op: api.MatchLt, input: "1"},
		{name: "MatchLt-err-2", op: api.MatchLt, values: V{"1", "2", "3"}, input: "1"},
		{name: "MatchLt-err-3", op: api.MatchLt, values: V{"a"}, input: "1"},
		{name: "MatchLt-err-4", op: api.MatchLt, values: V{"2"}, input: "1.0"},

		{name: "MatchGtLt-err-1", op: api.MatchGtLt, input: "1"},
		{name: "MatchGtLt-err-2", op: api.MatchGtLt, values: V{"1"}, input: "1"},
		{name: "MatchGtLt-err-3", op: api.MatchGtLt, values: V{"2", "1"}, input: "1"},
		{name: "MatchGtLt-err-4", op: api.MatchGtLt, values: V{"1", "2", "3"}, input: "1"},
		{name: "MatchGtLt-err-5", op: api.MatchGtLt, values: V{"a", "2"}, input: "1"},
		{name: "MatchGtLt-err-6", op: api.MatchGtLt, values: V{"1", "10"}, input: "1.0"},

		{name: "MatchIsTrue-err-1", op: api.MatchIsTrue, values: V{"1"}, input: "true"},

		{name: "MatchIsFalse-err-1", op: api.MatchIsFalse, values: V{"1", "2"}, input: "false"},

		{name: "invalid-op-err", op: "non-existent-op", values: V{"1"}, input: 1},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := api.MatchExpression{Op: tc.op, Value: tc.values}
			res, err := me.Match(true, tc.input)
			assert.False(t, res)
			assert.NotNil(t, err)
		})
	}
}

func TestMatchKeys(t *testing.T) {
	type V = api.MatchValue
	type I = map[string]api.Nil
	type TC struct {
		name   string
		key    string
		op     api.MatchOp
		values V
		input  I
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", op: api.MatchAny, result: assert.True, err: assert.Nil},

		{name: "2", op: api.MatchExists, key: "foo", input: nil, result: assert.False, err: assert.Nil},
		{name: "3", op: api.MatchExists, key: "foo", input: I{"bar": {}}, result: assert.False, err: assert.Nil},
		{name: "4", op: api.MatchExists, key: "foo", input: I{"bar": {}, "foo": {}}, result: assert.True, err: assert.Nil},

		{name: "5", op: api.MatchDoesNotExist, key: "foo", input: nil, result: assert.True, err: assert.Nil},
		{name: "6", op: api.MatchDoesNotExist, key: "foo", input: I{}, result: assert.True, err: assert.Nil},
		{name: "7", op: api.MatchDoesNotExist, key: "foo", input: I{"bar": {}}, result: assert.True, err: assert.Nil},
		{name: "8", op: api.MatchDoesNotExist, key: "foo", input: I{"bar": {}, "foo": {}}, result: assert.False, err: assert.Nil},

		// All other ops should return an error
		{name: "9", op: api.MatchIn, values: V{"foo"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "10", op: api.MatchNotIn, values: V{"foo"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "11", op: api.MatchInRegexp, values: V{"foo"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "12", op: api.MatchGt, values: V{"1"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "13", op: api.MatchLt, values: V{"1"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "14", op: api.MatchGtLt, values: V{"1", "10"}, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "15", op: api.MatchIsTrue, key: "foo", result: assert.False, err: assert.NotNil},
		{name: "16", op: api.MatchIsFalse, key: "foo", result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := api.MatchExpression{Op: tc.op, Value: tc.values}
			res, err := me.MatchKeys(tc.key, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchValues(t *testing.T) {
	type V = []string
	type I = map[string]string

	type TC struct {
		name   string
		op     api.MatchOp
		values V
		key    string
		input  I
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", op: api.MatchAny, result: assert.True, err: assert.Nil},

		{name: "2", op: api.MatchIn, values: V{"1", "2"}, key: "foo", input: I{"bar": "2"}, result: assert.False, err: assert.Nil},
		{name: "3", op: api.MatchIn, values: V{"1", "2"}, key: "foo", input: I{"foo": "3"}, result: assert.False, err: assert.Nil},
		{name: "4", op: api.MatchIn, values: V{"1", "2"}, key: "foo", input: I{"foo": "2"}, result: assert.True, err: assert.Nil},

		{name: "5", op: api.MatchNotIn, values: V{"1", "2"}, key: "foo", input: I{"bar": "2"}, result: assert.False, err: assert.Nil},
		{name: "6", op: api.MatchNotIn, values: V{"1", "2"}, key: "foo", input: I{"foo": "3"}, result: assert.True, err: assert.Nil},
		{name: "7", op: api.MatchNotIn, values: V{"1", "2"}, key: "foo", input: I{"foo": "2"}, result: assert.False, err: assert.Nil},

		{name: "8", op: api.MatchInRegexp, values: V{"1", "2"}, key: "foo", input: I{"bar": "2"}, result: assert.False, err: assert.Nil},
		{name: "9", op: api.MatchInRegexp, values: V{"1", "[0-8]"}, key: "foo", input: I{"foo": "9"}, result: assert.False, err: assert.Nil},
		{name: "10", op: api.MatchInRegexp, values: V{"1", "[0-8]"}, key: "foo", input: I{"foo": "2"}, result: assert.True, err: assert.Nil},

		{name: "11", op: api.MatchExists, key: "foo", input: I{"bar": "1"}, result: assert.False, err: assert.Nil},
		{name: "12", op: api.MatchExists, key: "foo", input: I{"foo": "1"}, result: assert.True, err: assert.Nil},

		{name: "13", op: api.MatchDoesNotExist, key: "foo", input: nil, result: assert.True, err: assert.Nil},
		{name: "14", op: api.MatchDoesNotExist, key: "foo", input: I{"foo": "1"}, result: assert.False, err: assert.Nil},

		{name: "15", op: api.MatchGt, values: V{"2"}, key: "foo", input: I{"bar": "3"}, result: assert.False, err: assert.Nil},
		{name: "16", op: api.MatchGt, values: V{"2"}, key: "foo", input: I{"bar": "3", "foo": "2"}, result: assert.False, err: assert.Nil},
		{name: "17", op: api.MatchGt, values: V{"2"}, key: "foo", input: I{"bar": "3", "foo": "3"}, result: assert.True, err: assert.Nil},
		{name: "18", op: api.MatchGt, values: V{"2"}, key: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.False, err: assert.NotNil},

		{name: "19", op: api.MatchLt, values: V{"2"}, key: "foo", input: I{"bar": "1"}, result: assert.False, err: assert.Nil},
		{name: "20", op: api.MatchLt, values: V{"2"}, key: "foo", input: I{"bar": "1", "foo": "2"}, result: assert.False, err: assert.Nil},
		{name: "21", op: api.MatchLt, values: V{"2"}, key: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.True, err: assert.Nil},
		{name: "22", op: api.MatchLt, values: V{"2"}, key: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.False, err: assert.NotNil},

		{name: "23", op: api.MatchGtLt, values: V{"-10", "10"}, key: "foo", input: I{"bar": "1"}, result: assert.False, err: assert.Nil},
		{name: "24", op: api.MatchGtLt, values: V{"-10", "10"}, key: "foo", input: I{"bar": "1", "foo": "11"}, result: assert.False, err: assert.Nil},
		{name: "25", op: api.MatchGtLt, values: V{"-10", "10"}, key: "foo", input: I{"bar": "1", "foo": "-11"}, result: assert.False, err: assert.Nil},
		{name: "26", op: api.MatchGtLt, values: V{"-10", "10"}, key: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.True, err: assert.Nil},
		{name: "27", op: api.MatchGtLt, values: V{"-10", "10"}, key: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.False, err: assert.NotNil},

		{name: "28", op: api.MatchIsTrue, key: "foo", result: assert.False, err: assert.Nil},
		{name: "29", op: api.MatchIsTrue, key: "foo", input: I{"foo": "1"}, result: assert.False, err: assert.Nil},
		{name: "30", op: api.MatchIsTrue, key: "foo", input: I{"foo": "true"}, result: assert.True, err: assert.Nil},

		{name: "31", op: api.MatchIsFalse, key: "foo", input: I{"foo": "true"}, result: assert.False, err: assert.Nil},
		{name: "32", op: api.MatchIsFalse, key: "foo", input: I{"foo": "false"}, result: assert.True, err: assert.Nil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := api.MatchExpression{Op: tc.op, Value: tc.values}
			res, err := me.MatchValues(tc.key, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMESMatchKeys(t *testing.T) {
	type I = map[string]api.Nil
	type O = []api.MatchedElement
	type TC struct {
		name   string
		mes    string
		input  I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{output: O{}, result: assert.True, err: assert.Nil},

		{input: I{}, output: O{}, result: assert.True, err: assert.Nil},

		{input: I{"foo": {}}, output: O{}, result: assert.True, err: assert.Nil},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}, "buzz": {}},
			output: O{{"Name": "bar"}, {"Name": "foo"}},
			result: assert.True, err: assert.Nil},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"foo": {}, "bar": {}, "baz": {}},
			output: nil,
			result: assert.False, err: assert.Nil},

		{mes: `
foo: { op: In, value: ["bar"] }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}},
			output: nil,
			result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &api.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := mes.MatchGetKeys(tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = mes.MatchKeys(tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMESMatchValues(t *testing.T) {
	type I = map[string]string
	type O = []api.MatchedElement
	type TC struct {
		name   string
		mes    string
		input  I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", output: O{}, result: assert.True, err: assert.Nil},

		{name: "2", input: I{}, output: O{}, result: assert.True, err: assert.Nil},

		{name: "3", input: I{"foo": "bar"}, output: O{}, result: assert.True, err: assert.Nil},

		{name: "4",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"bar": "val"},
			result: assert.False, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123", "buzz": "light"},
			output: O{{"Name": "bar", "Value": "val"}, {"Name": "baz", "Value": "123"}, {"Name": "foo", "Value": "1"}},
			result: assert.True, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123.0"},
			result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &api.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := mes.MatchGetValues(tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = mes.MatchValues(tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMESMatchInstances(t *testing.T) {
	type I = api.InstanceFeature
	type O = []api.MatchedElement
	type A = map[string]string
	type TC struct {
		name   string
		mes    string
		input  []I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", output: O{}, result: assert.False, err: assert.Nil}, // nil instances -> false

		{name: "2", input: []I{}, output: O{}, result: assert.False, err: assert.Nil}, // zero instances -> false

		{name: "3", input: []I{I{Attributes: A{}}}, output: O{A{}}, result: assert.True, err: assert.Nil}, // one "empty" instance

		{name: "4",
			mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "1"}}},
			output: O{},
			result: assert.False, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"foo": "2", "bar": "1"}}},
			output: O{A{"foo": "2", "bar": "1"}},
			result: assert.True, err: assert.Nil},

		{name: "6",
			mes: `
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "0x1"}}},
			result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &api.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			out, err := mes.MatchGetInstances(tc.input)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err := mes.MatchInstances(tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}
