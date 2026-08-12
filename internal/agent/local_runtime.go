package agent

import (
	"context"
	"errors"
	"runtime"

	"github.com/Theearthwormsplitsvertically/scan/internal/capability"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

type LocalRuntime struct {
	root platform.Root
}

func NewLocalRuntime(root platform.Root) *LocalRuntime {
	return &LocalRuntime{root: root}
}

func (local *LocalRuntime) Doctor(ctx context.Context) (model.DoctorReport, error) {
	if err := ctx.Err(); err != nil {
		return model.DoctorReport{}, err
	}
	return capability.Detect(ctx, local.root, runtime.GOARCH), nil
}

func (local *LocalRuntime) Scan(context.Context) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("scan runtime not implemented")
}

type UnavailableRuntime struct{}

func (UnavailableRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return model.DoctorReport{}, errors.New("asset collection requires Linux")
}

func (UnavailableRuntime) Scan(context.Context) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("asset collection requires Linux")
}
