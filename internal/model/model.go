// Package model defines the stable local JSON protocol shared by all collectors.
package model

import (
	"encoding/json"
	"time"
)

// SchemaVersion identifies the current snapshot and doctor JSON contract.
const SchemaVersion = "1.0"

// Status records the outcome of a collector or a detected capability.
type Status string

// Collector and capability status values distinguish complete results from safe degradation.
const (
	StatusOK          Status = "ok"
	StatusPartial     Status = "partial"
	StatusDegraded    Status = "degraded"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusUnsupported Status = "unsupported"
)

// AgentInfo describes the executable that produced a report.
type AgentInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// ScanMetadata identifies one scan and records its wall-clock duration.
type ScanMetadata struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
}

// Capability states whether a Linux feature is usable and names its fallback when applicable.
type Capability struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

// CapabilityReport groups all capability observations for a doctor or scan result.
type CapabilityReport struct {
	Items []Capability `json:"items"`
}

// DoctorReport is the lightweight environment and capability report from doctor.
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

// Host contains host identity, operating system, hardware, and boot facts.
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

// NetworkInterface represents one operating-system network interface.
type NetworkInterface struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	MTU          int      `json:"mtu"`
	MAC          string   `json:"mac,omitempty"`
	Flags        []string `json:"flags"`
	Namespace    string   `json:"namespace,omitempty"`
	DNSDigestSHA string   `json:"dns_digest_sha256,omitempty"`
}

// Address associates one IP CIDR with a network interface.
type Address struct {
	InterfaceIndex int    `json:"interface_index"`
	InterfaceName  string `json:"interface_name"`
	CIDR           string `json:"cidr"`
	Family         int    `json:"family"`
}

// Route represents one normalized IP route.
type Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Family      int    `json:"family"`
}

// Process represents a process identity built from boot ID, PID, and start time.
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

// Socket represents one TCP or UDP socket observed from /proc/net.
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

// Service reserves the JSON shape for a future service collector.
type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Package reserves the JSON shape for a future package collector.
type Package struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Container reserves the JSON shape for a future container collector.
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Runtime string `json:"runtime,omitempty"`
}

// File reserves the JSON shape for a future executable and library collector.
type File struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}

// Application reserves the JSON shape for a future deep application collector.
type Application struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Relationship records a directed, evidence-backed link between two collected objects.
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

// CollectorStatus describes one collector's outcome without discarding other domains.
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

// ResourceUsage records the Agent's own resource measurements for one scan.
type ResourceUsage struct {
	WallTimeMS     int64  `json:"wall_time_ms"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapDeltaBytes int64  `json:"heap_delta_bytes"`
}

// Snapshot is the complete one-shot scan document written by the Agent.
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

// MarshalJSON normalizes every collection to [] so downstream consumers never receive null.
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

// ensureSlice converts a nil collection to an empty collection while preserving non-nil slices.
func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
