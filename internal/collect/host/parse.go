// host 包采集主机身份、操作系统、CPU 和内存事实。
package host

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// ParseMemoryBytes 将 /proc/meminfo 中的 MemTotal 转换为字节数。
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
