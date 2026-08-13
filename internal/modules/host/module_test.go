package host

import (
	"context"
	"reflect"
	"sort"
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

	result := collectHostForTest(t, minimalHost(), model.StatusOK)
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

func minimalHost() model.Host {
	return model.Host{
		Hostname: "server-1", DistributionName: "Example Linux 1",
		DistributionID: "example", DistributionVersion: "1",
		KernelRelease: "6.8.0-test", Architecture: "amd64",
		MemoryTotalBytes: 2_097_152, MachineID: "machine", BootID: "boot-1", DMIUUID: "dmi",
	}
}

func TestHostModulePublishesOnlyApprovedAttributes(t *testing.T) {
	result := collectHostForTest(t, minimalHost(), model.StatusOK)
	if len(result.Data.Records) != 1 {
		t.Fatalf("records = %+v", result.Data.Records)
	}
	record := result.Data.Records[0]
	wantKeys := []string{
		"architecture", "boot_id", "distribution_id", "distribution_name",
		"distribution_version", "dmi_uuid", "hostname", "kernel_release",
		"machine_id", "memory_total_bytes",
	}
	gotKeys := make([]string, 0, len(record.Attributes))
	for key := range record.Attributes {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("attribute keys = %v, want %v", gotKeys, wantKeys)
	}
	if record.Version != "" || record.Vendor != "" {
		t.Fatalf("duplicate top-level fields: version=%q vendor=%q", record.Version, record.Vendor)
	}
}

func TestHostModuleIdentityConfidence(t *testing.T) {
	both := minimalHost()
	machineOnly := minimalHost()
	machineOnly.DMIUUID = ""
	dmiOnly := minimalHost()
	dmiOnly.MachineID = ""
	hostnameFallback := minimalHost()
	hostnameFallback.MachineID = ""
	hostnameFallback.DMIUUID = ""
	noIdentity := minimalHost()
	noIdentity.MachineID = ""
	noIdentity.DMIUUID = ""
	noIdentity.Hostname = ""

	tests := []struct {
		name       string
		host       model.Host
		confidence string
		status     model.Status
		records    int
	}{
		{"machine and dmi", both, "exact", model.StatusComplete, 1},
		{"machine only", machineOnly, "strong", model.StatusComplete, 1},
		{"dmi only", dmiOnly, "strong", model.StatusComplete, 1},
		{"hostname fallback", hostnameFallback, "inferred", model.StatusPartial, 1},
		{"no identity", noIdentity, "", model.StatusFailed, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collectHostForTest(t, test.host, model.StatusOK)
			if result.Data.Status != test.status || len(result.Data.Records) != test.records {
				t.Fatalf("result = %+v", result.Data)
			}
			if test.records == 1 && result.Data.Records[0].Confidence != test.confidence {
				t.Fatalf("confidence = %q", result.Data.Records[0].Confidence)
			}
			if test.status != model.StatusComplete && result.Data.Authoritative {
				t.Fatal("non-complete result is authoritative")
			}
		})
	}
}

func TestHostModelContainsOnlyMinimalFacts(t *testing.T) {
	typ := reflect.TypeOf(model.Host{})
	want := []string{
		"Hostname", "DistributionName", "DistributionID", "DistributionVersion",
		"KernelRelease", "Architecture", "MachineID", "BootID", "DMIUUID", "MemoryTotalBytes",
	}
	got := make([]string, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		got[index] = typ.Field(index).Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Host fields = %v, want %v", got, want)
	}
}

func collectHostForTest(t *testing.T, host model.Host, status model.Status) coremodule.Result {
	t.Helper()
	providers, err := provider.NewSet("linux", fakeHostProvider{
		host: host,
		status: model.CollectorStatus{
			Collector: "host", Status: status, Errors: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return New().Collect(t.Context(), providers, coremodule.Request{})
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
