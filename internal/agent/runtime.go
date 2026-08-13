// agent 包负责运行时边界和动态模块扫描编排。
package agent

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

// Runtime 是 CLI 使用的动态扫描器边界。
type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Modules(context.Context) ([]coremodule.Info, error)
	ScanTarget(context.Context, string) (model.Batch, error)
}
