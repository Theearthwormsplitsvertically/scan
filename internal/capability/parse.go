// capability 包检测并解析 Linux 资产采集能力。
package capability

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// SelfStatus 是权限检测所需的 /proc/self/status 字段子集。
type SelfStatus struct {
	UIDs   [4]uint32
	GIDs   [4]uint32
	CapEff uint64
	CapBnd uint64
}

// ParseOSRelease 解析 /etc/os-release 的键值数据，并跳过格式错误的行。
func ParseOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values
}

// ParseSelfStatus 提取 UID/GID 集合和 capability 掩码，缺少可选字段时不失败。
func ParseSelfStatus(data []byte) SelfStatus {
	var result SelfStatus
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Uid":
			parseFourUint32(strings.Fields(value), &result.UIDs)
		case "Gid":
			parseFourUint32(strings.Fields(value), &result.GIDs)
		case "CapEff":
			result.CapEff, _ = strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		case "CapBnd":
			result.CapBnd, _ = strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		}
	}
	return result
}

// parseFourUint32 原子解析 Linux 的四值 UID 或 GID 字段。
func parseFourUint32(fields []string, destination *[4]uint32) {
	if len(fields) < len(destination) {
		return
	}
	var parsed [4]uint32
	for index := range parsed {
		value, err := strconv.ParseUint(fields[index], 10, 32)
		if err != nil {
			return
		}
		parsed[index] = uint32(value)
	}
	*destination = parsed
}
