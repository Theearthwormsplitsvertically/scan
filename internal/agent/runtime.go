package agent

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Scan(context.Context) (model.Snapshot, error)
}
