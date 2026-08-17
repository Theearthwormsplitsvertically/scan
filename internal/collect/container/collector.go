// container 包从容器运行时采集容器与镜像事实。
package container

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// ErrRuntimeUnavailable 表示容器运行时不可用（如 Docker socket 不存在）。
var ErrRuntimeUnavailable = errors.New("container runtime unavailable")

// Source 抽象容器运行时数据源，使采集器与具体运行时解耦、可注入测试假实现。
type Source interface {
	ListContainers(context.Context) ([]model.Container, error)
	ListImages(context.Context) ([]model.ContainerImage, error)
}

// Collect 采集容器与镜像事实。运行时不可用时返回 unsupported，镜像读取失败降级为 partial。
func Collect(ctx context.Context, source Source) ([]model.Container, []model.ContainerImage, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "container", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	if err := ctx.Err(); err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		return nil, nil, finish(status, started, 0)
	}
	containers, err := source.ListContainers(ctx)
	if err != nil {
		if errors.Is(err, ErrRuntimeUnavailable) {
			status.Status = model.StatusUnsupported
		} else {
			status.Status = model.StatusFailed
		}
		status.Errors = append(status.Errors, err.Error())
		return nil, nil, finish(status, started, 0)
	}
	images, err := source.ListImages(ctx)
	if err != nil {
		status.Errors = append(status.Errors, err.Error())
		status.Status = model.StatusPartial
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].ID < containers[j].ID })
	sort.Slice(images, func(i, j int) bool { return images[i].ID < images[j].ID })
	return containers, images, finish(status, started, len(containers)+len(images))
}

func finish(status model.CollectorStatus, started time.Time, objects int) model.CollectorStatus {
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = objects
	return status
}
