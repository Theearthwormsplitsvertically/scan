// Package network 实现独立网络资产扫描模块。
package network

import (
	"context"
	"fmt"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Facts 保存一次网络采集的原始强类型事实。
type Facts struct {
	Interfaces []model.NetworkInterface
	Addresses  []model.Address
	Routes     []model.Route
}

// Collector 是网络模块的无状态实现。
type Collector struct{}

// New 创建网络模块。
func New() Collector { return Collector{} }

// Descriptor 返回网络模块的依赖、周期和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "network", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes: []string{"network_interface", "address", "route"}, Commands: coremodule.StandardCommands(),
		RequiredCapabilities: []string{provider.CapabilityNetwork}, OptionalCapabilities: []string{},
		HardDependencies: []string{"host"}, SoftDependencies: []string{},
		DefaultInterval: "6h", ResourceClass: "light", Timeout: "30s",
	}
}

// Probe 判断当前平台是否提供强类型网络能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.NetworkProvider](providers, provider.CapabilityNetwork); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 network Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 network Provider", Provider: provider.CapabilityNetwork}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 复用 host 硬依赖中的稳定身份并只发布网络记录。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	host, ok := hostDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 host 硬依赖结果")
	}
	backend, ok := provider.As[provider.NetworkProvider](providers, provider.CapabilityNetwork)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 network Provider")
	}
	interfaces, addresses, routes, status := backend.Collect(ctx)
	hostID := hostmodule.RecordID(host)
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0, len(interfaces)+len(addresses)+len(routes))
	relationships := make([]model.RelationshipRecord, 0, len(addresses)+len(routes))
	interfaceIDs := make(map[string]string, len(interfaces)*2)
	for _, item := range interfaces {
		recordID := coremodule.StableRecordID("network_interface", hostID, item.Namespace, item.Name, item.MAC)
		interfaceIDs[item.Name] = recordID
		interfaceIDs[fmt.Sprintf("#%d", item.Index)] = recordID
		records = append(records, newRecord(providers.Platform(), hostID, recordID, "network_interface", item.Name, observedAt, map[string]any{
			"index": item.Index, "name": item.Name, "mtu": item.MTU, "mac": item.MAC,
			"flags": item.Flags, "namespace": item.Namespace, "dns_digest_sha256": item.DNSDigestSHA,
		}))
	}
	for _, item := range addresses {
		interfaceID := interfaceIDs[fmt.Sprintf("#%d", item.InterfaceIndex)]
		if interfaceID == "" {
			interfaceID = interfaceIDs[item.InterfaceName]
		}
		recordID := coremodule.StableRecordID("address", hostID, item.InterfaceName, item.CIDR)
		records = append(records, newRecord(providers.Platform(), hostID, recordID, "address", item.CIDR, observedAt, map[string]any{
			"interface_index": item.InterfaceIndex, "interface_name": item.InterfaceName,
			"cidr": item.CIDR, "family": item.Family,
		}))
		if interfaceID != "" {
			relationships = append(relationships, newRelationship("assigned_to", recordID, interfaceID, observedAt))
		}
	}
	for _, item := range routes {
		recordID := coremodule.StableRecordID("route", hostID, item.Interface, item.Destination, item.Gateway, fmt.Sprintf("%d", item.Metric))
		records = append(records, newRecord(providers.Platform(), hostID, recordID, "route", item.Destination, observedAt, map[string]any{
			"interface": item.Interface, "destination": item.Destination, "gateway": item.Gateway,
			"metric": item.Metric, "family": item.Family,
		}))
		if interfaceID := interfaceIDs[item.Interface]; interfaceID != "" {
			relationships = append(relationships, newRelationship("uses_interface", recordID, interfaceID, observedAt))
		}
	}
	facts := Facts{Interfaces: interfaces, Addresses: addresses, Routes: routes}
	data := coremodule.NewModuleResult("network", status, []string{hostID}, records, relationships)
	return coremodule.Result{Data: data, Internal: facts}
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
		Collector: "network", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("network", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}

func newRecord(platform, hostID, recordID, recordType, name string, observedAt time.Time, attributes map[string]any) model.AssetRecord {
	return model.AssetRecord{
		RecordID: recordID, RecordType: recordType, HostID: hostID,
		ScopeID: hostID, ScopeType: "host", Name: name, Platform: platform,
		States:        model.AssetStates{Installed: true, Running: true, Loaded: true},
		FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact", Attributes: attributes,
		Evidence: []model.Evidence{{Provider: provider.CapabilityNetwork, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
	}
}

func newRelationship(relationshipType, fromID, toID string, observedAt time.Time) model.RelationshipRecord {
	return model.RelationshipRecord{
		RecordID:         coremodule.StableRecordID("relationship", relationshipType, fromID, toID),
		RelationshipType: relationshipType, FromID: fromID, ToID: toID,
		ObservedAt: observedAt, Confidence: "exact",
		Evidence: []model.Evidence{{Provider: provider.CapabilityNetwork, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
	}
}
