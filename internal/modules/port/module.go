// Package port 将原始 socket 事实转换为监听端口资产。
package port

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Facts 保存全部 socket 事实，供 connection 模块复用而不重复采集。
type Facts struct {
	Sockets                    []model.Socket
	SocketProcessRelationships []model.Relationship
	Status                     model.CollectorStatus
	HostID                     string
	ProcessRecordIDs           map[string]string
	PIDRecordIDs               map[int]string
}

// Collector 是监听端口模块的无状态实现。
type Collector struct{}

// New 创建监听端口模块。
func New() Collector { return Collector{} }

// Descriptor 返回监听端口模块的依赖、周期和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "port", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes: []string{"port"}, Commands: coremodule.StandardCommands(),
		RequiredCapabilities: []string{provider.CapabilitySocket}, OptionalCapabilities: []string{},
		HardDependencies: []string{"process"}, SoftDependencies: []string{},
		DefaultInterval: "1h", ResourceClass: "medium", Timeout: "2m",
	}
}

// Probe 判断当前平台是否提供强类型 socket 能力。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.SocketProvider](providers, provider.CapabilitySocket); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 socket Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 socket Provider", Provider: provider.CapabilitySocket}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 只发布监听语义，但把全部 socket 事实保存在 Internal 中。
func (Collector) Collect(ctx context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	processResult, processes, ok := processDependency(request)
	if !ok {
		return failedResult(started, model.StatusFailed, "缺少 process 硬依赖结果")
	}
	backend, ok := provider.As[provider.SocketProvider](providers, provider.CapabilitySocket)
	if !ok {
		return failedResult(started, model.StatusUnsupported, "缺少 socket Provider")
	}
	sockets, rawRelationships, status := backend.Collect(ctx, processes)
	processRecordIDs, pidRecordIDs, hostID := processRecordMaps(processResult)
	facts := Facts{
		Sockets: sockets, SocketProcessRelationships: rawRelationships, Status: status,
		HostID: hostID, ProcessRecordIDs: processRecordIDs, PIDRecordIDs: pidRecordIDs,
	}
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0)
	relationships := make([]model.RelationshipRecord, 0)
	portIDs := make(map[string]string)
	seenOwners := make(map[string]bool)
	for _, socket := range sockets {
		if !isListening(socket) {
			continue
		}
		recordID := coremodule.StableRecordID("port", hostID, socket.NetworkNS, socket.Protocol,
			fmt.Sprintf("%d", socket.Family), socket.LocalAddress, fmt.Sprintf("%d", socket.LocalPort))
		portIDs[socket.ID] = recordID
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "port", HostID: hostID,
			ScopeID: hostID, ScopeType: "host", Name: fmt.Sprintf("%s/%d", socket.Protocol, socket.LocalPort),
			Platform: providers.Platform(), States: model.AssetStates{Running: true, Loaded: true, Exposed: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"socket_id": socket.ID, "protocol": socket.Protocol, "family": socket.Family,
				"state": socket.State, "local_address": socket.LocalAddress, "local_port": socket.LocalPort,
				"inode": socket.Inode, "network_namespace": socket.NetworkNS,
				"pids": socket.PIDs, "process_ids": socket.ProcessIDs,
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilitySocket, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
		})
		for _, rawID := range socket.ProcessIDs {
			appendOwnership(&relationships, seenOwners, processRecordIDs[rawID], recordID, observedAt)
		}
		for _, pid := range socket.PIDs {
			appendOwnership(&relationships, seenOwners, pidRecordIDs[pid], recordID, observedAt)
		}
	}
	for _, relationship := range rawRelationships {
		portID := portIDs[relationship.FromID]
		if portID == "" {
			continue
		}
		processID := processRecordIDs[relationship.ToID]
		if processID == "" {
			processID = processRecordIDs[relationship.FromID]
			portID = portIDs[relationship.ToID]
		}
		appendOwnership(&relationships, seenOwners, processID, portID, observedAt)
	}
	data := coremodule.NewModuleResult("port", status, scope(hostID), records, relationships)
	return coremodule.Result{Data: data, Internal: facts}
}

func processDependency(request coremodule.Request) (coremodule.Result, []model.Process, bool) {
	dependency, ok := request.Dependencies["process"]
	if !ok {
		return coremodule.Result{}, nil, false
	}
	processes, ok := dependency.Internal.([]model.Process)
	return dependency, processes, ok
}

func processRecordMaps(result coremodule.Result) (map[string]string, map[int]string, string) {
	byRawID := make(map[string]string)
	byPID := make(map[int]string)
	hostID := ""
	for _, record := range result.Data.Records {
		if record.RecordType != "process" {
			continue
		}
		if hostID == "" {
			hostID = record.HostID
		}
		if rawID, ok := record.Attributes["process_id"].(string); ok && rawID != "" {
			byRawID[rawID] = record.RecordID
		}
		if pid, ok := record.Attributes["pid"].(int); ok {
			byPID[pid] = record.RecordID
		}
	}
	if hostID == "" && len(result.Data.Coverage.ExpectedScopes) > 0 {
		hostID = result.Data.Coverage.ExpectedScopes[0]
	}
	return byRawID, byPID, hostID
}

func appendOwnership(relationships *[]model.RelationshipRecord, seen map[string]bool, processID, portID string, observedAt time.Time) {
	if processID == "" || portID == "" {
		return
	}
	key := processID + "\x00" + portID
	if seen[key] {
		return
	}
	seen[key] = true
	*relationships = append(*relationships, model.RelationshipRecord{
		RecordID:         coremodule.StableRecordID("relationship", "listens_on", processID, portID),
		RelationshipType: "listens_on", FromID: processID, ToID: portID,
		ObservedAt: observedAt, Confidence: "exact",
		Evidence: []model.Evidence{{Provider: provider.CapabilitySocket, SourceType: "provider", ObservedAt: observedAt, Confidence: "exact"}},
	})
}

func isListening(socket model.Socket) bool {
	if strings.EqualFold(socket.Protocol, "tcp") {
		return strings.EqualFold(socket.State, "LISTEN")
	}
	if !strings.EqualFold(socket.Protocol, "udp") || socket.RemotePort != 0 {
		return false
	}
	remote := strings.TrimSpace(socket.RemoteAddress)
	return remote == "" || remote == "0.0.0.0" || remote == "::"
}

func scope(hostID string) []string {
	if hostID == "" {
		return []string{"host"}
	}
	return []string{hostID}
}

func failedResult(started time.Time, status model.Status, message string) coremodule.Result {
	collectorStatus := model.CollectorStatus{
		Collector: "socket", Status: status, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("port", collectorStatus, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
