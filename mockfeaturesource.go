package main

import (
	"sigs.k8s.io/node-feature-discovery/source"
	"github.com/stretchr/testify/mock"
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
func (_m *MockFeatureSource) Discover() (source.Features, error) {
	ret := _m.Called()

	var r0 source.Features
	if rf, ok := ret.Get(0).(func() source.Features); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(source.Features)
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
