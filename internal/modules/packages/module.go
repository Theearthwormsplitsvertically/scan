// Package packages 实现已安装软件包资产扫描模块。
package packages

import (
	"context"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Collector 是软件包模块的无状态实现。
type Collector struct{}

// New 创建软件包模块。
func New() Collector { return Collector{} }

// Descriptor 返回软件包模块的依赖、周期和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "package", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes:          []string{"package"},
		RequiredCapabilities: []string{provider.CapabilityPackage}, OptionalCapabilities: []string{},
		HardDependencies: []string{"host"}, SoftDependencies: []string{},
		DefaultInterval: "24h", ResourceClass: "light", Timeout: "2m",
	}
}

// Probe 判断当前平台是否提供强类型 package 能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.PackageProvider](providers, provider.CapabilityPackage); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 package Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 package Provider", Provider: provider.CapabilityPackage}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 采集软件包事实并转换为统一资产记录；版本不进稳定 ID。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	host, ok := hostDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 host 硬依赖结果")
	}
	backend, ok := provider.As[provider.PackageProvider](providers, provider.CapabilityPackage)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 package Provider")
	}
	packages, status := backend.Collect(ctx)
	hostID := hostmodule.RecordID(host)
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0, len(packages))
	for _, item := range packages {
		recordID := coremodule.StableRecordID("package", hostID, item.Architecture, item.Name)
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "package", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: item.Name,
			Version: item.Version, Vendor: item.Maintainer, Platform: providers.Platform(),
			States:        model.AssetStates{Installed: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"architecture":         item.Architecture,
				"package_source":       item.Source,
				"description":          item.Description,
				"installed_size_bytes": item.InstalledSizeBytes,
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilityPackage, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
	}
	data := coremodule.NewModuleResult("package", status, scope(hostID), records, []model.RelationshipRecord{})
	return coremodule.Result{Data: data, Internal: packages}
}

func hostDependency(request coremodule.Request) (model.Host, bool) {
	dependency, ok := request.Dependencies["host"]
	if !ok {
		return model.Host{}, false
	}
	host, ok := dependency.Internal.(model.Host)
	return host, ok
}

func scope(hostID string) []string {
	if hostID == "" {
		return []string{"host"}
	}
	return []string{hostID}
}

func failedResult(started time.Time, status model.Status, message string) coremodule.Result {
	collectorStatus := model.CollectorStatus{
		Collector: "package", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("package", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
