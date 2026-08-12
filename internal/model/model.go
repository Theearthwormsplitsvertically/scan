// model 包定义所有采集器共享的稳定本地 JSON 协议。
package model

import (
	"encoding/json"
	"time"
)

// SchemaVersion 标识当前 snapshot 和 doctor JSON 协议版本。
const SchemaVersion = "1.0"

// Status 记录采集器或已检测能力的结果。
type Status string

// 这些采集器和能力状态值区分完整结果与安全降级结果。
const (
	StatusOK          Status = "ok"
	StatusPartial     Status = "partial"
	StatusDegraded    Status = "degraded"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusUnsupported Status = "unsupported"
)

// AgentInfo 描述生成报告的可执行文件。
type AgentInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// ScanMetadata 标识一次扫描并记录其墙钟耗时。
type ScanMetadata struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
}

// Capability 说明某项 Linux 功能是否可用，并在适用时记录其降级路径。
type Capability struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

// CapabilityReport 汇总一次 doctor 或 scan 的所有能力观测结果。
type CapabilityReport struct {
	Items []Capability `json:"items"`
}

// DoctorReport 是 doctor 输出的轻量环境和能力报告。
type DoctorReport struct {
	SchemaVersion string           `json:"schema_version"`
	Agent         AgentInfo        `json:"agent"`
	OS            string           `json:"os"`
	Distribution  string           `json:"distribution,omitempty"`
	Kernel        string           `json:"kernel,omitempty"`
	Architecture  string           `json:"architecture,omitempty"`
	Root          bool             `json:"root"`
	Capabilities  CapabilityReport `json:"capabilities"`
}

// Host 包含主机身份、操作系统、硬件和启动事实。
type Host struct {
	ID             string `json:"id,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	Distribution   string `json:"distribution,omitempty"`
	DistributionID string `json:"distribution_id,omitempty"`
	OSVersion      string `json:"os_version,omitempty"`
	Kernel         string `json:"kernel,omitempty"`
	Architecture   string `json:"architecture,omitempty"`
	MachineID      string `json:"machine_id,omitempty"`
	BootID         string `json:"boot_id,omitempty"`
	DMIUUID        string `json:"dmi_uuid,omitempty"`
	Vendor         string `json:"vendor,omitempty"`
	Model          string `json:"model,omitempty"`
	CPUModel       string `json:"cpu_model,omitempty"`
	CPUCount       int    `json:"cpu_count,omitempty"`
	MemoryBytes    uint64 `json:"memory_bytes,omitempty"`
}

// NetworkInterface 表示一个操作系统网络接口。
type NetworkInterface struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	MTU          int      `json:"mtu"`
	MAC          string   `json:"mac,omitempty"`
	Flags        []string `json:"flags"`
	Namespace    string   `json:"namespace,omitempty"`
	DNSDigestSHA string   `json:"dns_digest_sha256,omitempty"`
}

// Address 将一个 IP CIDR 与网络接口关联。
type Address struct {
	InterfaceIndex int    `json:"interface_index"`
	InterfaceName  string `json:"interface_name"`
	CIDR           string `json:"cidr"`
	Family         int    `json:"family"`
}

// Route 表示一条规范化的 IP 路由。
type Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Family      int    `json:"family"`
}

// Process 表示由 boot ID、PID 和启动时间构成的进程身份。
type Process struct {
	ID          string   `json:"id"`
	PID         int      `json:"pid"`
	PPID        int      `json:"ppid"`
	StartTime   uint64   `json:"start_time"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	UID         int      `json:"uid,omitempty"`
	GID         int      `json:"gid,omitempty"`
	Executable  string   `json:"executable,omitempty"`
	CommandLine []string `json:"command_line"`
	WorkingDir  string   `json:"working_dir,omitempty"`
	RootDir     string   `json:"root_dir,omitempty"`
	Cgroups     []string `json:"cgroups"`
	MountNS     string   `json:"mount_namespace,omitempty"`
	NetworkNS   string   `json:"network_namespace,omitempty"`
	PIDNS       string   `json:"pid_namespace,omitempty"`
}

// Socket 表示从 /proc/net 观测到的一个 TCP 或 UDP socket。
type Socket struct {
	ID            string   `json:"id"`
	Protocol      string   `json:"protocol"`
	Family        int      `json:"family"`
	State         string   `json:"state"`
	LocalAddress  string   `json:"local_address"`
	LocalPort     int      `json:"local_port"`
	RemoteAddress string   `json:"remote_address"`
	RemotePort    int      `json:"remote_port"`
	Inode         uint64   `json:"inode"`
	NetworkNS     string   `json:"network_namespace,omitempty"`
	PIDs          []int    `json:"pids"`
	ProcessIDs    []string `json:"process_ids"`
}

// Service 为后续服务采集器预留 JSON 结构。
type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Package 为后续软件包采集器预留 JSON 结构。
type Package struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Container 为后续容器采集器预留 JSON 结构。
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

// File 为后续可执行文件和动态库采集器预留 JSON 结构。
type File struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}

// Application 为后续应用深度采集器预留 JSON 结构。
type Application struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Relationship 记录两个采集对象之间有证据支撑的有向关联。
type Relationship struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	FromID     string    `json:"from_id"`
	ToID       string    `json:"to_id"`
	Source     string    `json:"source"`
	Collector  string    `json:"collector"`
	Confidence string    `json:"confidence"`
	ObservedAt time.Time `json:"observed_at"`
}

// CollectorStatus 描述一个采集器的结果，不影响其他采集域。
type CollectorStatus struct {
	Collector  string    `json:"collector"`
	Status     Status    `json:"status"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Objects    int       `json:"objects,omitempty"`
	Errors     []string  `json:"errors"`
	Fallback   string    `json:"fallback,omitempty"`
}

// ResourceUsage 记录 Agent 在一次扫描中的自身资源指标。
type ResourceUsage struct {
	WallTimeMS     int64  `json:"wall_time_ms"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapDeltaBytes int64  `json:"heap_delta_bytes"`
}

// Snapshot 是 Agent 写出的完整一次性扫描文档。
type Snapshot struct {
	SchemaVersion     string             `json:"schema_version"`
	Scan              ScanMetadata       `json:"scan"`
	Agent             AgentInfo          `json:"agent"`
	Capabilities      CapabilityReport   `json:"capabilities"`
	Host              Host               `json:"host"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces"`
	Addresses         []Address          `json:"addresses"`
	Routes            []Route            `json:"routes"`
	Processes         []Process          `json:"processes"`
	Sockets           []Socket           `json:"sockets"`
	Services          []Service          `json:"services"`
	Packages          []Package          `json:"packages"`
	Containers        []Container        `json:"containers"`
	Files             []File             `json:"files"`
	Applications      []Application      `json:"applications"`
	Relationships     []Relationship     `json:"relationships"`
	CollectorStatus   []CollectorStatus  `json:"collector_status"`
	ResourceUsage     ResourceUsage      `json:"resource_usage"`
}

// MarshalJSON 将每个集合规范化为 []，避免下游消费者收到 null。
func (snapshot Snapshot) MarshalJSON() ([]byte, error) {
	normalized := snapshot
	normalized.Capabilities.Items = ensureSlice(normalized.Capabilities.Items)
	normalized.NetworkInterfaces = ensureSlice(normalized.NetworkInterfaces)
	normalized.Addresses = ensureSlice(normalized.Addresses)
	normalized.Routes = ensureSlice(normalized.Routes)
	normalized.Processes = ensureSlice(normalized.Processes)
	normalized.Sockets = ensureSlice(normalized.Sockets)
	normalized.Services = ensureSlice(normalized.Services)
	normalized.Packages = ensureSlice(normalized.Packages)
	normalized.Containers = ensureSlice(normalized.Containers)
	normalized.Files = ensureSlice(normalized.Files)
	normalized.Applications = ensureSlice(normalized.Applications)
	normalized.Relationships = ensureSlice(normalized.Relationships)
	normalized.CollectorStatus = ensureSlice(normalized.CollectorStatus)
	for index := range normalized.NetworkInterfaces {
		normalized.NetworkInterfaces[index].Flags = ensureSlice(normalized.NetworkInterfaces[index].Flags)
	}
	for index := range normalized.Processes {
		normalized.Processes[index].CommandLine = ensureSlice(normalized.Processes[index].CommandLine)
		normalized.Processes[index].Cgroups = ensureSlice(normalized.Processes[index].Cgroups)
	}
	for index := range normalized.Sockets {
		normalized.Sockets[index].PIDs = ensureSlice(normalized.Sockets[index].PIDs)
		normalized.Sockets[index].ProcessIDs = ensureSlice(normalized.Sockets[index].ProcessIDs)
	}
	for index := range normalized.CollectorStatus {
		normalized.CollectorStatus[index].Errors = ensureSlice(normalized.CollectorStatus[index].Errors)
	}
	type snapshotAlias Snapshot
	return json.Marshal(snapshotAlias(normalized))
}

// ensureSlice 将 nil 集合转换为空集合，同时保留非 nil 切片。
func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
