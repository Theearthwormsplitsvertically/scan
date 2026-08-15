package modules

import (
	"reflect"
	"testing"

	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

func TestNewRegistryContainsOnlyImplementedModulesAndPlansAll(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	descriptors := registry.List()
	listed := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		listed = append(listed, descriptor.Name)
	}
	if !reflect.DeepEqual(listed, []string{"connection", "host", "network", "package", "port", "process", "service"}) {
		t.Fatalf("listed modules = %v", listed)
	}
	if _, exists := registry.Lookup("all"); exists {
		t.Fatal("virtual all target was registered as a real module")
	}
	plan, err := registry.PlanAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := moduleNames(plan); !reflect.DeepEqual(got, []string{"host", "network", "package", "process", "port", "service", "connection"}) {
		t.Fatalf("all plan = %v", got)
	}
	counts := map[string]int{}
	for _, name := range moduleNames(plan) {
		counts[name]++
	}
	if counts["process"] != 1 {
		t.Fatalf("process appears %d times in all plan", counts["process"])
	}
}

func moduleNames(items []coremodule.Module) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Descriptor().Name)
	}
	return names
}
