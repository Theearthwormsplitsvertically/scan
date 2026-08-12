// socket 包采集 TCP/UDP 事实，并将 socket inode 映射到进程。
package socket

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// Collect 解析全部支持的 procfs socket 表，再将 inode 关联到进程。
// 在实现直接 SockDiag 采集器前，它将 /proc/net 作为当前降级来源。
func Collect(ctx context.Context, root platform.Root, processes []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "socket", Status: model.StatusOK, StartedAt: started, Errors: []string{}, Fallback: "/proc/net"}
	sockets := make([]model.Socket, 0)
	if err := ctx.Err(); err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		return sockets, []model.Relationship{}, finish(status, started, 0)
	}
	netns, _ := root.Readlink("/proc/self/ns/net")
	files := []struct {
		path, protocol string
		family         int
	}{{"/proc/net/tcp", "tcp", 4}, {"/proc/net/tcp6", "tcp", 6}, {"/proc/net/udp", "udp", 4}, {"/proc/net/udp6", "udp", 6}}
	for _, item := range files {
		data, err := root.ReadFile(item.path, 32<<20)
		if err != nil {
			status.Errors = append(status.Errors, item.path+": "+err.Error())
			continue
		}
		parsed, parseErrors := ParseProcNet(strings.NewReader(string(data)), item.protocol, item.family, netns)
		sockets = append(sockets, parsed...)
		for _, parseErr := range parseErrors {
			status.Errors = append(status.Errors, item.path+": "+parseErr.Error())
		}
	}
	sort.Slice(sockets, func(i, j int) bool { return sockets[i].ID < sockets[j].ID })
	sockets, relationships := Correlate(root, sockets, processes)
	if len(status.Errors) > 0 {
		status.Status = model.StatusPartial
	}
	if len(sockets) == 0 && len(status.Errors) == len(files) {
		status.Status = model.StatusFailed
	}
	return sockets, relationships, finish(status, started, len(sockets))
}

// finish 为 socket 采集状态填充耗时和对象数量。
func finish(status model.CollectorStatus, started time.Time, objects int) model.CollectorStatus {
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = objects
	return status
}
