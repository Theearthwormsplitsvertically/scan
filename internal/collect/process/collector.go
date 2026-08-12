package process

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
	"github.com/Theearthwormsplitsvertically/scan/internal/security"
)

// processFileLimit 限制每次 procfs 进程读取的大小，包括命令行。
const processFileLimit = 1 << 20

// Collect 枚举数值形式的 /proc 条目并构建稳定进程身份。
// 读取期间退出的 PID 重试一次；仍存在的竞态被标为 partial 状态。
func Collect(ctx context.Context, root platform.Root, bootID string) ([]model.Process, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "process", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	processes := make([]model.Process, 0)
	entries, err := root.ReadDir("/proc")
	if err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		return processes, finishStatus(status, started, 0)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			status.Status = model.StatusPartial
			status.Errors = append(status.Errors, err.Error())
			break
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		collected, err := collectPID(root, pid, bootID)
		if err != nil {
			collected, err = collectPID(root, pid, bootID)
		}
		if err != nil {
			status.Errors = append(status.Errors, fmt.Sprintf("pid %d: %v", pid, err))
			continue
		}
		processes = append(processes, collected)
	}
	sort.Slice(processes, func(i, j int) bool { return processes[i].PID < processes[j].PID })
	if len(status.Errors) > 0 && status.Status == model.StatusOK {
		status.Status = model.StatusPartial
	}
	return processes, finishStatus(status, started, len(processes))
}

// collectPID 读取一个 PID 的允许事实，并明确不读取 environ。
func collectPID(root platform.Root, pid int, bootID string) (model.Process, error) {
	prefix := fmt.Sprintf("/proc/%d", pid)
	statData, err := root.ReadFile(prefix+"/stat", processFileLimit)
	if err != nil {
		return model.Process{}, err
	}
	stat, err := ParseStat(statData)
	if err != nil {
		return model.Process{}, err
	}
	result := model.Process{ID: fmt.Sprintf("%s:%d:%d", bootID, pid, stat.StartTime), PID: pid, PPID: stat.PPID, StartTime: stat.StartTime, Name: stat.Command, State: stat.State, UID: -1, GID: -1, CommandLine: []string{}, Cgroups: []string{}}
	if data, err := root.ReadFile(prefix+"/status", processFileLimit); err == nil {
		result.UID, result.GID = ParseIDs(data)
	}
	if data, err := root.ReadFile(prefix+"/cmdline", processFileLimit); err == nil {
		result.CommandLine = security.RedactArgs(ParseCmdline(data))
	}
	if data, err := root.ReadFile(prefix+"/cgroup", processFileLimit); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line != "" {
				result.Cgroups = append(result.Cgroups, line)
			}
		}
	}
	result.Executable, _ = root.Readlink(prefix + "/exe")
	result.WorkingDir, _ = root.Readlink(prefix + "/cwd")
	result.RootDir, _ = root.Readlink(prefix + "/root")
	result.MountNS, _ = root.Readlink(prefix + "/ns/mnt")
	result.NetworkNS, _ = root.Readlink(prefix + "/ns/net")
	result.PIDNS, _ = root.Readlink(prefix + "/ns/pid")
	return result, nil
}

// finishStatus 在一次进程采集后填充耗时和对象数量。
func finishStatus(status model.CollectorStatus, started time.Time, objects int) model.CollectorStatus {
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = objects
	return status
}
