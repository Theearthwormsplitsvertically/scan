package module

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeModule struct {
	name string
	hard []string
}

func (item fakeModule) Descriptor() Descriptor {
	return Descriptor{Name: item.name, HardDependencies: item.hard}
}

func (fakeModule) Probe(context.Context, provider.Lookup) SupportResult {
	return SupportResult{Status: model.StatusOK}
}

func (fakeModule) Collect(context.Context, provider.Lookup, Request) Result {
	return Result{}
}

func TestRegistryListsNewModuleWithoutKnownNameTable(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(fakeModule{name: "custom"}); err != nil {
		t.Fatal(err)
	}
	listed := registry.List()
	if len(listed) != 1 || listed[0].Name != "custom" {
		t.Fatalf("listed = %+v", listed)
	}
	plan, err := registry.Plan("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].Descriptor().Name != "custom" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestRegistryPlansHardDependenciesOnce(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	mustRegister(t, registry, fakeModule{name: "host"})
	mustRegister(t, registry, fakeModule{name: "process", hard: []string{"host"}})
	mustRegister(t, registry, fakeModule{name: "port", hard: []string{"process", "host"}})
	plan, err := registry.Plan("port")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(plan); !reflect.DeepEqual(got, []string{"host", "process", "port"}) {
		t.Fatalf("plan = %v", got)
	}
}

func TestRegistryPlansSameLayerAlphabetically(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	mustRegister(t, registry, fakeModule{name: "zeta"})
	mustRegister(t, registry, fakeModule{name: "alpha"})
	mustRegister(t, registry, fakeModule{name: "target", hard: []string{"zeta", "alpha"}})
	plan, err := registry.Plan("target")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(plan); !reflect.DeepEqual(got, []string{"alpha", "zeta", "target"}) {
		t.Fatalf("plan = %v", got)
	}
}

func TestRegistryFinishesDependencyLayerBeforeDependents(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	mustRegister(t, registry, fakeModule{name: "alpha"})
	mustRegister(t, registry, fakeModule{name: "zeta"})
	mustRegister(t, registry, fakeModule{name: "beta", hard: []string{"alpha"}})
	plan, err := registry.Plan("all")
	if err != nil {
		t.Fatal(err)
	}
	if got := names(plan); !reflect.DeepEqual(got, []string{"alpha", "zeta", "beta"}) {
		t.Fatalf("plan = %v", got)
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modules []fakeModule
	}{
		{name: "empty name", modules: []fakeModule{{name: ""}}},
		{name: "reserved all", modules: []fakeModule{{name: "all"}}},
		{name: "duplicate", modules: []fakeModule{{name: "host"}, {name: "host"}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			var err error
			for _, item := range test.modules {
				err = registry.Register(item)
				if err != nil {
					break
				}
			}
			if err == nil {
				t.Fatal("invalid registration accepted")
			}
		})
	}
}

func TestRegistryRejectsUnknownDependencyAndCycleWithoutPartialPlan(t *testing.T) {
	t.Parallel()

	t.Run("unknown dependency", func(t *testing.T) {
		registry := NewRegistry()
		mustRegister(t, registry, fakeModule{name: "process", hard: []string{"host"}})
		plan, err := registry.Plan("process")
		if err == nil || plan != nil {
			t.Fatalf("plan = %v, err = %v", names(plan), err)
		}
	})

	t.Run("dependency cycle", func(t *testing.T) {
		registry := NewRegistry()
		mustRegister(t, registry, fakeModule{name: "alpha", hard: []string{"beta"}})
		mustRegister(t, registry, fakeModule{name: "beta", hard: []string{"alpha"}})
		plan, err := registry.Plan("all")
		if err == nil || plan != nil {
			t.Fatalf("plan = %v, err = %v", names(plan), err)
		}
	})
}

func TestRegistryRejectsUnknownTarget(t *testing.T) {
	t.Parallel()

	plan, err := NewRegistry().Plan("missing")
	if err == nil || plan != nil {
		t.Fatalf("plan = %v, err = %v", plan, err)
	}
}

func mustRegister(t *testing.T, registry *Registry, item Module) {
	t.Helper()
	if err := registry.Register(item); err != nil {
		t.Fatal(err)
	}
}

func names(items []Module) []string {
	if items == nil {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Descriptor().Name)
	}
	return result
}
