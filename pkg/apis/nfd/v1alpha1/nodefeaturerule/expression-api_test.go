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

package nodefeaturerule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	api "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1/nodefeaturerule"
)

type BoolAssertionFunc func(assert.TestingT, bool, ...interface{}) bool

type ValueAssertionFunc func(assert.TestingT, interface{}, ...interface{}) bool

func TestMatchKeys(t *testing.T) {
	type I = map[string]nfdv1alpha1.Nil
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
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := api.MatchGetKeys(mes, tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = api.MatchKeys(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchValues(t *testing.T) {
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
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := api.MatchGetValues(mes, tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = api.MatchValues(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchInstances(t *testing.T) {
	type I = nfdv1alpha1.InstanceFeature
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
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			out, err := api.MatchGetInstances(mes, tc.input)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err := api.MatchInstances(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}
