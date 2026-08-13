package module

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// StandardCommands 返回每个扫描模块共有的命令空间。
func StandardCommands() []CommandDescriptor {
	return []CommandDescriptor{
		{Name: "scan", Description: "执行该模块扫描", Options: []string{"--output-dir", "-o"}},
		{Name: "describe", Description: "显示模块描述", Options: []string{}},
		{Name: "status", Description: "显示当前平台支持状态", Options: []string{}},
		{Name: "schedule", Description: "显示默认扫描周期", Options: []string{}},
	}
}

// StableRecordID 使用长度分隔的稳定键生成确定性记录 ID。
func StableRecordID(recordType string, parts ...string) string {
	hash := sha256.New()
	for _, part := range append([]string{recordType}, parts...) {
		_, _ = hash.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hash.Write([]byte{':'})
		_, _ = hash.Write([]byte(part))
	}
	return recordType + ":" + hex.EncodeToString(hash.Sum(nil))
}

// NewModuleResult 把现有采集器状态转换为协议 2.0 模块结果。
func NewModuleResult(moduleName string, status model.CollectorStatus, scopes []string, records []model.AssetRecord, relationships []model.RelationshipRecord) model.ModuleResult {
	now := time.Now().UTC()
	if status.StartedAt.IsZero() {
		status.StartedAt = now
	}
	if status.FinishedAt.IsZero() {
		status.FinishedAt = now
	}
	if status.DurationMS == 0 {
		status.DurationMS = status.FinishedAt.Sub(status.StartedAt).Milliseconds()
	}
	resultStatus := moduleStatus(status.Status)
	errors := make([]model.ErrorDetail, 0, len(status.Errors))
	for _, message := range status.Errors {
		errors = append(errors, model.ErrorDetail{Code: "collection_error", Message: message, Provider: status.Collector})
	}
	coverage := model.Coverage{
		ExpectedScopes:  append([]string{}, scopes...),
		CompletedScopes: []string{},
		FailedScopes:    []string{},
	}
	switch resultStatus {
	case model.StatusComplete:
		coverage.CompletedScopes = append(coverage.CompletedScopes, scopes...)
	case model.StatusPartial, model.StatusDegraded:
		if len(records) > 0 {
			coverage.CompletedScopes = append(coverage.CompletedScopes, scopes...)
		}
		coverage.FailedScopes = append(coverage.FailedScopes, scopes...)
	default:
		coverage.FailedScopes = append(coverage.FailedScopes, scopes...)
	}
	return model.ModuleResult{
		Module: moduleName, SchemaVersion: model.BatchSchemaVersion,
		Status: resultStatus, Authoritative: resultStatus == model.StatusComplete,
		StartedAt: status.StartedAt, FinishedAt: status.FinishedAt, DurationMS: status.DurationMS,
		Coverage: coverage, Errors: errors,
		Records: records, Relationships: relationships,
	}
}

func moduleStatus(status model.Status) model.Status {
	switch status {
	case model.StatusOK, model.StatusComplete:
		return model.StatusComplete
	case model.StatusPartial, model.StatusDegraded, model.StatusFailed, model.StatusTimeout, model.StatusUnsupported:
		return status
	default:
		if strings.TrimSpace(string(status)) == "" {
			return model.StatusFailed
		}
		return status
	}
}
