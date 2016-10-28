package main

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type MockAPIHelpers struct {
	mock.Mock
}

// GetClient provides a mock function with no input arguments and
// *client.Client and error as return value
func (_m *MockAPIHelpers) GetClient() (*client.Client, error) {
	ret := _m.Called()

	var r0 *client.Client
	if rf, ok := ret.Get(0).(func() *client.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*client.Client)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNode provides a mock function with *client.Client as input argument and
// *api.Node and error as return values
func (_m *MockAPIHelpers) GetNode(_a0 *client.Client) (*api.Node, error) {
	ret := _m.Called(_a0)

	var r0 *api.Node
	if rf, ok := ret.Get(0).(func(*client.Client) *api.Node); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*client.Client) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveLabels provides a mock function with *api.Node and main.Labels as the input arguments and
// no return value
func (_m *MockAPIHelpers) RemoveLabels(_a0 *api.Node, _a1 string) {
	_m.Called(_a0, _a1)
}

// AddLabels provides a mock function with *api.Node and main.Labels as the input arguments and
// no return value
func (_m *MockAPIHelpers) AddLabels(_a0 *api.Node, _a1 Labels) {
	_m.Called(_a0, _a1)
}

// UpdateNode provides a mock function with *client.Client and *api.Node as the input arguments and
// error as the return value
func (_m *MockAPIHelpers) UpdateNode(_a0 *client.Client, _a1 *api.Node) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*client.Client, *api.Node) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
