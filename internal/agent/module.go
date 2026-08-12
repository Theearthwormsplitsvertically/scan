package agent

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// Module 是用户可选择的资产扫描模块名称。
type Module string

const (
	ModuleAll     Module = "all"
	ModuleHost    Module = "host"
	ModuleNetwork Module = "network"
	ModuleProcess Module = "process"
	ModuleSocket  Module = "socket"
)

// ScanModule 执行一个资产模块及其必要的最小内部依赖。
func (local *LocalRuntime) ScanModule(ctx context.Context, module Module) (model.ModuleReport, error) {
	if err := ctx.Err(); err != nil {
		return model.ModuleReport{}, err
	}
	if module == ModuleAll {
		return model.ModuleReport{}, fmt.Errorf("module %q uses full scan", module)
	}
	if module != ModuleHost && module != ModuleNetwork && module != ModuleProcess && module != ModuleSocket {
		return model.ModuleReport{}, fmt.Errorf("unknown scan module %q", module)
	}

	started := time.Now().UTC()
	var memoryBefore runtime.MemStats
	runtime.ReadMemStats(&memoryBefore)
	report := newModuleReport(module, started)
	doctor, doctorStatus := invokeDoctor(ctx, local.dependencies.Doctor)
	doctorStatus.Backend = "system_profile"
	report.SystemProfile = doctor.SystemProfile
	strategy := selectStrategy(module, report.SystemProfile)
	report.Strategies = []model.CollectionStrategy{strategy}
	report.CollectorStatus = append(report.CollectorStatus, doctorStatus)

	if !canExecuteStrategy(strategy) {
		report.CollectorStatus = append(report.CollectorStatus, skippedCollectorStatus(module, strategy))
		finishModuleReport(&report, started, memoryBefore)
		return report, nil
	}

	switch module {
	case ModuleHost:
		hostResult, status := invokeHost(ctx, local.dependencies.Host)
		report.Data.Host = &hostResult
		report.CollectorStatus = append(report.CollectorStatus, applyStrategyEvidence(status, strategy))
	case ModuleNetwork:
		interfaces, addresses, routes, status := invokeNetwork(ctx, local.dependencies.Network)
		report.Data.NetworkInterfaces = nonNil(interfaces)
		report.Data.Addresses = nonNil(addresses)
		report.Data.Routes = nonNil(routes)
		report.CollectorStatus = append(report.CollectorStatus, applyStrategyEvidence(status, strategy))
	case ModuleProcess:
		hostResult, hostStatus := invokeHost(ctx, local.dependencies.Host)
		processes, processStatus := invokeProcesses(ctx, local.dependencies.Processes, hostResult.BootID)
		report.Data.Processes = nonNil(processes)
		report.CollectorStatus = append(report.CollectorStatus, hostStatus, applyStrategyEvidence(processStatus, strategy))
	case ModuleSocket:
		hostResult, hostStatus := invokeHost(ctx, local.dependencies.Host)
		processes, processStatus := invokeProcesses(ctx, local.dependencies.Processes, hostResult.BootID)
		sockets, relationships, socketStatus := invokeSockets(ctx, local.dependencies.Sockets, processes)
		report.Data.Sockets = nonNil(sockets)
		report.Data.Relationships = nonNil(relationships)
		report.CollectorStatus = append(report.CollectorStatus, hostStatus, processStatus, applyStrategyEvidence(socketStatus, strategy))
	}

	finishModuleReport(&report, started, memoryBefore)
	return report, nil
}

// finishModuleReport 统一完成扫描计时和 Agent 自身内存用量记录。
func finishModuleReport(report *model.ModuleReport, started time.Time, memoryBefore runtime.MemStats) {
	finished := time.Now().UTC()
	var memoryAfter runtime.MemStats
	runtime.ReadMemStats(&memoryAfter)
	report.Scan.FinishedAt = finished
	report.Scan.DurationMS = finished.Sub(started).Milliseconds()
	report.ResourceUsage = model.ResourceUsage{
		WallTimeMS:     report.Scan.DurationMS,
		HeapAllocBytes: memoryAfter.HeapAlloc,
		HeapDeltaBytes: int64(memoryAfter.HeapAlloc) - int64(memoryBefore.HeapAlloc),
	}
}

func newModuleReport(module Module, started time.Time) model.ModuleReport {
	return model.ModuleReport{
		SchemaName:      model.ModuleReportSchemaName,
		SchemaVersion:   model.SchemaVersion,
		Module:          string(module),
		Scan:            model.ScanMetadata{ID: fmt.Sprintf("scan-%d", started.UnixNano()), Type: "module", StartedAt: started},
		Agent:           model.AgentInfo{Name: "asset-agent", Version: buildinfo.Version, Commit: buildinfo.Commit, BuildTime: buildinfo.BuildTime},
		Strategies:      []model.CollectionStrategy{},
		CollectorStatus: []model.CollectorStatus{},
	}
}
