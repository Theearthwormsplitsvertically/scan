package agent

import (
	"context"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestScanModuleHostExposesOnlyHostData(t *testing.T) {
	t.Parallel()

	hostCalls := 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: testDoctor("procfs", "sysfs"),
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			hostCalls++
			return model.Host{Hostname: "module-host"}, okStatus("host")
		},
	})

	report, err := runtime.ScanModule(context.Background(), ModuleHost)

	if err != nil {
		t.Fatalf("ScanModule() error = %v", err)
	}
	if hostCalls != 1 {
		t.Fatalf("host calls = %d, want 1", hostCalls)
	}
	if report.SchemaName != model.ModuleReportSchemaName || report.Module != string(ModuleHost) {
		t.Fatalf("report identity = %+v", report)
	}
	if report.Data.Host == nil || report.Data.Host.Hostname != "module-host" {
		t.Fatalf("host data = %+v", report.Data.Host)
	}
	if report.Data.NetworkInterfaces != nil || report.Data.Processes != nil || report.Data.Sockets != nil {
		t.Fatalf("unselected module data leaked: %+v", report.Data)
	}
	assertCollectorStatus(t, report.CollectorStatus, "host", model.StatusOK)
}

func TestScanModuleProcessRunsHostDependencyWithoutExposingIt(t *testing.T) {
	t.Parallel()

	hostCalls := 0
	processCalls := 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: testDoctor("procfs"),
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			hostCalls++
			return model.Host{BootID: "boot-a"}, okStatus("host")
		},
		Processes: func(_ context.Context, bootID string) ([]model.Process, model.CollectorStatus) {
			processCalls++
			if bootID != "boot-a" {
				t.Fatalf("boot ID = %q, want boot-a", bootID)
			}
			return []model.Process{{ID: "boot-a:12:3", PID: 12}}, okStatus("process")
		},
	})

	report, err := runtime.ScanModule(context.Background(), ModuleProcess)

	if err != nil {
		t.Fatalf("ScanModule() error = %v", err)
	}
	if hostCalls != 1 || processCalls != 1 {
		t.Fatalf("calls host=%d process=%d, want 1 each", hostCalls, processCalls)
	}
	if report.Data.Host != nil {
		t.Fatalf("internal host dependency leaked: %+v", report.Data.Host)
	}
	if len(report.Data.Processes) != 1 || report.Data.Processes[0].PID != 12 {
		t.Fatalf("process data = %+v", report.Data.Processes)
	}
	assertCollectorStatus(t, report.CollectorStatus, "host", model.StatusOK)
	assertCollectorStatus(t, report.CollectorStatus, "process", model.StatusOK)
}

func TestScanModuleSocketRunsDependenciesOnceAndExposesEvidence(t *testing.T) {
	t.Parallel()

	hostCalls := 0
	processCalls := 0
	socketCalls := 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: testDoctor("procfs", "proc_net"),
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			hostCalls++
			return model.Host{BootID: "boot-b"}, okStatus("host")
		},
		Processes: func(context.Context, string) ([]model.Process, model.CollectorStatus) {
			processCalls++
			return []model.Process{{ID: "boot-b:22:4", PID: 22}}, okStatus("process")
		},
		Sockets: func(_ context.Context, processes []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
			socketCalls++
			if len(processes) != 1 || processes[0].PID != 22 {
				t.Fatalf("socket process dependency = %+v", processes)
			}
			return []model.Socket{{ID: "socket:100", Inode: 100, PIDs: []int{22}, ProcessIDs: []string{"boot-b:22:4"}}},
				[]model.Relationship{{ID: "rel:100:22", Confidence: "exact"}}, okStatus("socket")
		},
	})

	report, err := runtime.ScanModule(context.Background(), ModuleSocket)

	if err != nil {
		t.Fatalf("ScanModule() error = %v", err)
	}
	if hostCalls != 1 || processCalls != 1 || socketCalls != 1 {
		t.Fatalf("calls host=%d process=%d socket=%d, want 1 each", hostCalls, processCalls, socketCalls)
	}
	if report.Data.Processes != nil {
		t.Fatalf("internal process dependency leaked: %+v", report.Data.Processes)
	}
	if len(report.Data.Sockets) != 1 || len(report.Data.Relationships) != 1 {
		t.Fatalf("socket data = %+v", report.Data)
	}
	if report.Data.Relationships[0].Confidence != "exact" {
		t.Fatalf("relationship = %+v", report.Data.Relationships[0])
	}
}

func TestScanModuleRunsSystemPreflightBeforeCollector(t *testing.T) {
	t.Parallel()

	preflightComplete := false
	doctorCalls := 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: func(context.Context) model.DoctorReport {
			doctorCalls++
			preflightComplete = true
			return model.DoctorReport{SystemProfile: model.SystemProfile{
				OS: "linux", DistributionID: "centos", DistributionVersion: "7",
				AvailableSources: map[string]bool{"procfs": true, "sysfs": true},
			}}
		},
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			if !preflightComplete {
				t.Fatal("host collector ran before system preflight")
			}
			return model.Host{Hostname: "ordered-host"}, okStatus("host")
		},
	})

	report, err := runtime.ScanModule(context.Background(), ModuleHost)
	if err != nil {
		t.Fatalf("ScanModule() error = %v", err)
	}
	if doctorCalls != 1 {
		t.Fatalf("doctor calls = %d, want 1", doctorCalls)
	}
	if report.SystemProfile.DistributionID != "centos" {
		t.Fatalf("system profile = %+v", report.SystemProfile)
	}
	if len(report.Strategies) != 1 {
		t.Fatalf("strategies = %+v, want one", report.Strategies)
	}
	strategy := report.Strategies[0]
	if strategy.Module != "host" || strategy.Backend != "procfs_sysfs" || strategy.Status != model.StatusOK {
		t.Fatalf("strategy = %+v", strategy)
	}
	assertCollectorStatus(t, report.CollectorStatus, "capability", model.StatusOK)
}

func TestScanModuleSkipsProcessWhenRequiredSourceIsMissing(t *testing.T) {
	t.Parallel()

	processCalls := 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: testDoctor("sysfs"),
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			return model.Host{BootID: "boot-c"}, okStatus("host")
		},
		Processes: func(context.Context, string) ([]model.Process, model.CollectorStatus) {
			processCalls++
			return nil, okStatus("process")
		},
	})

	report, err := runtime.ScanModule(context.Background(), ModuleProcess)
	if err != nil {
		t.Fatalf("ScanModule() error = %v", err)
	}
	if processCalls != 0 {
		t.Fatalf("process calls = %d, want 0", processCalls)
	}
	if len(report.Strategies) != 1 {
		t.Fatalf("strategies = %+v, want one", report.Strategies)
	}
	strategy := report.Strategies[0]
	if strategy.Backend != "unavailable" || strategy.Status != model.StatusUnsupported {
		t.Fatalf("strategy = %+v", strategy)
	}
	if len(strategy.MissingSources) != 1 || strategy.MissingSources[0] != "procfs" || strategy.Reason == "" {
		t.Fatalf("missing-source evidence = %+v", strategy)
	}
	assertCollectorStatus(t, report.CollectorStatus, "process", model.StatusUnsupported)
}

func TestScanModuleRejectsUnknownModule(t *testing.T) {
	t.Parallel()

	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{})

	_, err := runtime.ScanModule(context.Background(), Module("unknown"))

	if err == nil {
		t.Fatal("ScanModule() error = nil, want unknown module error")
	}
}

func okStatus(collector string) model.CollectorStatus {
	return model.CollectorStatus{Collector: collector, Status: model.StatusOK, Errors: []string{}}
}

func testDoctor(sources ...string) func(context.Context) model.DoctorReport {
	return func(context.Context) model.DoctorReport {
		available := make(map[string]bool, len(sources))
		for _, source := range sources {
			available[source] = true
		}
		return model.DoctorReport{SystemProfile: model.SystemProfile{
			OS: "linux", SecurityModules: []string{}, ContainerRuntimes: []string{}, AvailableSources: available,
		}}
	}
}
