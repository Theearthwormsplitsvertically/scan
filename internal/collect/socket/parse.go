package socket

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

// ParseProcNet 将一个 /proc/net/tcp*、udp* 或 IPv6 表规范化为 Socket 记录。
// 无效行单独返回，避免一条损坏的 procfs 行丢弃其他有效 socket。
func ParseProcNet(reader io.Reader, protocol string, family int, netns string) ([]model.Socket, []error) {
	result := make([]model.Socket, 0)
	errorsFound := make([]error, 0)
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber == 1 {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			errorsFound = append(errorsFound, fmt.Errorf("line %d: expected at least 10 fields", lineNumber))
			continue
		}
		localAddress, localPort, err := parseEndpoint(fields[1], family)
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d local endpoint: %w", lineNumber, err))
			continue
		}
		remoteAddress, remotePort, err := parseEndpoint(fields[2], family)
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d remote endpoint: %w", lineNumber, err))
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("line %d inode: %w", lineNumber, err))
			continue
		}
		result = append(result, model.Socket{
			ID: fmt.Sprintf("%s:%d:%s", netns, inode, protocol), Protocol: protocol, Family: family,
			State: socketState(protocol, fields[3]), LocalAddress: localAddress, LocalPort: localPort,
			RemoteAddress: remoteAddress, RemotePort: remotePort, Inode: inode, NetworkNS: netns,
			PIDs: []int{}, ProcessIDs: []string{},
		})
	}
	if err := scanner.Err(); err != nil {
		errorsFound = append(errorsFound, err)
	}
	return result, errorsFound
}

// parseEndpoint 将一个十六进制 procfs 端点解码为 IP 地址和端口。
// Linux 以小端序存储 IPv4，以反转的 32 位字存储 IPv6。
func parseEndpoint(value string, family int) (string, int, error) {
	addressHex, portHex, found := strings.Cut(value, ":")
	if !found {
		return "", 0, fmt.Errorf("missing port")
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, err
	}
	data, err := hex.DecodeString(addressHex)
	if err != nil {
		return "", 0, err
	}
	switch family {
	case 4:
		if len(data) != 4 {
			return "", 0, fmt.Errorf("IPv4 length %d", len(data))
		}
		data[0], data[3] = data[3], data[0]
		data[1], data[2] = data[2], data[1]
	case 6:
		if len(data) != 16 {
			return "", 0, fmt.Errorf("IPv6 length %d", len(data))
		}
		for offset := 0; offset < len(data); offset += 4 {
			data[offset], data[offset+3] = data[offset+3], data[offset]
			data[offset+1], data[offset+2] = data[offset+2], data[offset+1]
		}
	default:
		return "", 0, fmt.Errorf("unsupported family %d", family)
	}
	return net.IP(data).String(), int(port), nil
}

// socketState 将 Linux 十六进制状态码转换为可读的 TCP 或 UDP 状态。
func socketState(protocol, value string) string {
	if protocol == "udp" {
		switch value {
		case "01":
			return "ESTABLISHED"
		case "07":
			return "UNCONNECTED"
		default:
			return "UDP_" + value
		}
	}
	states := map[string]string{"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV", "04": "FIN_WAIT1", "05": "FIN_WAIT2", "06": "TIME_WAIT", "07": "CLOSE", "08": "CLOSE_WAIT", "09": "LAST_ACK", "0A": "LISTEN", "0B": "CLOSING"}
	if state, exists := states[value]; exists {
		return state
	}
	return "TCP_" + value
}
