package agent

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	"github.com/Theearthwormsplitsvertically/scan/internal/capability"
	"github.com/Theearthwormsplitsvertically/scan/internal/collect/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/collect/network"
	"github.com/Theearthwormsplitsvertically/scan/internal/collect/process"
	"github.com/Theearthwormsplitsvertically/scan/internal/collect/socket"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// Dependencies is the injectable collector set used by LocalRuntime.
// Tests replace individual functions to verify orchestration and failure isolation.
type Dependencies struct {
	Doctor    func(context.Context) model.DoctorReport
	Host      func(context.Context) (model.Host, model.CollectorStatus)
	Network   func(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus)
	Processes func(context.Context, string) ([]model.Process, model.CollectorStatus)
	Sockets   func(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus)
}

// defaultDependencies wires the production collectors to one Linux filesystem root.
func defaultDependencies(root platform.Root) Dependencies {
	return Dependencies{
		Doctor: func(ctx context.Context) model.DoctorReport {
			return capability.Detect(ctx, root, runtime.GOARCH)
		},
		Host: func(ctx context.Context) (model.Host, model.CollectorStatus) {
			return host.Collect(ctx, root)
		},
		Network: func(ctx context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
			return network.Collect(ctx, root, network.SystemInterfaceSource{})
		},
		Processes: func(ctx context.Context, bootID string) ([]model.Process, model.CollectorStatus) {
			return process.Collect(ctx, root, bootID)
		},
		Sockets: func(ctx context.Context, processes []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
			return socket.Collect(ctx, root, processes)
		},
	}
}

// Scan runs one full baseline collection and returns all successful domains plus per-domain status.
// Context cancellation before work is fatal; individual collector failures remain local to that domain.
func (local *LocalRuntime) Scan(ctx context.Context) (model.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return model.Snapshot{}, err
	}
	started := time.Now().UTC()
	var memoryBefore runtime.MemStats
	runtime.ReadMemStats(&memoryBefore)

	snapshot := newSnapshot(started)
	doctor, doctorStatus := invokeDoctor(ctx, local.dependencies.Doctor)
	snapshot.Capabilities = nonNilCapabilities(doctor.Capabilities)
	hostResult, hostStatus := invokeHost(ctx, local.dependencies.Host)
	snapshot.Host = hostResult
	networkInterfaces, addresses, routes, networkStatus := invokeNetwork(ctx, local.dependencies.Network)
	snapshot.NetworkInterfaces = nonNil(networkInterfaces)
	snapshot.Addresses = nonNil(addresses)
	snapshot.Routes = nonNil(routes)
	processes, processStatus := invokeProcesses(ctx, local.dependencies.Processes, hostResult.BootID)
	snapshot.Processes = nonNil(processes)
	sockets, relationships, socketStatus := invokeSockets(ctx, local.dependencies.Sockets, snapshot.Processes)
	snapshot.Sockets = nonNil(sockets)
	snapshot.Relationships = nonNil(relationships)
	snapshot.CollectorStatus = []model.CollectorStatus{doctorStatus, hostStatus, networkStatus, processStatus, socketStatus}

	finished := time.Now().UTC()
	var memoryAfter runtime.MemStats
	runtime.ReadMemStats(&memoryAfter)
	snapshot.Scan.FinishedAt = finished
	snapshot.Scan.DurationMS = finished.Sub(started).Milliseconds()
	snapshot.ResourceUsage = model.ResourceUsage{
		WallTimeMS:     snapshot.Scan.DurationMS,
		HeapAllocBytes: memoryAfter.HeapAlloc,
		HeapDeltaBytes: int64(memoryAfter.HeapAlloc) - int64(memoryBefore.HeapAlloc),
	}
	return snapshot, nil
}

// newSnapshot creates a complete schema-shaped document with all collections initialized.
func newSnapshot(started time.Time) model.Snapshot {
	return model.Snapshot{
		SchemaVersion:     model.SchemaVersion,
		Scan:              model.ScanMetadata{ID: fmt.Sprintf("scan-%d", started.UnixNano()), Type: "full", StartedAt: started},
		Agent:             model.AgentInfo{Name: "asset-agent", Version: buildinfo.Version, Commit: buildinfo.Commit, BuildTime: buildinfo.BuildTime},
		Capabilities:      model.CapabilityReport{Items: []model.Capability{}},
		NetworkInterfaces: []model.NetworkInterface{}, Addresses: []model.Address{}, Routes: []model.Route{}, Processes: []model.Process{}, Sockets: []model.Socket{},
		Services: []model.Service{}, Packages: []model.Package{}, Containers: []model.Container{}, Files: []model.File{}, Applications: []model.Application{},
		Relationships: []model.Relationship{}, CollectorStatus: []model.CollectorStatus{},
	}
}

// invokeDoctor runs capability detection with a host/network timeout and panic isolation.
func invokeDoctor(ctx context.Context, collector func(context.Context) model.DoctorReport) (report model.DoctorReport, status model.CollectorStatus) {
	started := time.Now().UTC()
	status = model.CollectorStatus{Collector: "capability", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	defer func() {
		if recovered := recover(); recovered != nil {
			status.Status = model.StatusFailed
			status.Errors = append(status.Errors, fmt.Sprintf("panic: %v", recovered))
			report = model.DoctorReport{}
		}
		status = finishCollectorStatus(status, started, len(report.Capabilities.Items))
	}()
	if collector == nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, "collector unavailable")
		return report, status
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	report = collector(bounded)
	return report, status
}

// invokeHost runs host collection with a 15-second limit and converts panic to failed status.
func invokeHost(ctx context.Context, collector func(context.Context) (model.Host, model.CollectorStatus)) (result model.Host, status model.CollectorStatus) {
	started := time.Now().UTC()
	defer func() {
		if recovered := recover(); recovered != nil {
			status = failedStatus("host", started, fmt.Sprintf("panic: %v", recovered))
			result = model.Host{}
		}
		status = normalizeStatus(status, "host", started, 1)
	}()
	if collector == nil {
		return result, failedStatus("host", started, "collector unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return collector(bounded)
}

// invokeNetwork runs network collection with a 15-second limit and preserves other domains on failure.
func invokeNetwork(ctx context.Context, collector func(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus)) (interfaces []model.NetworkInterface, addresses []model.Address, routes []model.Route, status model.CollectorStatus) {
	started := time.Now().UTC()
	defer func() {
		if recovered := recover(); recovered != nil {
			interfaces, addresses, routes = []model.NetworkInterface{}, []model.Address{}, []model.Route{}
			status = failedStatus("network", started, fmt.Sprintf("panic: %v", recovered))
		}
		status = normalizeStatus(status, "network", started, len(interfaces)+len(addresses)+len(routes))
	}()
	if collector == nil {
		return interfaces, addresses, routes, failedStatus("network", started, "collector unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return collector(bounded)
}

// invokeProcesses runs process collection with a 30-second limit and the host boot ID.
func invokeProcesses(ctx context.Context, collector func(context.Context, string) ([]model.Process, model.CollectorStatus), bootID string) (processes []model.Process, status model.CollectorStatus) {
	started := time.Now().UTC()
	defer func() {
		if recovered := recover(); recovered != nil {
			processes = []model.Process{}
			status = failedStatus("process", started, fmt.Sprintf("panic: %v", recovered))
		}
		status = normalizeStatus(status, "process", started, len(processes))
	}()
	if collector == nil {
		return processes, failedStatus("process", started, "collector unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return collector(bounded, bootID)
}

// invokeSockets runs socket collection with a 30-second limit after processes are available.
func invokeSockets(ctx context.Context, collector func(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus), processes []model.Process) (sockets []model.Socket, relationships []model.Relationship, status model.CollectorStatus) {
	started := time.Now().UTC()
	defer func() {
		if recovered := recover(); recovered != nil {
			sockets, relationships = []model.Socket{}, []model.Relationship{}
			status = failedStatus("socket", started, fmt.Sprintf("panic: %v", recovered))
		}
		status = normalizeStatus(status, "socket", started, len(sockets))
	}()
	if collector == nil {
		return sockets, relationships, failedStatus("socket", started, "collector unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return collector(bounded, processes)
}

// failedStatus creates a complete failed status record for a missing collector or recovered panic.
func failedStatus(collector string, started time.Time, message string) model.CollectorStatus {
	return finishCollectorStatus(model.CollectorStatus{Collector: collector, Status: model.StatusFailed, StartedAt: started, Errors: []string{message}}, started, 0)
}

// normalizeStatus fills required metadata when a collector returns an incomplete status record.
func normalizeStatus(status model.CollectorStatus, collector string, started time.Time, objects int) model.CollectorStatus {
	if status.Collector == "" {
		status.Collector = collector
	}
	if status.Status == "" {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, "collector returned no status")
	}
	if status.Errors == nil {
		status.Errors = []string{}
	}
	if status.StartedAt.IsZero() {
		status.StartedAt = started
	}
	return finishCollectorStatus(status, started, objects)
}

// finishCollectorStatus stamps timing and object count while retaining the collector's outcome.
func finishCollectorStatus(status model.CollectorStatus, started time.Time, objects int) model.CollectorStatus {
	if status.FinishedAt.IsZero() {
		status.FinishedAt = time.Now().UTC()
	}
	if status.StartedAt.IsZero() {
		status.StartedAt = started
	}
	status.DurationMS = status.FinishedAt.Sub(status.StartedAt).Milliseconds()
	status.Objects = objects
	return status
}

// nonNilCapabilities guarantees a JSON array for capability items.
func nonNilCapabilities(report model.CapabilityReport) model.CapabilityReport {
	report.Items = nonNil(report.Items)
	return report
}

// nonNil normalizes a nil slice to an empty slice for the in-memory snapshot.
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
