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
	"testing"

	"github.com/stretchr/testify/assert"
)

type BoolAssertionFunc func(assert.TestingT, bool, ...interface{}) bool

type ValueAssertionFunc func(assert.TestingT, interface{}, ...interface{}) bool

func TestMatchExpressionValidate(t *testing.T) {
	type V = MatchValue
	type TC struct {
		name   string
		op     MatchOp
		values V
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", op: MatchAny, err: assert.Nil}, // #0
		{name: "2", op: MatchAny, values: V{"1"}, err: assert.NotNil},

		{name: "3", op: MatchIn, err: assert.NotNil},
		{name: "4", op: MatchIn, values: V{"1"}, err: assert.Nil},
		{name: "5", op: MatchIn, values: V{"1", "2", "3", "4"}, err: assert.Nil},

		{name: "6", op: MatchNotIn, err: assert.NotNil},
		{name: "7", op: MatchNotIn, values: V{"1"}, err: assert.Nil},
		{name: "8", op: MatchNotIn, values: V{"1", "2"}, err: assert.Nil},

		{name: "9", op: MatchInRegexp, err: assert.NotNil},
		{name: "10", op: MatchInRegexp, values: V{"1"}, err: assert.Nil},
		{name: "11", op: MatchInRegexp, values: V{"()", "2", "3"}, err: assert.Nil},
		{name: "12", op: MatchInRegexp, values: V{"("}, err: assert.NotNil},

		{name: "13", op: MatchExists, err: assert.Nil},
		{name: "14", op: MatchExists, values: V{"1"}, err: assert.NotNil},

		{name: "15", op: MatchDoesNotExist, err: assert.Nil},
		{name: "16", op: MatchDoesNotExist, values: V{"1"}, err: assert.NotNil},

		{name: "17", op: MatchGt, err: assert.NotNil},
		{name: "18", op: MatchGt, values: V{"1"}, err: assert.Nil},
		{name: "19", op: MatchGt, values: V{"-10"}, err: assert.Nil},
		{name: "20", op: MatchGt, values: V{"1", "2"}, err: assert.NotNil},
		{name: "21", op: MatchGt, values: V{""}, err: assert.NotNil},

		{name: "22", op: MatchLt, err: assert.NotNil},
		{name: "23", op: MatchLt, values: V{"1"}, err: assert.Nil},
		{name: "24", op: MatchLt, values: V{"-1"}, err: assert.Nil},
		{name: "25", op: MatchLt, values: V{"1", "2", "3"}, err: assert.NotNil},
		{name: "26", op: MatchLt, values: V{"a"}, err: assert.NotNil},

		{name: "27", op: MatchGtLt, err: assert.NotNil},
		{name: "28", op: MatchGtLt, values: V{"1"}, err: assert.NotNil},
		{name: "29", op: MatchGtLt, values: V{"1", "2"}, err: assert.Nil},
		{name: "30", op: MatchGtLt, values: V{"2", "1"}, err: assert.NotNil},
		{name: "31", op: MatchGtLt, values: V{"1", "2", "3"}, err: assert.NotNil},
		{name: "32", op: MatchGtLt, values: V{"a", "2"}, err: assert.NotNil},

		{name: "33", op: MatchIsTrue, err: assert.Nil},
		{name: "34", op: MatchIsTrue, values: V{"1"}, err: assert.NotNil},

		{name: "35", op: MatchIsFalse, err: assert.Nil},
		{name: "36", op: MatchIsFalse, values: V{"1", "2"}, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			me := MatchExpression{Op: tc.op, Value: tc.values}
			err := me.Validate()
			tc.err(t, err)
		})
	}
}

func TestUnmarshalMatchExpressionSet(t *testing.T) {
	type TC struct {
		name string
		data string
		out  MatchExpressionSet
		err  ValueAssertionFunc
	}

	tcs := []TC{
		{
			name: "empty",
			data: "{}",
			out:  MatchExpressionSet{},
			err:  assert.Nil,
		},
		{
			name: "multiple expressions",
			data: "{}",
			out:  MatchExpressionSet{},
			err:  assert.Nil,
		},
		{
			name: "multiple expressions",
			data: `{
"key-1":{"op":"Exists"},
"key-2":{"op":"DoesNotExist"},
"key-3":{"op":"IsTrue"},
"key-4":{"op":"IsFalse"},
"key-5":{"op":"In","value":["str","true"]},
"key-6":{"op":"InRegexp","value":["^foo$"]},
"key-7":{"op":"Lt","value":1},
"key-8":{"op":"Gt","value":2},
"key-9":{"op":"GtLt","value":["0","3"]}}`,
			out: MatchExpressionSet{
				"key-1": &MatchExpression{Op: MatchExists},
				"key-2": &MatchExpression{Op: MatchDoesNotExist},
				"key-3": &MatchExpression{Op: MatchIsTrue},
				"key-4": &MatchExpression{Op: MatchIsFalse},
				"key-5": &MatchExpression{Op: MatchIn, Value: MatchValue{"str", "true"}},
				"key-6": &MatchExpression{Op: MatchInRegexp, Value: MatchValue{"^foo$"}},
				"key-7": &MatchExpression{Op: MatchLt, Value: MatchValue{"1"}},
				"key-8": &MatchExpression{Op: MatchGt, Value: MatchValue{"2"}},
				"key-9": &MatchExpression{Op: MatchGtLt, Value: MatchValue{"0", "3"}},
			},
			err: assert.Nil,
		},
		{
			name: "special values",
			data: `{
"key-1":{"op":"In","value":"str"},
"key-2":{"op":"In","value":true},
"key-3":{"op":"In","value":1.23}}`,
			out: MatchExpressionSet{
				"key-1": &MatchExpression{Op: MatchIn, Value: MatchValue{"str"}},
				"key-2": &MatchExpression{Op: MatchIn, Value: MatchValue{"true"}},
				"key-3": &MatchExpression{Op: MatchIn, Value: MatchValue{"1.23"}},
			},
			err: assert.Nil,
		},
		{
			name: "shortform array",
			data: `["key-1","key-2=val-2"]`,
			out: MatchExpressionSet{
				"key-1": &MatchExpression{Op: MatchExists},
				"key-2": &MatchExpression{Op: MatchIn, Value: MatchValue{"val-2"}},
			},
			err: assert.Nil,
		},
		{
			name: "shortform string",
			data: `{"key":"value"}`,
			out: MatchExpressionSet{
				"key": &MatchExpression{Op: MatchIn, Value: MatchValue{"value"}},
			},
			err: assert.Nil,
		},
		{
			name: "Lt nan error",
			data: `{"key-7":{"op":"Lt","value":"str"}}`,
			out:  MatchExpressionSet{},
			err:  assert.NotNil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &MatchExpressionSet{}
			err := mes.UnmarshalJSON([]byte(tc.data))
			tc.err(t, err)
			assert.Equal(t, tc.out, *mes)
		})
	}
}
