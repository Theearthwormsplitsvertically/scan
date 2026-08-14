package agent

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

var scanSequence atomic.Uint64

// Scanner 使用动态模块计划顺序执行一次扫描。
type Scanner struct {
	registry  *coremodule.Registry
	providers provider.Lookup
	agent     model.AgentInfo
	clock     func() time.Time
}

// NewScanner 使用生产构建信息和 UTC 系统时钟创建扫描器。
func NewScanner(registry *coremodule.Registry, providers provider.Lookup) *Scanner {
	return NewScannerWithClock(registry, providers, model.AgentInfo{
		Name: "asset-agent", Version: buildinfo.Version, Commit: buildinfo.Commit, BuildTime: buildinfo.BuildTime,
	}, time.Now)
}

// NewScannerWithClock 创建可使用确定性时钟测试的扫描器。
func NewScannerWithClock(registry *coremodule.Registry, providers provider.Lookup, agentInfo model.AgentInfo, clock func() time.Time) *Scanner {
	if clock == nil {
		clock = time.Now
	}
	return &Scanner{registry: registry, providers: providers, agent: agentInfo, clock: clock}
}

// Doctor 返回平台 Provider 诊断；没有 ProfileProvider 时返回通用明确结果。
func (scanner *Scanner) Doctor(ctx context.Context) (model.DoctorReport, error) {
	if err := ctx.Err(); err != nil {
		return model.DoctorReport{}, err
	}
	if backend, ok := provider.As[provider.ProfileProvider](scanner.providers, provider.CapabilitySystemProfile); ok {
		return backend.Detect(ctx), nil
	}
	platform := "unknown"
	if scanner.providers != nil && scanner.providers.Platform() != "" {
		platform = scanner.providers.Platform()
	}
	return model.DoctorReport{
		SchemaVersion: model.DoctorSchemaVersion, Agent: scanner.agent, OS: platform, Root: false,
		Capabilities: model.CapabilityReport{Items: []model.Capability{}},
		SystemProfile: model.SystemProfile{
			OS: platform, SecurityModules: []string{}, ContainerRuntimes: []string{}, AvailableSources: map[string]bool{},
		},
	}, nil
}

// Modules 按名称返回动态注册模块及其平台支持状态。
func (scanner *Scanner) Modules(ctx context.Context) ([]coremodule.Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	descriptors := scanner.registry.List()
	infos := make([]coremodule.Info, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, ok := scanner.registry.Lookup(descriptor.Name)
		if !ok {
			return nil, fmt.Errorf("模块 %q 在探测前从注册表消失", descriptor.Name)
		}
		infos = append(infos, coremodule.Info{Descriptor: normalizeDescriptor(descriptor), Support: invokeProbe(ctx, item, scanner.providers)})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Descriptor.Name < infos[j].Descriptor.Name })
	return infos, nil
}

// Scan 执行全量扫描或显式模块集合及其硬依赖。
func (scanner *Scanner) Scan(ctx context.Context, selection ScanSelection) (ScanOutcome, error) {
	if err := ctx.Err(); err != nil {
		return ScanOutcome{}, err
	}
	if selection.All && len(selection.Modules) > 0 {
		return ScanOutcome{}, fmt.Errorf("all scan cannot include selected modules")
	}
	if !selection.All && len(selection.Modules) == 0 {
		return ScanOutcome{}, fmt.Errorf("no modules selected")
	}
	selected := make(map[string]bool, len(selection.Modules))
	selectedNames := make([]string, 0, len(selection.Modules))
	for _, name := range selection.Modules {
		if selected[name] {
			continue
		}
		selected[name] = true
		selectedNames = append(selectedNames, name)
	}
	sort.Strings(selectedNames)

	var plan []coremodule.Module
	var err error
	requestedModule := "all"
	batchType := model.BatchTypeSnapshot
	if selection.All {
		plan, err = scanner.registry.PlanAll()
	} else {
		plan, err = scanner.registry.PlanSelected(selectedNames)
		batchType = model.BatchTypeModule
		requestedModule = selectedNames[0]
		if len(selectedNames) > 1 {
			requestedModule = "multi"
		}
	}
	if err != nil {
		return ScanOutcome{}, err
	}
	started := scanner.clock().UTC()
	platform := "unknown"
	if scanner.providers != nil && scanner.providers.Platform() != "" {
		platform = scanner.providers.Platform()
	}
	batch := model.Batch{
		SchemaName: model.BatchSchemaName, SchemaVersion: model.BatchSchemaVersion,
		ID: newScanID(started), Type: batchType, RequestedModule: requestedModule,
		Platform: platform, Agent: scanner.agent, StartedAt: started, Results: []model.ModuleResult{},
	}
	executed := make(map[string]coremodule.Result, len(plan))
	recordCounts := make(map[string]int, len(plan))
	for _, item := range plan {
		if err := ctx.Err(); err != nil {
			return ScanOutcome{}, err
		}
		descriptor := item.Descriptor()
		request := coremodule.Request{Dependencies: dependencyResults(descriptor, executed)}
		result, blocked := blockedResult(descriptor, request.Dependencies)
		if !blocked {
			result = invokeModule(ctx, item, scanner.providers, request)
			result = constrainByDependencies(result, descriptor, request.Dependencies)
		}
		result = normalizeCollectedResult(result, descriptor.Name)
		executed[descriptor.Name] = result
		recordCounts[descriptor.Name] = len(result.Data.Records)

		published := selection.All || selected[descriptor.Name]
		output := result.Data
		output.Published = published
		if !published {
			output.Records = []model.AssetRecord{}
			output.Relationships = []model.RelationshipRecord{}
		}
		batch.Results = append(batch.Results, output)
	}
	batch.FinishedAt = scanner.clock().UTC()
	return ScanOutcome{Batch: batch, RecordCounts: recordCounts}, nil
}

func dependencyResults(descriptor coremodule.Descriptor, executed map[string]coremodule.Result) map[string]coremodule.Result {
	result := make(map[string]coremodule.Result)
	for _, name := range append(append([]string{}, descriptor.HardDependencies...), descriptor.SoftDependencies...) {
		if dependency, ok := executed[name]; ok {
			result[name] = dependency
		}
	}
	return result
}

func blockedResult(descriptor coremodule.Descriptor, dependencies map[string]coremodule.Result) (coremodule.Result, bool) {
	for _, name := range descriptor.HardDependencies {
		dependency, ok := dependencies[name]
		if !ok {
			return failureResult(descriptor.Name, model.StatusFailed, "dependency_failed", "缺少硬依赖 "+name), true
		}
		switch dependency.Data.Status {
		case model.StatusFailed, model.StatusTimeout:
			return failureResult(descriptor.Name, model.StatusFailed, "dependency_failed", "硬依赖 "+name+" 执行失败"), true
		case model.StatusUnsupported:
			return failureResult(descriptor.Name, model.StatusUnsupported, "dependency_failed", "硬依赖 "+name+" 不受支持"), true
		}
	}
	return coremodule.Result{}, false
}

func constrainByDependencies(result coremodule.Result, descriptor coremodule.Descriptor, dependencies map[string]coremodule.Result) coremodule.Result {
	if result.Data.Status != model.StatusComplete {
		return result
	}
	for _, name := range descriptor.HardDependencies {
		dependency := dependencies[name]
		if dependency.Data.Status == model.StatusPartial || dependency.Data.Status == model.StatusDegraded {
			result.Data.Status = model.StatusPartial
			result.Data.Authoritative = false
			result.Data.Errors = append(result.Data.Errors, model.ErrorDetail{
				Code: "dependency_partial", Message: "硬依赖 " + name + " 结果不完整",
			})
		}
	}
	for _, name := range descriptor.SoftDependencies {
		dependency, ok := dependencies[name]
		if !ok {
			result = constrainBySoftDependency(result, "soft_dependency_unavailable", "软依赖 "+name+" 结果缺失")
			continue
		}
		switch dependency.Data.Status {
		case model.StatusComplete:
			continue
		case model.StatusPartial, model.StatusDegraded:
			result = constrainBySoftDependency(result, "soft_dependency_partial", "软依赖 "+name+" 结果不完整")
		default:
			result = constrainBySoftDependency(result, "soft_dependency_unavailable", "软依赖 "+name+" 不可用")
		}
	}
	return result
}

func constrainBySoftDependency(result coremodule.Result, code, message string) coremodule.Result {
	result.Data.Status = model.StatusPartial
	result.Data.Authoritative = false
	result.Data.Errors = append(result.Data.Errors, model.ErrorDetail{Code: code, Message: message})
	return result
}

func normalizeDescriptor(descriptor coremodule.Descriptor) coremodule.Descriptor {
	descriptor.RecordTypes = scannerNonNil(descriptor.RecordTypes)
	descriptor.RequiredCapabilities = scannerNonNil(descriptor.RequiredCapabilities)
	descriptor.OptionalCapabilities = scannerNonNil(descriptor.OptionalCapabilities)
	descriptor.HardDependencies = scannerNonNil(descriptor.HardDependencies)
	descriptor.SoftDependencies = scannerNonNil(descriptor.SoftDependencies)
	return descriptor
}

func newScanID(started time.Time) string {
	return fmt.Sprintf("scan-%s-%06d", started.Format("20060102T150405.000000000Z"), scanSequence.Add(1))
}

func scannerNonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
