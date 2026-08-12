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

// Dependencies 是 LocalRuntime 使用的可注入采集器集合。
// 测试替换其中单个函数，以验证编排和故障隔离。
type Dependencies struct {
	Doctor    func(context.Context) model.DoctorReport
	Host      func(context.Context) (model.Host, model.CollectorStatus)
	Network   func(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus)
	Processes func(context.Context, string) ([]model.Process, model.CollectorStatus)
	Sockets   func(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus)
}

// defaultDependencies 将生产采集器连接到一个 Linux 文件系统根。
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

// Scan 执行一次完整基线采集，并返回所有成功域和每个域的状态。
// 开始工作前的 context 取消是顶层失败；单个采集器失败只保留在自身采集域。
func (local *LocalRuntime) Scan(ctx context.Context) (model.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return model.Snapshot{}, err
	}
	started := time.Now().UTC()
	var memoryBefore runtime.MemStats
	runtime.ReadMemStats(&memoryBefore)

	snapshot := newSnapshot(started)
	doctor, doctorStatus := invokeDoctor(ctx, local.dependencies.Doctor)
	doctorStatus.Backend = "system_profile"
	snapshot.Capabilities = nonNilCapabilities(doctor.Capabilities)
	snapshot.SystemProfile = doctor.SystemProfile
	hostStrategy := selectStrategy(ModuleHost, snapshot.SystemProfile)
	networkStrategy := selectStrategy(ModuleNetwork, snapshot.SystemProfile)
	processStrategy := selectStrategy(ModuleProcess, snapshot.SystemProfile)
	socketStrategy := selectStrategy(ModuleSocket, snapshot.SystemProfile)
	snapshot.Strategies = []model.CollectionStrategy{hostStrategy, networkStrategy, processStrategy, socketStrategy}

	var hostResult model.Host
	var hostStatus model.CollectorStatus
	if canExecuteStrategy(hostStrategy) {
		hostResult, hostStatus = invokeHost(ctx, local.dependencies.Host)
		hostStatus = applyStrategyEvidence(hostStatus, hostStrategy)
		snapshot.Host = hostResult
	} else {
		hostStatus = skippedCollectorStatus(ModuleHost, hostStrategy)
	}

	var networkStatus model.CollectorStatus
	if canExecuteStrategy(networkStrategy) {
		networkInterfaces, addresses, routes, status := invokeNetwork(ctx, local.dependencies.Network)
		snapshot.NetworkInterfaces = nonNil(networkInterfaces)
		snapshot.Addresses = nonNil(addresses)
		snapshot.Routes = nonNil(routes)
		networkStatus = applyStrategyEvidence(status, networkStrategy)
	} else {
		networkStatus = skippedCollectorStatus(ModuleNetwork, networkStrategy)
	}

	var processStatus model.CollectorStatus
	if canExecuteStrategy(processStrategy) {
		processes, status := invokeProcesses(ctx, local.dependencies.Processes, hostResult.BootID)
		snapshot.Processes = nonNil(processes)
		processStatus = applyStrategyEvidence(status, processStrategy)
	} else {
		processStatus = skippedCollectorStatus(ModuleProcess, processStrategy)
	}

	var socketStatus model.CollectorStatus
	if canExecuteStrategy(socketStrategy) {
		sockets, relationships, status := invokeSockets(ctx, local.dependencies.Sockets, snapshot.Processes)
		snapshot.Sockets = nonNil(sockets)
		snapshot.Relationships = nonNil(relationships)
		socketStatus = applyStrategyEvidence(status, socketStrategy)
	} else {
		socketStatus = skippedCollectorStatus(ModuleSocket, socketStrategy)
	}
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

// newSnapshot 创建符合完整 schema 的文档，并初始化全部集合字段。
func newSnapshot(started time.Time) model.Snapshot {
	return model.Snapshot{
		SchemaName:        model.SnapshotSchemaName,
		SchemaVersion:     model.SchemaVersion,
		Scan:              model.ScanMetadata{ID: fmt.Sprintf("scan-%d", started.UnixNano()), Type: "full", StartedAt: started},
		Agent:             model.AgentInfo{Name: "asset-agent", Version: buildinfo.Version, Commit: buildinfo.Commit, BuildTime: buildinfo.BuildTime},
		Capabilities:      model.CapabilityReport{Items: []model.Capability{}},
		SystemProfile:     model.SystemProfile{SecurityModules: []string{}, ContainerRuntimes: []string{}, AvailableSources: map[string]bool{}},
		Strategies:        []model.CollectionStrategy{},
		NetworkInterfaces: []model.NetworkInterface{}, Addresses: []model.Address{}, Routes: []model.Route{}, Processes: []model.Process{}, Sockets: []model.Socket{},
		Services: []model.Service{}, Packages: []model.Package{}, Containers: []model.Container{}, Files: []model.File{}, Applications: []model.Application{},
		Relationships: []model.Relationship{}, CollectorStatus: []model.CollectorStatus{},
	}
}

// invokeDoctor 在主机/网络超时范围内运行能力检测并隔离 panic。
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

// invokeHost 以 15 秒限制运行主机采集，并将 panic 转为 failed 状态。
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

// invokeNetwork 以 15 秒限制运行网络采集，失败时保留其他采集域。
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

// invokeProcesses 使用主机 boot ID 以 30 秒限制运行进程采集。
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

// invokeSockets 在进程结果可用后，以 30 秒限制运行 socket 采集。
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

// failedStatus 为缺失采集器或已恢复的 panic 创建完整 failed 状态记录。
func failedStatus(collector string, started time.Time, message string) model.CollectorStatus {
	return finishCollectorStatus(model.CollectorStatus{Collector: collector, Status: model.StatusFailed, StartedAt: started, Errors: []string{message}}, started, 0)
}

// normalizeStatus 在采集器返回不完整状态时补齐必填元数据。
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

// finishCollectorStatus 填充时间和对象数，同时保留采集器已有结果。
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

// nonNilCapabilities 保证能力项在 JSON 中为数组。
func nonNilCapabilities(report model.CapabilityReport) model.CapabilityReport {
	report.Items = nonNil(report.Items)
	return report
}

// nonNil 将 nil 切片规范化为空切片，用于内存中的 snapshot。
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
