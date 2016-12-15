package main

import (
	"k8s.io/client-go/pkg/api/resource"
	api "k8s.io/client-go/pkg/api/v1"
)

////////////////////////////////////////////////////////////////////////////////
// Fake Source (used only for testing)

// Implements main.FeatureSource.
type fakeSource struct{}

func (s fakeSource) Name() string { return "fake" }
func (s fakeSource) Discover() ([]string, error) {
	features := []string{}

	// Adding three fake features.
	features = append(features, "fakefeature1", "fakefeature2", "fakefeature3")

	return features, nil
}
func (s fakeSource) DiscoverResources() (api.ResourceList, error) {
	resources := api.ResourceList{
		api.ResourceName("fake"): resource.MustParse("8"),
	}
	return resources, nil
}

////////////////////////////////////////////////////////////////////////////////
// Fake Panic Source (used only for testing)

// Implements main.FeatureSource.
type fakePanicSource struct{}

func (s fakePanicSource) Name() string { return "fakepanic" }
func (s fakePanicSource) Discover() ([]string, error) {
	panic("fake panic error")
}
func (s fakePanicSource) DiscoverResources() (api.ResourceList, error) {
	panic("fake panic error")
}
