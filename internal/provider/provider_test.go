package provider

import "testing"

type fakeProvider struct {
	name string
}

func (provider fakeProvider) Capability() string { return provider.name }

func TestSetRegistersCapabilitiesWithoutCentralSwitch(t *testing.T) {
	t.Parallel()

	set, err := NewSet("linux", fakeProvider{name: "custom-capability"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := set.Lookup("custom-capability")
	if !ok || got.Capability() != "custom-capability" {
		t.Fatalf("provider = %#v, ok = %v", got, ok)
	}
	if set.Platform() != "linux" {
		t.Fatalf("platform = %q", set.Platform())
	}
}

func TestSetRejectsDuplicateCapability(t *testing.T) {
	t.Parallel()

	_, err := NewSet("linux",
		fakeProvider{name: "process"},
		fakeProvider{name: "process"},
	)
	if err == nil {
		t.Fatal("duplicate capability accepted")
	}
}

func TestSetRejectsEmptyCapability(t *testing.T) {
	t.Parallel()

	_, err := NewSet("linux", fakeProvider{})
	if err == nil {
		t.Fatal("empty capability accepted")
	}
}

type typedFakeProvider struct {
	fakeProvider
}

func (typedFakeProvider) Marker() {}

type markedProvider interface {
	Provider
	Marker()
}

func TestAsReturnsOnlyMatchingTypedProvider(t *testing.T) {
	t.Parallel()

	set, err := NewSet("linux",
		typedFakeProvider{fakeProvider{name: "typed"}},
		fakeProvider{name: "untyped"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := As[markedProvider](set, "typed"); !ok {
		t.Fatal("typed provider was not returned")
	}
	if _, ok := As[markedProvider](set, "missing"); ok {
		t.Fatal("missing provider reported as available")
	}
	if _, ok := As[markedProvider](set, "untyped"); ok {
		t.Fatal("wrong provider type reported as available")
	}
}
