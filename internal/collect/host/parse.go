// Package host collects host identity, operating-system, CPU, and memory facts.
package host

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// ParseMemoryBytes converts MemTotal from /proc/meminfo into bytes.
func ParseMemoryBytes(data []byte) uint64 {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		if len(fields) >= 3 && strings.EqualFold(fields[2], "kB") {
			return value * 1024
		}
		return value
	}
	return 0
}

// ParseCPUModel returns the first useful x86 or ARM model description from /proc/cpuinfo.
func ParseCPUModel(data []byte) string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if _, exists := values[key]; !exists {
			values[key] = strings.TrimSpace(value)
		}
	}
	if values["Hardware"] != "" {
		return values["Hardware"]
	}
	if values["model name"] != "" {
		return values["model name"]
	}
	return values["Processor"]
}
