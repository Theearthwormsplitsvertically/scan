// agent 包负责运行时边界和动态模块扫描编排。
package agent

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

// ScanSelection 表示一次全量扫描或显式模块集合；两种模式不能混用。
type ScanSelection struct {
	All     bool
	Modules []string
}

// ScanOutcome 同时返回可发布批次和仅供终端摘要使用的真实采集记录数。
type ScanOutcome struct {
	Batch        model.Batch
	RecordCounts map[string]int
}

// Runtime 是 CLI 使用的动态扫描器边界。
type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Modules(context.Context) ([]coremodule.Info, error)
	Scan(context.Context, ScanSelection) (ScanOutcome, error)
	ScanTarget(context.Context, string) (model.Batch, error)
}
