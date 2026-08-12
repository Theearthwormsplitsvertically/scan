// Package socket collects TCP/UDP facts and maps socket inodes to processes.
package socket

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// Collect parses all supported procfs socket tables, then correlates their inodes to processes.
// It reports /proc/net as the current fallback until a direct SockDiag collector is added.
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

// finish stamps duration and object count on the socket collector status.
func finish(status model.CollectorStatus, started time.Time, objects int) model.CollectorStatus {
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = objects
	return status
}
