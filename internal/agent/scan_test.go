package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestScanRetainsSuccessfulDomainsWhenSocketCollectorFails(t *testing.T) {
	t.Parallel()

	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: func(context.Context) model.DoctorReport {
			return model.DoctorReport{
				SchemaVersion: model.SchemaVersion,
				Capabilities:  model.CapabilityReport{Items: []model.Capability{{Name: "procfs", Status: model.StatusOK}}},
				SystemProfile: model.SystemProfile{OS: "linux", AvailableSources: map[string]bool{"procfs": true, "sysfs": true, "proc_net": true, "standard_library_network": true}},
			}
		},
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			return model.Host{Hostname: "asset-test-host"}, model.CollectorStatus{Collector: "host", Status: model.StatusOK, Errors: []string{}}
		},
		Network: func(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
			return []model.NetworkInterface{}, []model.Address{}, []model.Route{}, model.CollectorStatus{Collector: "network", Status: model.StatusOK, Errors: []string{}}
		},
		Processes: func(context.Context, string) ([]model.Process, model.CollectorStatus) {
			return []model.Process{{ID: "boot:10:1", PID: 10}}, model.CollectorStatus{Collector: "process", Status: model.StatusOK, Errors: []string{}}
		},
		Sockets: func(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
			return []model.Socket{}, []model.Relationship{}, model.CollectorStatus{Collector: "socket", Status: model.StatusFailed, Errors: []string{"/proc/net/tcp: permission denied"}}
		},
	})

	snapshot, err := runtime.Scan(context.Background())

	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if snapshot.SchemaVersion != model.SchemaVersion || snapshot.Host.Hostname != "asset-test-host" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if snapshot.Scan.ID == "" || snapshot.Scan.StartedAt.Location().String() != "UTC" || snapshot.Scan.FinishedAt.IsZero() {
		t.Fatalf("scan metadata = %+v", snapshot.Scan)
	}
	if snapshot.Sockets == nil || snapshot.Relationships == nil || snapshot.Packages == nil || snapshot.Applications == nil {
		t.Fatalf("reserved arrays must be non-nil: %+v", snapshot)
	}
	assertCollectorStatus(t, snapshot.CollectorStatus, "host", model.StatusOK)
	assertCollectorStatus(t, snapshot.CollectorStatus, "socket", model.StatusFailed)
	if len(snapshot.Strategies) != 4 || snapshot.Strategies[3].Backend != "proc_net" {
		t.Fatalf("strategies = %+v", snapshot.Strategies)
	}
}

func TestScanSkipsUnavailableDomainsAfterSinglePreflight(t *testing.T) {
	t.Parallel()

	doctorCalls, processCalls, socketCalls := 0, 0, 0
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{
		Doctor: func(context.Context) model.DoctorReport {
			doctorCalls++
			return model.DoctorReport{SystemProfile: model.SystemProfile{
				OS: "linux", AvailableSources: map[string]bool{"sysfs": true, "standard_library_network": true},
			}}
		},
		Host: func(context.Context) (model.Host, model.CollectorStatus) {
			return model.Host{Hostname: "limited-host"}, okStatus("host")
		},
		Network: func(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
			return nil, nil, nil, okStatus("network")
		},
		Processes: func(context.Context, string) ([]model.Process, model.CollectorStatus) {
			processCalls++
			return nil, okStatus("process")
		},
		Sockets: func(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
			socketCalls++
			return nil, nil, okStatus("socket")
		},
	})

	snapshot, err := runtime.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if doctorCalls != 1 || processCalls != 0 || socketCalls != 0 {
		t.Fatalf("calls doctor=%d process=%d socket=%d", doctorCalls, processCalls, socketCalls)
	}
	if len(snapshot.Strategies) != 4 {
		t.Fatalf("strategies = %+v", snapshot.Strategies)
	}
	assertCollectorStatus(t, snapshot.CollectorStatus, "process", model.StatusUnsupported)
	assertCollectorStatus(t, snapshot.CollectorStatus, "socket", model.StatusUnsupported)
}

func TestScanReturnsCanceledBeforeStartingCollectors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := NewLocalRuntimeWithDependencies(platform.NewRoot(t.TempDir()), Dependencies{})

	snapshot, err := runtime.Scan(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if snapshot.SchemaVersion != "" {
		t.Fatalf("snapshot = %+v, want zero value", snapshot)
	}
}

func assertCollectorStatus(t *testing.T, statuses []model.CollectorStatus, collector string, want model.Status) {
	t.Helper()
	for _, status := range statuses {
		if status.Collector == collector {
			if status.Status != want {
				t.Fatalf("collector %s status = %s, want %s", collector, status.Status, want)
			}
			return
		}
	}
	t.Fatalf("collector %s not found", collector)
}
