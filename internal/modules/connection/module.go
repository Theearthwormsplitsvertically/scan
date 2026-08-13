// Package connection 将 port 模块保留的 socket 事实转换为活动连接资产。
package connection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	portmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/port"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Collector 是活动连接模块的无状态实现。
type Collector struct{}

// New 创建活动连接模块。
func New() Collector { return Collector{} }

// Descriptor 返回连接模块的依赖、周期和资源约束。
func (Collector) Descriptor() coremodule.Descriptor {
	return coremodule.Descriptor{
		Name: "connection", SchemaVersion: model.BatchSchemaVersion,
		RecordTypes: []string{"connection"}, Commands: coremodule.StandardCommands(),
		RequiredCapabilities: []string{provider.CapabilitySocket}, OptionalCapabilities: []string{},
		HardDependencies: []string{"port"}, SoftDependencies: []string{},
		DefaultInterval: "1h", ResourceClass: "light", Timeout: "30s",
	}
}

// Probe 判断执行其 port 依赖所需的 socket 能力是否存在。
func (Collector) Probe(_ context.Context, providers provider.Lookup) coremodule.SupportResult {
	if _, ok := provider.As[provider.SocketProvider](providers, provider.CapabilitySocket); !ok {
		return coremodule.SupportResult{
			Status: model.StatusUnsupported, Reason: "当前平台未提供 socket Provider",
			Errors: []model.ErrorDetail{{Code: "missing_capability", Message: "缺少 socket Provider", Provider: provider.CapabilitySocket}},
		}
	}
	return coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}}
}

// Collect 只消费 port Internal，不再次查询 SocketProvider。
func (Collector) Collect(_ context.Context, providers provider.Lookup, request coremodule.Request) coremodule.Result {
	started := time.Now().UTC()
	portResult, facts, ok := portDependency(request)
	if !ok {
		return failedResult(started, "缺少 port 硬依赖结果")
	}
	observedAt := time.Now().UTC()
	records := make([]model.AssetRecord, 0)
	relationships := make([]model.RelationshipRecord, 0)
	connectionIDs := make(map[string]string)
	seenOwners := make(map[string]bool)
	connections := make([]model.Socket, 0)
	for _, socket := range facts.Sockets {
		if !strings.EqualFold(socket.State, "ESTABLISHED") {
			continue
		}
		connections = append(connections, socket)
		recordID := coremodule.StableRecordID("connection", facts.HostID, socket.NetworkNS, socket.Protocol,
			fmt.Sprintf("%d", socket.Family), socket.LocalAddress, fmt.Sprintf("%d", socket.LocalPort),
			socket.RemoteAddress, fmt.Sprintf("%d", socket.RemotePort), fmt.Sprintf("%d", socket.Inode))
		connectionIDs[socket.ID] = recordID
		records = append(records, model.AssetRecord{
			RecordID: recordID, RecordType: "connection", HostID: facts.HostID,
			ScopeID: facts.HostID, ScopeType: "host",
			Name:     fmt.Sprintf("%s:%d -> %s:%d", socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort),
			Platform: providers.Platform(), States: model.AssetStates{Running: true, Loaded: true},
			FirstObserved: observedAt, LastObserved: observedAt, Confidence: "exact",
			Attributes: map[string]any{
				"socket_id": socket.ID, "protocol": socket.Protocol, "family": socket.Family,
				"state": socket.State, "local_address": socket.LocalAddress, "local_port": socket.LocalPort,
				"remote_address": socket.RemoteAddress, "remote_port": socket.RemotePort,
				"inode": socket.Inode, "network_namespace": socket.NetworkNS,
				"pids": socket.PIDs, "process_ids": socket.ProcessIDs,
			},
			Evidence: []model.Evidence{{Provider: provider.CapabilitySocket, SourceType: "dependency", ObservedAt: observedAt, Confidence: "exact"}},
		})
		for _, rawID := range socket.ProcessIDs {
			appendOwnership(&relationships, seenOwners, facts.ProcessRecordIDs[rawID], recordID, observedAt)
		}
		for _, pid := range socket.PIDs {
			appendOwnership(&relationships, seenOwners, facts.PIDRecordIDs[pid], recordID, observedAt)
		}
	}
	for _, relationship := range facts.SocketProcessRelationships {
		connectionID := connectionIDs[relationship.FromID]
		processID := facts.ProcessRecordIDs[relationship.ToID]
		if connectionID == "" {
			connectionID = connectionIDs[relationship.ToID]
			processID = facts.ProcessRecordIDs[relationship.FromID]
		}
		appendOwnership(&relationships, seenOwners, processID, connectionID, observedAt)
	}
	status := facts.Status
	if status.Collector == "" {
		status.Collector = "socket"
	}
	if status.Status == "" {
		if portResult.Data.Status == model.StatusComplete {
			status.Status = model.StatusOK
		} else {
			status.Status = portResult.Data.Status
		}
	}
	data := coremodule.NewModuleResult("connection", status, scope(facts.HostID), records, relationships)
	return coremodule.Result{Data: data, Internal: connections}
}

func portDependency(request coremodule.Request) (coremodule.Result, portmodule.Facts, bool) {
	dependency, ok := request.Dependencies["port"]
	if !ok {
		return coremodule.Result{}, portmodule.Facts{}, false
	}
	facts, ok := dependency.Internal.(portmodule.Facts)
	return dependency, facts, ok
}

func appendOwnership(relationships *[]model.RelationshipRecord, seen map[string]bool, processID, connectionID string, observedAt time.Time) {
	if processID == "" || connectionID == "" {
		return
	}
	key := processID + "\x00" + connectionID
	if seen[key] {
		return
	}
	seen[key] = true
	*relationships = append(*relationships, model.RelationshipRecord{
		RecordID:         coremodule.StableRecordID("relationship", "connects_to", processID, connectionID),
		RelationshipType: "connects_to", FromID: processID, ToID: connectionID,
		ObservedAt: observedAt, Confidence: "exact",
		Evidence: []model.Evidence{{Provider: provider.CapabilitySocket, SourceType: "dependency", ObservedAt: observedAt, Confidence: "exact"}},
	})
}

func scope(hostID string) []string {
	if hostID == "" {
		return []string{"host"}
	}
	return []string{hostID}
}

func failedResult(started time.Time, message string) coremodule.Result {
	status := model.CollectorStatus{
		Collector: "connection", Status: model.StatusFailed, StartedAt: started,
		FinishedAt: time.Now().UTC(), Errors: []string{message},
	}
	return coremodule.Result{Data: coremodule.NewModuleResult("connection", status, []string{"host"}, []model.AssetRecord{}, []model.RelationshipRecord{})}
}
