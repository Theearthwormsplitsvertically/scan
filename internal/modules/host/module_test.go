package host

import (
	"context"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeHostProvider struct {
	host   model.Host
	status model.CollectorStatus
}

func (fakeHostProvider) Capability() string { return provider.CapabilityHost }

func (item fakeHostProvider) Collect(context.Context) (model.Host, model.CollectorStatus) {
	return item.host, item.status
}

func TestHostModuleCollectsUnifiedRecordAndKeepsInternalFact(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux", fakeHostProvider{
		host: model.Host{
			ID: "machine:dmi", Hostname: "server-1", MachineID: "machine",
			BootID: "boot-1", Distribution: "Example Linux", CPUCount: 4,
		},
		status: model.CollectorStatus{Collector: "host", Status: model.StatusOK, Errors: []string{}},
	})
	if err != nil {
		t.Fatal(err)
	}

	result := New().Collect(context.Background(), providers, coremodule.Request{})
	if result.Data.Status != model.StatusComplete {
		t.Fatalf("status = %q, errors = %+v", result.Data.Status, result.Data.Errors)
	}
	if len(result.Data.Records) != 1 {
		t.Fatalf("records = %+v", result.Data.Records)
	}
	record := result.Data.Records[0]
	if record.RecordType != "host" || record.HostID == "" || !record.States.Running {
		t.Fatalf("record = %+v", record)
	}
	if record.RecordID != record.HostID || record.ScopeID != record.HostID {
		t.Fatalf("record identity = %+v", record)
	}
	if len(record.Evidence) != 1 || record.Evidence[0].Provider != provider.CapabilityHost {
		t.Fatalf("evidence = %+v", record.Evidence)
	}
	internal, ok := result.Internal.(model.Host)
	if !ok || internal.BootID != "boot-1" {
		t.Fatalf("internal = %#v", result.Internal)
	}
}

func TestHostModuleDescriptorAndUnsupportedProbe(t *testing.T) {
	t.Parallel()

	item := New()
	descriptor := item.Descriptor()
	if descriptor.Name != "host" || descriptor.DefaultInterval != "24h" ||
		descriptor.ResourceClass != "light" || descriptor.Timeout != "15s" {
		t.Fatalf("descriptor = %+v", descriptor)
	}
	providers, err := provider.NewSet("windows")
	if err != nil {
		t.Fatal(err)
	}
	if support := item.Probe(context.Background(), providers); support.Status != model.StatusUnsupported {
		t.Fatalf("support = %+v", support)
	}
}

func TestRecordIDFallsBackDeterministically(t *testing.T) {
	t.Parallel()

	host := model.Host{MachineID: "machine", DMIUUID: "dmi", Hostname: "server"}
	first := RecordID(host)
	second := RecordID(host)
	if first == "" || first != second {
		t.Fatalf("record IDs = %q, %q", first, second)
	}
}
