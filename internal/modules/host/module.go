// Package host 实现独立主机扫描模块。
package host

import (
	"context"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Collector 是主机模块的无状态实现。
type Collector struct{}

// New 创建主机模块。
func New() Collector { return Collector{} }

// Descriptor 返回主机模块固定的能力、命令和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "host", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes: []string{"host"}, Commands: coremodule.StandardCommands(),
		RequiredCapabilities: []string{provider.CapabilityHost}, OptionalCapabilities: []string{},
		HardDependencies: []string{}, SoftDependencies: []string{},
		DefaultInterval: "24h", ResourceClass: "light", Timeout: "15s",
	}
}

// Probe 判断当前平台是否提供强类型主机能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.HostProvider](providers, provider.CapabilityHost); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported,
			Reason: "当前平台未提供 host Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 host Provider", Provider: provider.CapabilityHost}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 采集主机事实并转换为统一资产记录。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, _ coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	backend, ok := provider.As[provider.HostProvider](providers, provider.CapabilityHost)
	if !ok {
		status := model.CollectorStatus{
			Collector: provider.CapabilityHost, Status: model.StatusUnsupported,
			StartedAt: started, FinishedAt: time.Now().UTC(), Errors: []string{"缺少 host Provider"},
		}
		return coremodule.Result{Data: coremodule.NewModuleResult("host", status, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
	}
	host, status := backend.Collect(ctx)
	rawDMIUUID := strings.TrimSpace(host.DMIUUID)
	host.DMIUUID = normalizeDMIUUID(rawDMIUUID)
	if rawDMIUUID != "" && host.DMIUUID == "" {
		status.Errors = append(status.Errors, "invalid DMI UUID; treated as missing")
	}
	recordID := RecordID(host)
	confidence := identityConfidence(host)
	if recordID == "" {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, "无法建立主机身份")
		data := coremodule.NewModuleResult(
			"host", status, []string{"host"},
			[]model.AssetRecord{}, []model.RelationshipRecord{},
		)
		return coremodule.Result{Data: data, Internal: host}
	}
	if confidence == "inferred" && (status.Status == model.StatusOK || status.Status == model.StatusComplete) {
		status.Status = model.StatusPartial
		status.Errors = append(status.Errors, "仅能使用 hostname 推断主机身份")
	}
	observedAt := time.Now().UTC()
	name := strings.TrimSpace(host.Hostname)
	if name == "" {
		name = recordID
	}
	record := model.AssetRecord{
		RecordID: recordID, RecordType: "host", HostID: recordID,
		ScopeID: recordID, ScopeType: "host", Name: name,
		Platform:      providers.Platform(),
		States:        model.AssetStates{Installed: true, Running: true, Loaded: true},
		FirstObserved: observedAt, LastObserved: observedAt, Confidence: confidence,
		Attributes: map[string]any{
			"hostname":             host.Hostname,
			"distribution_name":    host.DistributionName,
			"distribution_id":      host.DistributionID,
			"distribution_version": host.DistributionVersion,
			"kernel_release":       host.KernelRelease,
			"architecture":         host.Architecture,
			"memory_total_bytes":   host.MemoryTotalBytes,
			"boot_id":              host.BootID,
			"dmi_uuid":             host.DMIUUID,
		},
		Evidence: []model.Evidence{{
			Provider: provider.CapabilityHost, SourceType: "provider",
			ObservedAt: observedAt, Confidence: confidence,
		}},
	}
	data := coremodule.NewModuleResult("host", status, []string{recordID}, []model.AssetRecord{record}, []model.RelationshipRecord{})
	return coremodule.Result{Data: data, Internal: host}
}

// RecordID 根据现有稳定主机身份或允许的回退字段生成确定性 ID。
func RecordID(host model.Host) string {
	dmiUUID := normalizeDMIUUID(host.DMIUUID)
	if dmiUUID != "" {
		return coremodule.StableRecordID("host", dmiUUID)
	}
	if hostname := strings.TrimSpace(host.Hostname); hostname != "" {
		return coremodule.StableRecordID("host", "", "", hostname)
	}
	return ""
}

func identityConfidence(host model.Host) string {
	dmi := normalizeDMIUUID(host.DMIUUID) != ""
	switch {
	case dmi:
		return "strong"
	case strings.TrimSpace(host.Hostname) != "":
		return "inferred"
	default:
		return ""
	}
}

func normalizeDMIUUID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return ""
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return ""
			}
		default:
			if !((character >= '0' && character <= '9') ||
				(character >= 'a' && character <= 'f') ||
				(character >= 'A' && character <= 'F')) {
				return ""
			}
		}
	}
	normalized := strings.ToLower(value)
	if normalized == "00000000-0000-0000-0000-000000000000" ||
		normalized == "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		return ""
	}
	return normalized
}
