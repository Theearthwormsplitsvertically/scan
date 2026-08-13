package host

import (
	"context"
	"reflect"
	"sort"
	"strings"
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
		MemoryTotalBytes: 2_097_152, BootID: "boot-1", DMIUUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	}
}

func TestHostModuleNormalizesCanonicalDMIUUIDAcrossIdentityOutputs(t *testing.T) {
	const canonical = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

	upper := minimalHost()
	upper.DMIUUID = "  6BA7B810-9DAD-11D1-80B4-00C04FD430C8\t"
	lower := minimalHost()
	lower.DMIUUID = canonical

	upperResult := collectHostForTest(t, upper, model.StatusOK)
	lowerResult := collectHostForTest(t, lower, model.StatusOK)
	if len(upperResult.Data.Records) != 1 || len(lowerResult.Data.Records) != 1 {
		t.Fatalf("records: upper=%+v lower=%+v", upperResult.Data.Records, lowerResult.Data.Records)
	}
	upperRecord := upperResult.Data.Records[0]
	lowerRecord := lowerResult.Data.Records[0]
	if upperRecord.RecordID != lowerRecord.RecordID {
		t.Fatalf("equivalent DMI UUID IDs differ: upper=%q lower=%q", upperRecord.RecordID, lowerRecord.RecordID)
	}
	if got := upperRecord.Attributes["dmi_uuid"]; got != canonical {
		t.Fatalf("published dmi_uuid = %#v, want %q", got, canonical)
	}
	internal, ok := upperResult.Internal.(model.Host)
	if !ok || internal.DMIUUID != canonical {
		t.Fatalf("internal = %#v, want normalized DMI UUID %q", upperResult.Internal, canonical)
	}
	if upperRecord.Confidence != "strong" {
		t.Fatalf("confidence = %q, want strong", upperRecord.Confidence)
	}
}

func TestHostModuleRejectsPlaceholderDMIUUIDsAndFallsBackToHostname(t *testing.T) {
	placeholders := []string{
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	}
	for _, placeholder := range placeholders {
		t.Run(placeholder, func(t *testing.T) {
			host := minimalHost()
			host.DMIUUID = placeholder
			result := collectHostForTest(t, host, model.StatusOK)
			if result.Data.Status != model.StatusPartial || result.Data.Authoritative {
				t.Fatalf("status = %q, authoritative = %v, errors = %+v", result.Data.Status, result.Data.Authoritative, result.Data.Errors)
			}
			if len(result.Data.Records) != 1 {
				t.Fatalf("records = %+v", result.Data.Records)
			}
			record := result.Data.Records[0]
			wantID := coremodule.StableRecordID("host", "", "", "server-1")
			if record.RecordID != wantID || record.Confidence != "inferred" {
				t.Fatalf("record identity = %+v, want ID %q and inferred confidence", record, wantID)
			}
			if got := record.Attributes["dmi_uuid"]; got != "" {
				t.Fatalf("published dmi_uuid = %#v, want empty", got)
			}
			internal, ok := result.Internal.(model.Host)
			if !ok || internal.DMIUUID != "" {
				t.Fatalf("internal = %#v, want empty DMI UUID", result.Internal)
			}
			assertHostErrorContains(t, result, "invalid DMI UUID")
		})
	}
}

func TestHostModuleRejectsInvalidDMIUUIDWithoutHostname(t *testing.T) {
	host := minimalHost()
	host.DMIUUID = "not-a-uuid"
	host.Hostname = ""

	result := collectHostForTest(t, host, model.StatusOK)
	if result.Data.Status != model.StatusFailed || result.Data.Authoritative {
		t.Fatalf("status = %q, authoritative = %v", result.Data.Status, result.Data.Authoritative)
	}
	if len(result.Data.Records) != 0 {
		t.Fatalf("records = %+v, want none", result.Data.Records)
	}
	internal, ok := result.Internal.(model.Host)
	if !ok || internal.DMIUUID != "" {
		t.Fatalf("internal = %#v, want empty DMI UUID", result.Internal)
	}
	assertHostErrorContains(t, result, "invalid DMI UUID")
}

func TestHostnameOnlyRecordIDIgnoresBootID(t *testing.T) {
	first := RecordID(model.Host{Hostname: "server", BootID: "boot-1"})
	second := RecordID(model.Host{Hostname: "server", BootID: "boot-2"})
	if first == "" || second != first {
		t.Fatalf("hostname-only record IDs = %q, %q", first, second)
	}
}

func assertHostErrorContains(t *testing.T, result coremodule.Result, substring string) {
	t.Helper()
	for _, detail := range result.Data.Errors {
		if strings.Contains(detail.Message, substring) {
			return
		}
	}
	t.Fatalf("errors = %+v, want message containing %q", result.Data.Errors, substring)
}

func TestHostModulePublishesOnlyApprovedAttributes(t *testing.T) {
	result := collectHostForTest(t, minimalHost(), model.StatusOK)
	if len(result.Data.Records) != 1 {
		t.Fatalf("records = %+v", result.Data.Records)
	}
	record := result.Data.Records[0]
	wantKeys := []string{
		"architecture", "boot_id", "distribution_id", "distribution_name",
		"distribution_version", "dmi_uuid", "hostname", "kernel_release", "memory_total_bytes",
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
	dmiOnly := minimalHost()
	hostnameFallback := minimalHost()
	hostnameFallback.DMIUUID = ""
	noIdentity := minimalHost()
	noIdentity.DMIUUID = ""
	noIdentity.Hostname = ""

	tests := []struct {
		name       string
		host       model.Host
		confidence string
		status     model.Status
		records    int
	}{
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

func TestHostModuleDowngradesHostnameFallbackFromSuccessfulProviderStatuses(t *testing.T) {
	host := minimalHost()
	host.DMIUUID = ""

	for _, upstreamStatus := range []model.Status{model.StatusOK, model.StatusComplete} {
		t.Run(string(upstreamStatus), func(t *testing.T) {
			result := collectHostForTest(t, host, upstreamStatus)
			if result.Data.Status != model.StatusPartial || result.Data.Authoritative {
				t.Fatalf("result = %+v", result.Data)
			}
		})
	}
}

func TestHostModuleExposesPartialProviderErrors(t *testing.T) {
	result := collectHostWithStatusForTest(t, minimalHost(), model.CollectorStatus{
		Collector: "host", Status: model.StatusPartial, Errors: []string{"/proc/meminfo: missing or invalid MemTotal"},
	})
	if result.Data.Status != model.StatusPartial || result.Data.Authoritative {
		t.Fatalf("result = %+v", result.Data)
	}
	want := []model.ErrorDetail{{
		Code: "collection_error", Message: "/proc/meminfo: missing or invalid MemTotal", Provider: "host",
	}}
	if !reflect.DeepEqual(result.Data.Errors, want) {
		t.Fatalf("errors = %#v, want %#v", result.Data.Errors, want)
	}
}

func TestHostModelContainsOnlyMinimalFacts(t *testing.T) {
	typ := reflect.TypeOf(model.Host{})
	want := []string{
		"Hostname", "DistributionName", "DistributionID", "DistributionVersion",
		"KernelRelease", "Architecture", "BootID", "DMIUUID", "MemoryTotalBytes",
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
	return collectHostWithStatusForTest(t, host, model.CollectorStatus{
		Collector: "host", Status: status, Errors: []string{},
	})
}

func collectHostWithStatusForTest(t *testing.T, host model.Host, status model.CollectorStatus) coremodule.Result {
	t.Helper()
	providers, err := provider.NewSet("linux", fakeHostProvider{
		host:   host,
		status: status,
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

	host := model.Host{DMIUUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Hostname: "server"}
	first := RecordID(host)
	second := RecordID(host)
	if first == "" || first != second {
		t.Fatalf("record IDs = %q, %q", first, second)
	}

	bootChanged := host
	bootChanged.BootID = "boot-2"
	if got := RecordID(bootChanged); got != first {
		t.Fatalf("record ID changed with boot ID: %q, %q", first, got)
	}
	dmiChanged := host
	dmiChanged.DMIUUID = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
	if got := RecordID(dmiChanged); got == first {
		t.Fatalf("record ID did not change with DMI UUID: %q", got)
	}
}
