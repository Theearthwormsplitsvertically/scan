package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/modules"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type scannerFakeModule struct {
	descriptor coremodule.Descriptor
	probe      func(context.Context, provider.Lookup) coremodule.SupportResult
	collect    func(context.Context, provider.Lookup, coremodule.Request) coremodule.Result
}

func (item scannerFakeModule) Descriptor() coremodule.Descriptor { return item.descriptor }

func (item scannerFakeModule) Probe(ctx context.Context, providers provider.Lookup) coremodule.SupportResult {
	if item.probe != nil {
		return item.probe(ctx, providers)
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

func (item scannerFakeModule) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	if item.collect != nil {
		return item.collect(ctx, providers, request)
	}
	return successfulFakeResult(item.descriptor.Name)
}

func TestScannerAllUsesRegistryPlan(t *testing.T) {
	t.Parallel()

	registry := coremodule.NewRegistry()
	mustRegisterScannerModule(t, registry, recordingModule("custom"))
	providers, err := provider.NewSet("linux")
	if err != nil {
		t.Fatal(err)
	}
	scanner := NewScannerWithClock(registry, providers, model.AgentInfo{Name: "test-agent"}, fixedScannerClock)
	batch, err := scanner.ScanTarget(context.Background(), "all")
	if err != nil {
		t.Fatal(err)
	}
	if batch.RequestedModule != "all" || batch.Type != model.BatchTypeSnapshot || len(batch.Results) != 1 {
		t.Fatalf("batch = %+v", batch)
	}
	if !batch.Results[0].Published {
		t.Fatal("all result not published")
	}
}

func TestScannerSingleModuleHidesDependencyRecords(t *testing.T) {
	t.Parallel()

	registry := coremodule.NewRegistry()
	host := recordingModule("host")
	process := recordingModule("process")
	process.descriptor.HardDependencies = []string{"host"}
	process.collect = func(_ context.Context, _ provider.Lookup, request coremodule.Request) coremodule.Result {
		if len(request.Dependencies["host"].Data.Records) != 1 {
			t.Fatalf("host dependency = %+v", request.Dependencies["host"])
		}
		return successfulFakeResult("process")
	}
	mustRegisterScannerModule(t, registry, host)
	mustRegisterScannerModule(t, registry, process)
	providers, _ := provider.NewSet("linux")
	scanner := NewScannerWithClock(registry, providers, model.AgentInfo{Name: "test-agent"}, fixedScannerClock)

	batch, err := scanner.ScanTarget(context.Background(), "process")
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Results) != 2 {
		t.Fatalf("results = %+v", batch.Results)
	}
	if batch.Results[0].Module != "host" || batch.Results[0].Published || len(batch.Results[0].Records) != 0 {
		t.Fatalf("host result = %+v", batch.Results[0])
	}
	if batch.Results[1].Module != "process" || !batch.Results[1].Published || len(batch.Results[1].Records) != 1 {
		t.Fatalf("process result = %+v", batch.Results[1])
	}
}

func TestScannerReturnsCanceledBeforePlanning(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	providers, _ := provider.NewSet("linux")
	_, err := NewScanner(coremodule.NewRegistry(), providers).ScanTarget(ctx, "all")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestScannerIsolatesPanicAndTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		module     scannerFakeModule
		wantStatus model.Status
		wantCode   string
	}{
		{
			name: "panic",
			module: scannerFakeModule{
				descriptor: coremodule.Descriptor{Name: "panic", Timeout: "1s"},
				collect:    func(context.Context, provider.Lookup, coremodule.Request) coremodule.Result { panic("boom") },
			},
			wantStatus: model.StatusFailed, wantCode: "module_panic",
		},
		{
			name: "timeout",
			module: scannerFakeModule{
				descriptor: coremodule.Descriptor{Name: "slow", Timeout: "1ms"},
				collect: func(ctx context.Context, _ provider.Lookup, _ coremodule.Request) coremodule.Result {
					<-ctx.Done()
					return coremodule.Result{}
				},
			},
			wantStatus: model.StatusTimeout, wantCode: "module_timeout",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			registry := coremodule.NewRegistry()
			mustRegisterScannerModule(t, registry, test.module)
			providers, _ := provider.NewSet("linux")
			batch, err := NewScanner(registry, providers).ScanTarget(context.Background(), test.module.descriptor.Name)
			if err != nil {
				t.Fatal(err)
			}
			result := batch.Results[0]
			if result.Status != test.wantStatus || len(result.Errors) != 1 || result.Errors[0].Code != test.wantCode {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestScannerDoesNotRunTargetAfterHardDependencyFailure(t *testing.T) {
	t.Parallel()

	registry := coremodule.NewRegistry()
	mustRegisterScannerModule(t, registry, scannerFakeModule{
		descriptor: coremodule.Descriptor{Name: "host"},
		collect: func(context.Context, provider.Lookup, coremodule.Request) coremodule.Result {
			return coremodule.Result{Data: model.ModuleResult{Module: "host", Status: model.StatusFailed, Errors: []model.ErrorDetail{}, Records: []model.AssetRecord{}, Relationships: []model.RelationshipRecord{}}}
		},
	})
	targetCalled := false
	mustRegisterScannerModule(t, registry, scannerFakeModule{
		descriptor: coremodule.Descriptor{Name: "process", HardDependencies: []string{"host"}},
		collect: func(context.Context, provider.Lookup, coremodule.Request) coremodule.Result {
			targetCalled = true
			return successfulFakeResult("process")
		},
	})
	providers, _ := provider.NewSet("linux")
	batch, err := NewScanner(registry, providers).ScanTarget(context.Background(), "process")
	if err != nil {
		t.Fatal(err)
	}
	if targetCalled {
		t.Fatal("target ran after hard dependency failure")
	}
	if batch.Results[1].Status != model.StatusFailed || batch.Results[1].Errors[0].Code != "dependency_failed" {
		t.Fatalf("target result = %+v", batch.Results[1])
	}
}

func TestScannerEmptyNonLinuxProvidersReturnUnsupported(t *testing.T) {
	t.Parallel()

	registry, err := modules.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := provider.NewSet("windows")
	batch, err := NewScanner(registry, providers).ScanTarget(context.Background(), "host")
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Results) != 1 || batch.Results[0].Status != model.StatusUnsupported || len(batch.Results[0].Records) != 0 {
		t.Fatalf("results = %+v", batch.Results)
	}
}

func TestScannerDoctorAndModulesDegradeWithoutPlatformProvider(t *testing.T) {
	t.Parallel()

	registry := coremodule.NewRegistry()
	mustRegisterScannerModule(t, registry, scannerFakeModule{
		descriptor: coremodule.Descriptor{Name: "panic-probe"},
		probe:      func(context.Context, provider.Lookup) coremodule.SupportResult { panic("probe boom") },
	})
	providers, _ := provider.NewSet("windows")
	scanner := NewScannerWithClock(registry, providers, model.AgentInfo{Name: "test-agent"}, fixedScannerClock)
	doctor, err := scanner.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if doctor.OS != "windows" || doctor.Root || doctor.Capabilities.Items == nil {
		t.Fatalf("doctor = %+v", doctor)
	}
	infos, err := scanner.Modules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Support.Status != model.StatusUnsupported ||
		len(infos[0].Support.Errors) != 1 || infos[0].Support.Errors[0].Code != "probe_panic" {
		t.Fatalf("infos = %+v", infos)
	}
}

func recordingModule(name string) scannerFakeModule {
	return scannerFakeModule{descriptor: coremodule.Descriptor{Name: name, Timeout: "1s"}}
}

func successfulFakeResult(name string) coremodule.Result {
	return coremodule.Result{Data: model.ModuleResult{
		Module: name, SchemaVersion: model.BatchSchemaVersion, Status: model.StatusComplete,
		Authoritative: true, Errors: []model.ErrorDetail{}, Coverage: model.Coverage{
			ExpectedScopes: []string{"host"}, CompletedScopes: []string{"host"}, FailedScopes: []string{},
		},
		Records:       []model.AssetRecord{{RecordID: name + ":1", RecordType: name, Evidence: []model.Evidence{}}},
		Relationships: []model.RelationshipRecord{},
	}}
}

func mustRegisterScannerModule(t *testing.T, registry *coremodule.Registry, item coremodule.Module) {
	t.Helper()
	if err := registry.Register(item); err != nil {
		t.Fatal(err)
	}
}

func fixedScannerClock() time.Time {
	return time.Date(2026, time.August, 13, 10, 0, 0, 123, time.UTC)
}
