package main

import (
	"github.com/stretchr/testify/mock"
	api "k8s.io/client-go/pkg/api/v1"
)

type MockFeatureSource struct {
	mock.Mock
}

// Name provides a mock function with no input arguments
// and string as return value
func (_m *MockFeatureSource) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Discover provides a mock function with no input arguments
// and []string and error as the return values
func (_m *MockFeatureSource) Discover() ([]string, error) {
	ret := _m.Called()

	var r0 []string
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
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

// DiscoverResources provides a mock function with no input arguments
// and api.ResourceList and error as the return values
func (_m *MockFeatureSource) DiscoverResources() (api.ResourceList, error) {
	ret := _m.Called()

	var r0 api.ResourceList
	if rf, ok := ret.Get(0).(func() api.ResourceList); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(api.ResourceList)
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
