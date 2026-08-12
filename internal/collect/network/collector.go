package network

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// InterfaceSource isolates standard-library interface queries so collectors are fixture-testable.
type InterfaceSource interface {
	Interfaces() ([]net.Interface, error)
	Addrs(net.Interface) ([]net.Addr, error)
}

// SystemInterfaceSource retrieves interfaces and addresses from the current network namespace.
type SystemInterfaceSource struct{}

// Interfaces returns all interfaces visible to the current process.
func (SystemInterfaceSource) Interfaces() ([]net.Interface, error) { return net.Interfaces() }

// Addrs returns addresses attached to one visible interface.
func (SystemInterfaceSource) Addrs(item net.Interface) ([]net.Addr, error) { return item.Addrs() }

// Collect gathers interfaces and IPs from the standard library, routes from procfs, and a DNS digest.
// It deliberately omits resolv.conf content and reports partial status for unavailable subdomains.
func Collect(ctx context.Context, root platform.Root, source InterfaceSource) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "network", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	interfaces := make([]model.NetworkInterface, 0)
	addresses := make([]model.Address, 0)
	routes := make([]model.Route, 0)
	finish := func() {
		status.FinishedAt = time.Now().UTC()
		status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
		status.Objects = len(interfaces) + len(addresses) + len(routes)
	}
	if err := ctx.Err(); err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		finish()
		return interfaces, addresses, routes, status
	}

	dnsDigest := ""
	if data, err := root.ReadFile("/etc/resolv.conf", 1<<20); err == nil {
		lines := make([]string, 0)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				lines = append(lines, strings.Join(strings.Fields(line), " "))
			}
		}
		digest := sha256.Sum256([]byte(strings.Join(lines, "\n")))
		dnsDigest = fmt.Sprintf("%x", digest)
	}
	items, err := source.Interfaces()
	if err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		finish()
		return interfaces, addresses, routes, status
	}
	for _, item := range items {
		flags := strings.Fields(item.Flags.String())
		interfaces = append(interfaces, model.NetworkInterface{Index: item.Index, Name: item.Name, MTU: item.MTU, MAC: item.HardwareAddr.String(), Flags: flags, DNSDigestSHA: dnsDigest})
		itemAddresses, err := source.Addrs(item)
		if err != nil {
			status.Errors = append(status.Errors, item.Name+": "+err.Error())
			continue
		}
		for _, address := range itemAddresses {
			cidr := address.String()
			family := 6
			ipText, _, err := net.ParseCIDR(cidr)
			if err == nil && ipText.To4() != nil {
				family = 4
			}
			addresses = append(addresses, model.Address{InterfaceIndex: item.Index, InterfaceName: item.Name, CIDR: cidr, Family: family})
		}
	}
	if data, err := root.ReadFile("/proc/net/route", 4<<20); err == nil {
		var routeErrors []error
		routes, routeErrors = ParseIPv4Routes(strings.NewReader(string(data)))
		for _, routeErr := range routeErrors {
			status.Errors = append(status.Errors, routeErr.Error())
		}
	} else {
		status.Errors = append(status.Errors, "/proc/net/route: "+err.Error())
	}
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Index < interfaces[j].Index })
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].CIDR < addresses[j].CIDR })
	sort.Slice(routes, func(i, j int) bool { return routes[i].Destination < routes[j].Destination })
	if len(status.Errors) > 0 {
		status.Status = model.StatusPartial
	}
	finish()
	return interfaces, addresses, routes, status
}
