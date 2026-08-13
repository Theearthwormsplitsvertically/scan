// Package process 实现独立进程资产扫描模块。
package process

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Collector 是进程模块的无状态实现。
type Collector struct{}

// New 创建进程模块。
func New() Collector { return Collector{} }

// Descriptor 返回进程模块的依赖、周期和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "process", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes: []string{"process"}, Commands: coremodule.StandardCommands(),
		RequiredCapabilities: []string{provider.CapabilityProcess}, OptionalCapabilities: []string{},
		HardDependencies: []string{"host"}, SoftDependencies: []string{},
		DefaultInterval: "12h", ResourceClass: "medium", Timeout: "2m",
	}
}

// Probe 判断当前平台是否提供强类型进程能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.ProcessProvider](providers, provider.CapabilityProcess); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 process Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 process Provider", Provider: provider.CapabilityProcess}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 从 host 硬依赖取得 Boot ID，再采集并发布进程记录。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	host, ok := hostDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 host 硬依赖结果")
	}
	backend, ok := provider.As[provider.ProcessProvider](providers, provider.CapabilityProcess)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 process Provider")
	}
	processes, status := backend.Collect(ctx, host.BootID)
	hostID := hostmodule.RecordID(host)
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0, len(processes))
	relationships := make([]model.RelationshipRecord, 0, len(processes))
	processIDs := make(map[int]string, len(processes))
	for _, item := range processes {
		stableKey := strings.TrimSpace(item.ID)
		if stableKey == "" {
			stableKey = fmt.Sprintf("%s:%d:%d", host.BootID, item.PID, item.StartTime)
		}
		recordID := coremodule.StableRecordID("process", hostID, stableKey)
		processIDs[item.PID] = recordID
		name := item.Name
		if name == "" {
			name = item.Executable
		}
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "process", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: name, Platform: providers.Platform(),
			States:        model.AssetStates{Running: true, Loaded: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"process_id": item.ID, "pid": item.PID, "ppid": item.PPID,
				"start_time": item.StartTime, "state": item.State, "uid": item.UID, "gid": item.GID,
				"executable": item.Executable, "command_line": item.CommandLine,
				"working_dir": item.WorkingDir, "root_dir": item.RootDir, "cgroups": item.Cgroups,
				"mount_namespace": item.MountNS, "network_namespace": item.NetworkNS, "pid_namespace": item.PIDNS,
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilityProcess, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}
	for _, item := range processes {
		childID := processIDs[item.PID]
		parentID := processIDs[item.PPID]
		if childID == "" || parentID == "" {
			continue
		}
		relationships = append(relationships, model.RelationshipRecord{
			RecordID:         coremodule.StableRecordID("relationship", "child_of", childID, parentID),
			RelationshipType: "child_of", FromID: childID, ToID: parentID,
			ObservedAt: observedAt, Confidence: "exact",
			Evidence: []model.Evidence{{Provider: provider.CapabilityProcess, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}
	data := coremodule.NewModuleResult("process", status, []string{hostID}, records, relationships)
	return coremodule.Result{Data: data, Internal: processes}
}

func hostDependency(request coremodule.Request) (model.Host, bool) {
	dependency, ok := request.Dependencies["host"]
	if !ok {
		return model.Host{}, false
	}
	host, ok := dependency.Internal.(model.Host)
	return host, ok
}

func failedResult(started time.Time, status model.Status, message string) coremodule.Result {
	collectorStatus := model.CollectorStatus{
		Collector: "process", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("process", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
