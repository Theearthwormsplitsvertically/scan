package process

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeProcessProvider struct {
	bootID *string
}

func (fakeProcessProvider) Capability() string { return provider.CapabilityProcess }

func (item fakeProcessProvider) Collect(_ context.Context, bootID string) ([]model.Process, model.CollectorStatus) {
	*item.bootID = bootID
	return []model.Process{{
		ID: "boot-1:10:20", PID: 10, PPID: 1, StartTime: 20,
		Name: "web", State: "S", CommandLine: []string{"web", "--listen", "443"},
		Cgroups: []string{"0::/system.slice/web.service"},
	}}, model.CollectorStatus{Collector: "process", Status: model.StatusOK, Errors: []string{}}
}

func TestProcessModuleUsesHostBootIDAndPublishesOnlyProcesses(t *testing.T) {
	t.Parallel()

	seenBootID := ""
	providers, err := provider.NewSet("linux", fakeProcessProvider{bootID: &seenBootID})
	if err != nil {
		t.Fatal(err)
	}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host": {Internal: model.Host{MachineID: "machine", DMIUUID: "dmi", BootID: "boot-1"}},
	}}
	result := New().Collect(context.Background(), providers, request)
	if seenBootID != "boot-1" {
		t.Fatalf("boot ID = %q", seenBootID)
	}
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	record := result.Data.Records[0]
	if record.RecordType != "process" || !record.States.Running {
		t.Fatalf("record = %+v", record)
	}
	if _, exists := record.Attributes["environ"]; exists {
		t.Fatalf("process record contains environ: %+v", record.Attributes)
	}
	if _, ok := result.Internal.([]model.Process); !ok {
		t.Fatalf("internal = %#v", result.Internal)
	}
}

func TestProcessModuleDescriptorDeclaresHostDependencyAndSchedule(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.Name != "process" || descriptor.DefaultInterval != "12h" ||
		!reflect.DeepEqual(descriptor.HardDependencies, []string{"host"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
