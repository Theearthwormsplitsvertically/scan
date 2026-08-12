// process 包从 procfs 采集只读进程事实和进程身份。
package process

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Stat 是从 /proc/<pid>/stat 解析出的、携带身份信息的字段子集。
type Stat struct {
	PID       int
	Command   string
	State     string
	PPID      int
	StartTime uint64
}

// ParseStat 解析 procfs stat 行，即使命令名包含空格或括号也能正确处理。
func ParseStat(data []byte) (Stat, error) {
	line := strings.TrimSpace(string(data))
	open := strings.IndexByte(line, '(')
	close := strings.LastIndex(line, ") ")
	if open <= 0 || close <= open {
		return Stat{}, fmt.Errorf("malformed stat command")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line[:open]))
	if err != nil {
		return Stat{}, fmt.Errorf("parse PID: %w", err)
	}
	fields := strings.Fields(line[close+2:])
	if len(fields) <= 19 {
		return Stat{}, fmt.Errorf("stat has %d fields after command, need 20", len(fields))
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Stat{}, fmt.Errorf("parse PPID: %w", err)
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return Stat{}, fmt.Errorf("parse starttime: %w", err)
	}
	return Stat{PID: pid, Command: line[open+1 : close], State: fields[0], PPID: ppid, StartTime: startTime}, nil
}

// ParseCmdline 分割 /proc/<pid>/cmdline 的 NUL 分隔表示。
func ParseCmdline(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, string(part))
		}
	}
	return result
}

// ParseIDs 返回 /proc/<pid>/status 中记录的真实 UID 和 GID。
func ParseIDs(data []byte) (uid, gid int) {
	uid, gid = -1, -1
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		parsed, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch key {
		case "Uid":
			uid = parsed
		case "Gid":
			gid = parsed
		}
	}
	return uid, gid
}
