// Package agent owns runtime boundaries and one-shot collection orchestration.
package agent

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// Runtime is the CLI-facing contract for diagnostics and one complete scan.
type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Scan(context.Context) (model.Snapshot, error)
}
