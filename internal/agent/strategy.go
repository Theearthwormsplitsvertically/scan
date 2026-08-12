package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// selectStrategy 根据扫描前生成的系统画像，为一个公开模块选择实际采集后端。
// RequiredSources 表示理想策略会使用的数据源；MissingSources 和 Reason 解释降级依据。
func selectStrategy(module Module, profile model.SystemProfile) model.CollectionStrategy {
	sources := profile.AvailableSources
	has := func(name string) bool { return sources != nil && sources[name] }

	switch module {
	case ModuleHost:
		strategy := newStrategy(module, []string{"procfs", "sysfs"}, sources)
		switch {
		case has("procfs") && has("sysfs"):
			strategy.Backend = "procfs_sysfs"
			strategy.Status = model.StatusOK
		case has("procfs"):
			strategy.Backend = "procfs"
			strategy.Status = model.StatusDegraded
			strategy.Reason = "sysfs unavailable; hardware identity may be incomplete"
		case has("sysfs"):
			strategy.Backend = "sysfs"
			strategy.Status = model.StatusDegraded
			strategy.Reason = "procfs unavailable; runtime host facts may be incomplete"
		default:
			markUnavailable(&strategy)
		}
		return strategy
	case ModuleNetwork:
		strategy := newStrategy(module, []string{"standard_library_network", "procfs"}, sources)
		if !has("standard_library_network") {
			markUnavailable(&strategy)
		} else if has("procfs") {
			strategy.Backend = "standard_library_procfs"
			strategy.Status = model.StatusOK
		} else {
			strategy.Backend = "standard_library"
			strategy.Status = model.StatusDegraded
			strategy.Reason = "procfs unavailable; route information may be incomplete"
		}
		return strategy
	case ModuleProcess:
		strategy := newStrategy(module, []string{"procfs"}, sources)
		if has("procfs") {
			strategy.Backend = "procfs"
			strategy.Status = model.StatusOK
		} else {
			markUnavailable(&strategy)
		}
		return strategy
	case ModuleSocket:
		strategy := newStrategy(module, []string{"procfs", "proc_net"}, sources)
		if has("procfs") && has("proc_net") {
			strategy.Backend = "proc_net"
			strategy.Status = model.StatusDegraded
			strategy.Reason = "sock_diag backend unavailable; using /proc/net fallback"
		} else {
			markUnavailable(&strategy)
		}
		return strategy
	default:
		return model.CollectionStrategy{
			Module: string(module), Backend: "unavailable", RequiredSources: []string{}, MissingSources: []string{},
			Status: model.StatusUnsupported, Reason: fmt.Sprintf("unsupported module %q", module),
		}
	}
}

// newStrategy 构造策略证据并计算系统画像中缺失的理想数据源。
func newStrategy(module Module, required []string, available map[string]bool) model.CollectionStrategy {
	strategy := model.CollectionStrategy{
		Module: string(module), RequiredSources: append([]string(nil), required...), MissingSources: []string{},
	}
	for _, source := range required {
		if available == nil || !available[source] {
			strategy.MissingSources = append(strategy.MissingSources, source)
		}
	}
	return strategy
}

// markUnavailable 表示模块缺少不可替代的数据源，因此本次不会调用其采集器。
func markUnavailable(strategy *model.CollectionStrategy) {
	strategy.Backend = "unavailable"
	strategy.Status = model.StatusUnsupported
	strategy.Reason = "required data source unavailable: " + strings.Join(strategy.MissingSources, ", ")
}

// canExecuteStrategy 判断选出的后端是否足以安全地执行模块。
func canExecuteStrategy(strategy model.CollectionStrategy) bool {
	return strategy.Backend != "unavailable" && strategy.Status != model.StatusUnsupported && strategy.Status != model.StatusFailed
}

// applyStrategyEvidence 把策略后端和降级原因同步到采集器状态，便于只看状态数组时定位问题。
func applyStrategyEvidence(status model.CollectorStatus, strategy model.CollectionStrategy) model.CollectorStatus {
	status.Backend = strategy.Backend
	if strategy.Status == model.StatusDegraded && status.Fallback == "" {
		status.Fallback = strategy.Reason
	}
	return status
}

// skippedCollectorStatus 为因数据源不足而跳过的模块生成可审计状态。
func skippedCollectorStatus(module Module, strategy model.CollectionStrategy) model.CollectorStatus {
	started := time.Now().UTC()
	status := model.CollectorStatus{
		Collector: string(module), Status: model.StatusUnsupported, StartedAt: started,
		Errors: []string{strategy.Reason}, Backend: strategy.Backend,
	}
	return finishCollectorStatus(status, started, 0)
}
