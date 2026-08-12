// network 包采集网络接口、地址、DNS 摘要和 IP 路由。
package network

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

// ParseIPv4Routes 解析 /proc/net/route，并转换 Linux 小端序 IPv4 字段。
// 格式错误的行以错误返回，格式正确的路由仍会保留。
func ParseIPv4Routes(reader io.Reader) ([]model.Route, []error) {
	routes := make([]model.Route, 0)
	errorsFound := make([]error, 0)
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber == 1 {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			errorsFound = append(errorsFound, fmt.Errorf("line %d: expected 8 fields", lineNumber))
			continue
		}
		destination, err := parseLinuxIPv4Hex(fields[1])
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d destination: %w", lineNumber, err))
			continue
		}
		gateway, err := parseLinuxIPv4Hex(fields[2])
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d gateway: %w", lineNumber, err))
			continue
		}
		mask, err := parseLinuxIPv4Hex(fields[7])
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d mask: %w", lineNumber, err))
			continue
		}
		prefix, _ := net.IPMask(mask.To4()).Size()
		metric, err := strconv.Atoi(fields[6])
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d metric: %w", lineNumber, err))
			continue
		}
		routes = append(routes, model.Route{Interface: fields[0], Destination: fmt.Sprintf("%s/%d", destination, prefix), Gateway: gateway.String(), Metric: metric, Family: 4})
	}
	if err := scanner.Err(); err != nil {
		errorsFound = append(errorsFound, err)
	}
	return routes, errorsFound
}

// parseLinuxIPv4Hex 解码 procfs 使用的四字节小端十六进制地址。
func parseLinuxIPv4Hex(value string) (net.IP, error) {
	data, err := hex.DecodeString(value)
	if err != nil || len(data) != 4 {
		return nil, fmt.Errorf("invalid IPv4 hex %q", value)
	}
	return net.IPv4(data[3], data[2], data[1], data[0]), nil
}
