package model

import "encoding/json"

const (
	// SnapshotSchemaName 标识完整扫描报告协议。
	SnapshotSchemaName = "asset-agent.snapshot"
	// ModuleReportSchemaName 标识单模块扫描报告协议。
	ModuleReportSchemaName = "asset-agent.module-report"
)

// ModuleData 保存用户所选模块的公开数据；内部依赖字段保持 nil 并从 JSON 中省略。
type ModuleData struct {
	Host              *Host              `json:"host,omitempty"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces,omitempty"`
	Addresses         []Address          `json:"addresses,omitempty"`
	Routes            []Route            `json:"routes,omitempty"`
	Processes         []Process          `json:"processes,omitempty"`
	Sockets           []Socket           `json:"sockets,omitempty"`
	Relationships     []Relationship     `json:"relationships,omitempty"`
}

// ModuleReport 是单个资产扫描模块的统一 JSON 信封。
type ModuleReport struct {
	SchemaName      string            `json:"schema_name"`
	SchemaVersion   string            `json:"schema_version"`
	Module          string            `json:"module"`
	Scan            ScanMetadata      `json:"scan"`
	Agent           AgentInfo         `json:"agent"`
	Data            ModuleData        `json:"data"`
	CollectorStatus []CollectorStatus `json:"collector_status"`
	ResourceUsage   ResourceUsage     `json:"resource_usage"`
}

// MarshalJSON 保证被选模块的集合字段输出 [] 而不是 null。
func (report ModuleReport) MarshalJSON() ([]byte, error) {
	normalized := report
	normalized.CollectorStatus = ensureSlice(normalized.CollectorStatus)
	data := map[string]any{}
	switch normalized.Module {
	case "host":
		data["host"] = normalized.Data.Host
	case "network":
		data["network_interfaces"] = ensureSlice(normalized.Data.NetworkInterfaces)
		data["addresses"] = ensureSlice(normalized.Data.Addresses)
		data["routes"] = ensureSlice(normalized.Data.Routes)
	case "process":
		processes := ensureSlice(normalized.Data.Processes)
		for index := range processes {
			processes[index].CommandLine = ensureSlice(processes[index].CommandLine)
			processes[index].Cgroups = ensureSlice(processes[index].Cgroups)
		}
		data["processes"] = processes
	case "socket":
		sockets := ensureSlice(normalized.Data.Sockets)
		for index := range sockets {
			sockets[index].PIDs = ensureSlice(sockets[index].PIDs)
			sockets[index].ProcessIDs = ensureSlice(sockets[index].ProcessIDs)
		}
		data["sockets"] = sockets
		data["relationships"] = ensureSlice(normalized.Data.Relationships)
	}
	type envelope struct {
		SchemaName      string            `json:"schema_name"`
		SchemaVersion   string            `json:"schema_version"`
		Module          string            `json:"module"`
		Scan            ScanMetadata      `json:"scan"`
		Agent           AgentInfo         `json:"agent"`
		Data            map[string]any    `json:"data"`
		CollectorStatus []CollectorStatus `json:"collector_status"`
		ResourceUsage   ResourceUsage     `json:"resource_usage"`
	}
	return json.Marshal(envelope{
		SchemaName: normalized.SchemaName, SchemaVersion: normalized.SchemaVersion, Module: normalized.Module,
		Scan: normalized.Scan, Agent: normalized.Agent, Data: data,
		CollectorStatus: normalized.CollectorStatus, ResourceUsage: normalized.ResourceUsage,
	})
}
