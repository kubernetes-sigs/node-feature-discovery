package main

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

////////////////////////////////////////////////////////////////////////////////
// Fake Panic Source (used only for testing)

// Implements main.FeatureSource.
type fakePanicSource struct{}

func (s fakePanicSource) Name() string { return "fakepanic" }
func (s fakePanicSource) Discover() ([]string, error) {
	panic("fake panic error")
}
