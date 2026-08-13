package model

import (
	"encoding/json"
	"time"
)

const (
	// BatchSchemaName 标识包含模块结果的内存扫描批次。
	BatchSchemaName = "asset-agent.batch"
	// BatchManifestSchemaName 标识 CMDB 用于校验正式批次的清单。
	BatchManifestSchemaName = "asset-agent.batch-manifest"
	// BatchSchemaVersion 是新分片批次协议的版本。
	BatchSchemaVersion = "2.0"

	BatchTypeSnapshot = "snapshot"
	BatchTypeModule   = "module"
	BatchTypeDelta    = "delta"
	BatchTypeMetrics  = "metrics"
)

// AssetStates 分别描述资产是否已安装、正在运行、已加载和已暴露。
type AssetStates struct {
	Installed bool `json:"installed"`
	Running   bool `json:"running"`
	Loaded    bool `json:"loaded"`
	Exposed   bool `json:"exposed"`
}

// Evidence 记录一个资产结论或关系的可追溯来源。
type Evidence struct {
	Provider    string    `json:"provider"`
	SourceType  string    `json:"source_type"`
	Locator     string    `json:"locator,omitempty"`
	LocatorHash string    `json:"locator_hash,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
	Digest      string    `json:"digest,omitempty"`
	Confidence  string    `json:"confidence"`
}

// AssetRecord 是所有平台和扫描模块共享的资产记录。
type AssetRecord struct {
	RecordID      string         `json:"record_id"`
	RecordType    string         `json:"record_type"`
	HostID        string         `json:"host_id,omitempty"`
	ScopeID       string         `json:"scope_id"`
	ScopeType     string         `json:"scope_type"`
	Name          string         `json:"name"`
	Version       string         `json:"version,omitempty"`
	Vendor        string         `json:"vendor,omitempty"`
	Platform      string         `json:"platform"`
	States        AssetStates    `json:"states"`
	FirstObserved time.Time      `json:"first_observed_at,omitempty"`
	LastObserved  time.Time      `json:"last_observed_at,omitempty"`
	Confidence    string         `json:"confidence,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
	Evidence      []Evidence     `json:"evidence"`
}

// MarshalJSON 保证证据集合始终编码为数组。
func (record AssetRecord) MarshalJSON() ([]byte, error) {
	normalized := record
	normalized.Evidence = ensureSlice(normalized.Evidence)
	type alias AssetRecord
	return json.Marshal(alias(normalized))
}

// RelationshipRecord 表示两个资产记录之间有证据支撑的关系。
type RelationshipRecord struct {
	RecordID         string     `json:"record_id"`
	RelationshipType string     `json:"relationship_type"`
	FromID           string     `json:"from_id"`
	ToID             string     `json:"to_id"`
	ObservedAt       time.Time  `json:"observed_at"`
	Confidence       string     `json:"confidence"`
	Evidence         []Evidence `json:"evidence"`
}

// MarshalJSON 保证关系证据集合始终编码为数组。
func (relationship RelationshipRecord) MarshalJSON() ([]byte, error) {
	normalized := relationship
	normalized.Evidence = ensureSlice(normalized.Evidence)
	type alias RelationshipRecord
	return json.Marshal(alias(normalized))
}

// ErrorDetail 是机器可判断、中文可解释的模块错误。
type ErrorDetail struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Provider string `json:"provider,omitempty"`
	ScopeID  string `json:"scope_id,omitempty"`
}

// Coverage 描述一次模块扫描预期、完成和失败的作用域。
type Coverage struct {
	ExpectedScopes  []string `json:"expected_scopes"`
	CompletedScopes []string `json:"completed_scopes"`
	FailedScopes    []string `json:"failed_scopes"`
}

// ModuleResult 保存一个模块的独立状态、资产和关系。
type ModuleResult struct {
	Module        string               `json:"module"`
	SchemaVersion string               `json:"schema_version"`
	Status        Status               `json:"status"`
	Authoritative bool                 `json:"authoritative"`
	StartedAt     time.Time            `json:"started_at"`
	FinishedAt    time.Time            `json:"finished_at"`
	DurationMS    int64                `json:"duration_ms"`
	Coverage      Coverage             `json:"coverage"`
	Errors        []ErrorDetail        `json:"errors"`
	Records       []AssetRecord        `json:"records"`
	Relationships []RelationshipRecord `json:"relationships"`
	Published     bool                 `json:"-"`
}

// MarshalJSON 保证空的模块结果仍使用数组表达覆盖、错误和数据。
func (result ModuleResult) MarshalJSON() ([]byte, error) {
	normalized := result
	normalized.Coverage.ExpectedScopes = ensureSlice(normalized.Coverage.ExpectedScopes)
	normalized.Coverage.CompletedScopes = ensureSlice(normalized.Coverage.CompletedScopes)
	normalized.Coverage.FailedScopes = ensureSlice(normalized.Coverage.FailedScopes)
	normalized.Errors = ensureSlice(normalized.Errors)
	normalized.Records = ensureSlice(normalized.Records)
	normalized.Relationships = ensureSlice(normalized.Relationships)
	type alias ModuleResult
	return json.Marshal(alias(normalized))
}

// Batch 是编排器与报告层之间的平台无关扫描批次。
type Batch struct {
	SchemaName      string         `json:"schema_name"`
	SchemaVersion   string         `json:"schema_version"`
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	RequestedModule string         `json:"requested_module"`
	Platform        string         `json:"platform"`
	Agent           AgentInfo      `json:"agent"`
	StartedAt       time.Time      `json:"started_at"`
	FinishedAt      time.Time      `json:"finished_at"`
	Results         []ModuleResult `json:"results"`
}

// MarshalJSON 保证没有模块结果时仍编码为 []。
func (batch Batch) MarshalJSON() ([]byte, error) {
	normalized := batch
	normalized.Results = ensureSlice(normalized.Results)
	type alias Batch
	return json.Marshal(alias(normalized))
}

// BatchFile 描述一个已同步 JSONL 分片。
type BatchFile struct {
	Name       string `json:"name"`
	Module     string `json:"module,omitempty"`
	RecordType string `json:"record_type"`
	Records    int    `json:"records"`
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256"`
}

// ModuleManifest 保存一个已执行模块的完整性结论。
type ModuleManifest struct {
	Module        string        `json:"module"`
	SchemaVersion string        `json:"schema_version"`
	Status        Status        `json:"status"`
	Authoritative bool          `json:"authoritative"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	DurationMS    int64         `json:"duration_ms"`
	Coverage      Coverage      `json:"coverage"`
	Errors        []ErrorDetail `json:"errors"`
}

// BatchManifest 是在批次发布前最后写入的完整性清单。
type BatchManifest struct {
	SchemaName      string           `json:"schema_name"`
	SchemaVersion   string           `json:"schema_version"`
	ScanID          string           `json:"scan_id"`
	BatchType       string           `json:"batch_type"`
	RequestedModule string           `json:"requested_module"`
	Platform        string           `json:"platform"`
	Agent           AgentInfo        `json:"agent"`
	StartedAt       time.Time        `json:"started_at"`
	FinishedAt      time.Time        `json:"finished_at"`
	Modules         []ModuleManifest `json:"modules"`
	Files           []BatchFile      `json:"files"`
}
