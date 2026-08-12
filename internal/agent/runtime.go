// agent 包负责运行时边界和一次性采集编排。
package agent

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// Runtime 是 CLI 所依赖的最小接口，提供环境诊断和一次完整扫描。
type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Scan(context.Context) (model.Snapshot, error)
}
