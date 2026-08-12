package agent

import (
	"context"
	"errors"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

type LocalRuntime struct {
	root         platform.Root
	dependencies Dependencies
}

func NewLocalRuntime(root platform.Root) *LocalRuntime {
	return NewLocalRuntimeWithDependencies(root, defaultDependencies(root))
}

func NewLocalRuntimeWithDependencies(root platform.Root, dependencies Dependencies) *LocalRuntime {
	return &LocalRuntime{root: root, dependencies: dependencies}
}

func (local *LocalRuntime) Doctor(ctx context.Context) (model.DoctorReport, error) {
	if err := ctx.Err(); err != nil {
		return model.DoctorReport{}, err
	}
	if local.dependencies.Doctor == nil {
		return model.DoctorReport{}, errors.New("doctor dependency unavailable")
	}
	return local.dependencies.Doctor(ctx), nil
}

type UnavailableRuntime struct{}

func (UnavailableRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return model.DoctorReport{}, errors.New("asset collection requires Linux")
}

func (UnavailableRuntime) Scan(context.Context) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("asset collection requires Linux")
}
