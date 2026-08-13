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

// InterfaceSource 隔离标准库网络接口查询，使采集器可使用 fixture 测试。
type InterfaceSource interface {
	Interfaces() ([]net.Interface, error)
	Addrs(net.Interface) ([]net.Addr, error)
}

// SystemInterfaceSource 从当前网络命名空间获取接口和地址。
type SystemInterfaceSource struct{}

// Interfaces 返回当前进程可见的所有接口。
func (SystemInterfaceSource) Interfaces() ([]net.Interface, error) { return net.Interfaces() }

// Addrs 返回一个可见接口绑定的地址。
func (SystemInterfaceSource) Addrs(item net.Interface) ([]net.Addr, error) { return item.Addrs() }

// Collect 通过标准库采集接口和 IP，通过 procfs 采集路由，并生成 DNS 摘要。
// 它有意不输出 resolv.conf 内容，子域不可用时返回 partial 状态。
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
