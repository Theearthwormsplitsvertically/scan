// Package service 实现 systemd Unit 服务资产扫描模块。
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Collector 是服务模块的无状态实现。
type Collector struct{}

// New 创建服务模块。
func New() Collector { return Collector{} }

// Descriptor 返回服务模块的依赖、周期和资源约束。
// host 是硬依赖（稳定归属）；process 是软依赖（cgroup 运行态，失败不阻断）。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "service", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes:          []string{"service"},
		RequiredCapabilities: []string{provider.CapabilityService}, OptionalCapabilities: []string{},
		HardDependencies: []string{"host"}, SoftDependencies: []string{"process"},
		DefaultInterval: "12h", ResourceClass: "medium", Timeout: "2m",
	}
}

// Probe 判断当前平台是否提供强类型 service 能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.ServiceProvider](providers, provider.CapabilityService); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 service Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 service Provider", Provider: provider.CapabilityService}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 发布 service 静态事实，并用 process 软依赖的 cgroup 事实推导运行态。
// 当前 Provider 只读 Unit 文件，无法取得真实 MainPID 与 systemd 状态，
// 因此不发布 main_pid / unit_state，也不发布 runs_as 关系，避免伪造证据。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	host, ok := hostDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 host 硬依赖结果")
	}
	backend, ok := provider.As[provider.ServiceProvider](providers, provider.CapabilityService)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 service Provider")
	}
	services, status := backend.Collect(ctx)
	hostID := hostmodule.RecordID(host)
	processes := processFacts(request) // 软依赖，缺失时按未运行处理
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0, len(services))
	for _, item := range services {
		recordID := coremodule.StableRecordID("service", hostID, item.UnitName)
		running := len(matchingPIDs(item.UnitName, processes)) > 0
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "service", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: item.UnitName + ".service", Platform: providers.Platform(),
			States:        model.AssetStates{Installed: true, Running: running, Loaded: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"unit_name":          item.UnitName,
				"service_manager":    "systemd",
				"native_service_key": item.UnitName,
				"exec_start":         item.ExecStart,
				"user":               item.User,
				"group":              item.Group,
				"load_state":         item.LoadState,
				"fragment_path":      item.FragmentPath,
				"description":        item.Description,
				"wanted_by":          item.WantedBy,
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilityService, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}
	data := coremodule.NewModuleResult("service", status, scope(hostID), records, []model.RelationshipRecord{})
	return coremodule.Result{Data: data, Internal: services}
}

// matchingPIDs 返回 cgroup 归属该 Unit 的进程 PID，按升序排列。
func matchingPIDs(unitName string, processes []model.Process) []int {
	suffix := "/" + systemdEscape(unitName) + ".service"
	pids := make([]int, 0)
	for _, process := range processes {
		for _, cgroup := range process.Cgroups {
			if strings.HasSuffix(cgroup, suffix) {
				pids = append(pids, process.PID)
				break
			}
		}
	}
	sort.Ints(pids)
	return pids
}

// systemdEscape 按 systemd 的 cgroup 路径转义规则转义 Unit 名，安全集之外的字节转为 \xNN。
func systemdEscape(name string) string {
	var builder strings.Builder
	for index := 0; index < len(name); index++ {
		character := name[index]
		if isSystemdSafe(character) {
			builder.WriteByte(character)
		} else {
			fmt.Fprintf(&builder, "\\x%02x", character)
		}
	}
	return builder.String()
}

func isSystemdSafe(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9') ||
		character == ':' || character == '_' || character == '.' || character == '@' || character == '-'
}

func hostDependency(request coremodule.Request) (model.Host, bool) {
	dependency, ok := request.Dependencies["host"]
	if !ok {
		return model.Host{}, false
	}
	host, ok := dependency.Internal.(model.Host)
	return host, ok
}

func processFacts(request coremodule.Request) []model.Process {
	dependency, ok := request.Dependencies["process"]
	if !ok {
		return nil
	}
	processes, _ := dependency.Internal.([]model.Process)
	return processes
}

func scope(hostID string) []string {
	if hostID == "" {
		return []string{"host"}
	}
	return []string{hostID}
}

func failedResult(started time.Time, status model.Status, message string) coremodule.Result {
	collectorStatus := model.CollectorStatus{
		Collector: "service", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("service", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
