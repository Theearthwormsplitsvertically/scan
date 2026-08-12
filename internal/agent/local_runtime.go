package agent

import (
	"context"
	"errors"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// LocalRuntime 将 Agent 绑定到一个只读 Linux 文件系统根和一组采集器。
// 生产环境使用 /；测试可使用相同 Linux 目录布局的模拟根。
type LocalRuntime struct {
	root         platform.Root
	dependencies Dependencies
}

// NewLocalRuntime 为给定文件系统根创建生产环境的采集依赖图。
func NewLocalRuntime(root platform.Root) *LocalRuntime {
	return NewLocalRuntimeWithDependencies(root, defaultDependencies(root))
}

// NewLocalRuntimeWithDependencies 使用可注入采集器创建运行时，供测试隔离编排逻辑。
func NewLocalRuntimeWithDependencies(root platform.Root, dependencies Dependencies) *LocalRuntime {
	return &LocalRuntime{root: root, dependencies: dependencies}
}

// Doctor 只执行能力检测，不执行完整资产扫描。
func (local *LocalRuntime) Doctor(ctx context.Context) (model.DoctorReport, error) {
	if err := ctx.Err(); err != nil {
		return model.DoctorReport{}, err
	}
	if local.dependencies.Doctor == nil {
		return model.DoctorReport{}, errors.New("doctor dependency unavailable")
	}
	return local.dependencies.Doctor(ctx), nil
}

// UnavailableRuntime 在非 Linux 构建收到采集命令时返回明确错误。
type UnavailableRuntime struct{}

// Doctor 说明当前可执行文件不能采集非 Linux 主机事实。
func (UnavailableRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return model.DoctorReport{}, errors.New("asset collection requires Linux")
}

// Scan 说明当前可执行文件不能采集非 Linux 主机事实。
func (UnavailableRuntime) Scan(context.Context) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("asset collection requires Linux")
}
