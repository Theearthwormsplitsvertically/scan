package model

import "time"

const SchemaVersion = "1.0"

type Status string

const (
	StatusOK          Status = "ok"
	StatusPartial     Status = "partial"
	StatusDegraded    Status = "degraded"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusUnsupported Status = "unsupported"
)

type AgentInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

type ScanMetadata struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
}

type Capability struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

type CapabilityReport struct {
	Items []Capability `json:"items"`
}

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

type NetworkInterface struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	MTU          int      `json:"mtu"`
	MAC          string   `json:"mac,omitempty"`
	Flags        []string `json:"flags"`
	Namespace    string   `json:"namespace,omitempty"`
	DNSDigestSHA string   `json:"dns_digest_sha256,omitempty"`
}

type Address struct {
	InterfaceIndex int    `json:"interface_index"`
	InterfaceName  string `json:"interface_name"`
	CIDR           string `json:"cidr"`
	Family         int    `json:"family"`
}

type Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Family      int    `json:"family"`
}

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

type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Package struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

type File struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}

type Application struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

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

type ResourceUsage struct {
	WallTimeMS     int64  `json:"wall_time_ms"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapDeltaBytes int64  `json:"heap_delta_bytes"`
}

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
