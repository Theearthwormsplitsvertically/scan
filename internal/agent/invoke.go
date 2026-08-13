package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

const defaultModuleTimeout = 5 * time.Minute

func invokeProbe(ctx context.Context, item coremodule.Module, providers provider.Lookup) (result coremodule.SupportResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = coremodule.SupportResult{
				Status: model.StatusUnsupported, Reason: "模块能力探测发生 panic",
				Errors: []model.ErrorDetail{{Code: "probe_panic", Message: fmt.Sprintf("模块能力探测 panic: %v", recovered)}},
			}
		}
		result.Errors = scannerNonNil(result.Errors)
		if result.Status == "" {
			result.Status = model.StatusUnsupported
		}
	}()
	return item.Probe(ctx, providers)
}

func invokeModule(parent context.Context, item coremodule.Module, providers provider.Lookup, request coremodule.Request) (result coremodule.Result) {
	descriptor := item.Descriptor()
	timeout := defaultModuleTimeout
	if descriptor.Timeout != "" {
		parsed, err := time.ParseDuration(descriptor.Timeout)
		if err != nil || parsed <= 0 {
			return failureResult(descriptor.Name, model.StatusFailed, "invalid_timeout", "模块 timeout 配置无效")
		}
		timeout = parsed
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			result = failureResult(descriptor.Name, model.StatusFailed, "module_panic", fmt.Sprintf("模块执行 panic: %v", recovered))
		}
	}()
	result = item.Collect(ctx, providers, request)
	if ctx.Err() == context.DeadlineExceeded {
		return failureResult(descriptor.Name, model.StatusTimeout, "module_timeout", "模块执行超过 "+timeout.String())
	}
	if ctx.Err() == context.Canceled && parent.Err() != nil {
		return failureResult(descriptor.Name, model.StatusFailed, "module_canceled", parent.Err().Error())
	}
	return result
}

func failureResult(moduleName string, status model.Status, code, message string) coremodule.Result {
	now := time.Now().UTC()
	return coremodule.Result{Data: model.ModuleResult{
		Module: moduleName, SchemaVersion: model.BatchSchemaVersion, Status: status, Authoritative: false,
		StartedAt: now, FinishedAt: now, Coverage: model.Coverage{
			ExpectedScopes: []string{"host"}, CompletedScopes: []string{}, FailedScopes: []string{"host"},
		},
		Errors:  []model.ErrorDetail{{Code: code, Message: message}},
		Records: []model.AssetRecord{}, Relationships: []model.RelationshipRecord{},
	}}
}

func normalizeCollectedResult(result coremodule.Result, moduleName string) coremodule.Result {
	now := time.Now().UTC()
	if result.Data.Module == "" {
		result.Data.Module = moduleName
	}
	if result.Data.SchemaVersion == "" {
		result.Data.SchemaVersion = model.BatchSchemaVersion
	}
	if result.Data.Status == "" {
		result.Data.Status = model.StatusFailed
		result.Data.Errors = append(result.Data.Errors, model.ErrorDetail{Code: "empty_status", Message: "模块未返回状态"})
	}
	if result.Data.StartedAt.IsZero() {
		result.Data.StartedAt = now
	}
	if result.Data.FinishedAt.IsZero() {
		result.Data.FinishedAt = now
	}
	if result.Data.DurationMS == 0 {
		result.Data.DurationMS = result.Data.FinishedAt.Sub(result.Data.StartedAt).Milliseconds()
	}
	result.Data.Coverage.ExpectedScopes = scannerNonNil(result.Data.Coverage.ExpectedScopes)
	result.Data.Coverage.CompletedScopes = scannerNonNil(result.Data.Coverage.CompletedScopes)
	result.Data.Coverage.FailedScopes = scannerNonNil(result.Data.Coverage.FailedScopes)
	result.Data.Errors = scannerNonNil(result.Data.Errors)
	result.Data.Records = scannerNonNil(result.Data.Records)
	result.Data.Relationships = scannerNonNil(result.Data.Relationships)
	if result.Data.Status != model.StatusComplete {
		result.Data.Authoritative = false
	}
	return result
}
