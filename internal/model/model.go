// model 包定义所有采集器共享的稳定本地 JSON 协议。
package model

import "time"

// DoctorSchemaVersion 标识 doctor 环境诊断 JSON 协议版本。
const DoctorSchemaVersion = "1.0"

// Status 记录采集器或已检测能力的结果。
type Status string

// 这些采集器和能力状态值区分完整结果与安全降级结果。
const (
	StatusComplete    Status = "complete"
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

// SystemProfile 是每次扫描前只读识别的 Linux 系统和数据源画像。
type SystemProfile struct {
	OS                  string          `json:"os"`
	DistributionID      string          `json:"distribution_id,omitempty"`
	DistributionVersion string          `json:"distribution_version,omitempty"`
	DistributionName    string          `json:"distribution_name,omitempty"`
	Kernel              string          `json:"kernel,omitempty"`
	Architecture        string          `json:"architecture,omitempty"`
	InitSystem          string          `json:"init_system,omitempty"`
	CgroupVersion       string          `json:"cgroup_version,omitempty"`
	SecurityModules     []string        `json:"security_modules"`
	ContainerRuntimes   []string        `json:"container_runtimes"`
	AvailableSources    map[string]bool `json:"available_sources"`
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
	SystemProfile SystemProfile    `json:"system_profile"`
}

// Host 包含主机身份、操作系统、硬件和启动事实。
type Host struct {
	Hostname            string `json:"hostname,omitempty"`
	DistributionName    string `json:"distribution_name,omitempty"`
	DistributionID      string `json:"distribution_id,omitempty"`
	DistributionVersion string `json:"distribution_version,omitempty"`
	KernelRelease       string `json:"kernel_release,omitempty"`
	Architecture        string `json:"architecture,omitempty"`
	BootID              string `json:"boot_id,omitempty"`
	DMIUUID             string `json:"dmi_uuid,omitempty"`
	MemoryTotalBytes    uint64 `json:"memory_total_bytes,omitempty"`
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
	Backend    string    `json:"backend,omitempty"`
}

// ensureSlice 将 nil 集合转换为空集合，同时保留非 nil 切片。
func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
