package agent

import (
	"context"
	"errors"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// LocalRuntime binds the Agent to one read-only Linux filesystem root and its collectors.
// Production uses /; tests may use a synthetic root with the same Linux layout.
type LocalRuntime struct {
	root         platform.Root
	dependencies Dependencies
}

// NewLocalRuntime creates the production dependency graph for a filesystem root.
func NewLocalRuntime(root platform.Root) *LocalRuntime {
	return NewLocalRuntimeWithDependencies(root, defaultDependencies(root))
}

// NewLocalRuntimeWithDependencies creates a runtime with injectable collectors for tests.
func NewLocalRuntimeWithDependencies(root platform.Root, dependencies Dependencies) *LocalRuntime {
	return &LocalRuntime{root: root, dependencies: dependencies}
}

// Doctor runs only capability detection and never performs the full asset scan.
func (local *LocalRuntime) Doctor(ctx context.Context) (model.DoctorReport, error) {
	if err := ctx.Err(); err != nil {
		return model.DoctorReport{}, err
	}
	if local.dependencies.Doctor == nil {
		return model.DoctorReport{}, errors.New("doctor dependency unavailable")
	}
	return local.dependencies.Doctor(ctx), nil
}

// UnavailableRuntime returns a clear error when a non-Linux build receives collection commands.
type UnavailableRuntime struct{}

// Doctor reports that the current executable cannot collect non-Linux host facts.
func (UnavailableRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return model.DoctorReport{}, errors.New("asset collection requires Linux")
}

// Scan reports that the current executable cannot collect non-Linux host facts.
func (UnavailableRuntime) Scan(context.Context) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("asset collection requires Linux")
}
