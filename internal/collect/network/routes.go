// Package network collects interfaces, addresses, DNS summaries, and IP routes.
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

// ParseIPv4Routes parses /proc/net/route and converts Linux little-endian IPv4 fields.
// Malformed rows are returned as errors while valid rows remain available.
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

// parseLinuxIPv4Hex decodes the four-byte, little-endian hexadecimal address used by procfs.
func parseLinuxIPv4Hex(value string) (net.IP, error) {
	data, err := hex.DecodeString(value)
	if err != nil || len(data) != 4 {
		return nil, fmt.Errorf("invalid IPv4 hex %q", value)
	}
	return net.IPv4(data[3], data[2], data[1], data[0]), nil
}
