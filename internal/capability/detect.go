package capability

import (
	"context"
	"os"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// smallFactLimit 限制能力检测中每个轻量事实文件的最大读取量。
const smallFactLimit = 1 << 20

// Detect 从固定的只读 Linux 事实路径构建 doctor 报告。
// 缺失可选事实时返回能力降级，而不是让整个 doctor 失败。
func Detect(ctx context.Context, root platform.Root, architecture string) model.DoctorReport {
	report := model.DoctorReport{
		SchemaVersion: model.SchemaVersion,
		Agent:         model.AgentInfo{Name: "asset-agent", Version: buildinfo.Version, Commit: buildinfo.Commit, BuildTime: buildinfo.BuildTime},
		OS:            "linux", Architecture: architecture,
		Capabilities: model.CapabilityReport{Items: make([]model.Capability, 0, 16)},
	}
	if ctx.Err() != nil {
		return report
	}

	if data, err := root.ReadFile("/etc/os-release", smallFactLimit); err == nil {
		values := ParseOSRelease(data)
		report.Distribution = values["PRETTY_NAME"]
		if report.Distribution == "" {
			report.Distribution = values["ID"]
		}
	}
	if data, err := root.ReadFile("/proc/sys/kernel/osrelease", smallFactLimit); err == nil {
		report.Kernel = strings.TrimSpace(string(data))
	}
	if data, err := root.ReadFile("/proc/self/status", smallFactLimit); err == nil {
		status := ParseSelfStatus(data)
		report.Root = status.UIDs[0] == 0
		report.Capabilities.Items = append(report.Capabilities.Items,
			model.Capability{Name: "procfs", Status: model.StatusOK, Detail: "readable"},
			model.Capability{Name: "linux_capabilities", Status: model.StatusOK, Detail: formatCapabilities(status.CapEff)},
		)
	} else {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "procfs", Status: model.StatusUnsupported, Detail: "unavailable"})
	}
	if _, err := root.Stat("/sys"); err == nil {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "sysfs", Status: model.StatusOK, Detail: "readable"})
	} else {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "sysfs", Status: model.StatusUnsupported, Detail: "unavailable"})
	}

	initDetail := "unknown"
	initStatus := model.StatusDegraded
	if data, err := root.ReadFile("/proc/1/comm", smallFactLimit); err == nil {
		initDetail = strings.TrimSpace(string(data))
		if initDetail != "" {
			initStatus = model.StatusOK
		}
	}
	report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "init", Status: initStatus, Detail: initDetail})

	if _, err := root.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "cgroup", Status: model.StatusOK, Detail: "v2"})
	} else if _, err := root.Stat("/proc/1/cgroup"); err == nil {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "cgroup", Status: model.StatusDegraded, Detail: "v1 or hybrid"})
	} else {
		report.Capabilities.Items = append(report.Capabilities.Items, model.Capability{Name: "cgroup", Status: model.StatusUnsupported, Detail: "not detected"})
	}
	report.Capabilities.Items = append(report.Capabilities.Items,
		securityModuleCapability(root, "selinux", "/sys/fs/selinux/enforce", "1"),
		securityModuleCapability(root, "apparmor", "/sys/module/apparmor/parameters/enabled", "Y"),
		socketCapability(root, "docker", "/var/run/docker.sock"),
		socketCapability(root, "containerd", "/run/containerd/containerd.sock"),
		model.Capability{Name: "sock_diag", Status: model.StatusDegraded, Detail: "/proc/net fallback", Fallback: "/proc/net/tcp*,/proc/net/udp*"},
		model.Capability{Name: "process_connector", Status: model.StatusDegraded, Detail: "polling fallback", Fallback: "/proc PID + starttime"},
		model.Capability{Name: "netlink", Status: model.StatusDegraded, Detail: "standard library fallback"},
		model.Capability{Name: "inotify", Status: model.StatusUnsupported, Detail: "not probed in this milestone"},
	)
	return report
}

// formatCapabilities 将 Linux capability 位掩码渲染为文本，无需执行外部命令。
func formatCapabilities(value uint64) string {
	const digits = "0123456789abcdef"
	buffer := make([]byte, 18)
	buffer[0], buffer[1] = '0', 'x'
	for index := 17; index >= 2; index-- {
		buffer[index] = digits[value&0xf]
		value >>= 4
	}
	return string(buffer)
}

// securityModuleCapability 将 SELinux 或 AppArmor 启用文件转换为能力记录。
func securityModuleCapability(root platform.Root, name, path, enabledValue string) model.Capability {
	data, err := root.ReadFile(path, smallFactLimit)
	if err != nil {
		return model.Capability{Name: name, Status: model.StatusUnsupported, Detail: "not detected"}
	}
	detail := "disabled"
	if strings.EqualFold(strings.TrimSpace(string(data)), enabledValue) {
		detail = "enabled"
	}
	return model.Capability{Name: name, Status: model.StatusOK, Detail: detail}
}

// socketCapability 报告本地运行时控制 socket 是否存在或无法访问。
func socketCapability(root platform.Root, name, path string) model.Capability {
	if _, err := root.Stat(path); err == nil {
		return model.Capability{Name: name, Status: model.StatusOK, Detail: "socket detected"}
	} else if !os.IsNotExist(err) {
		return model.Capability{Name: name, Status: model.StatusDegraded, Detail: "socket inaccessible"}
	}
	return model.Capability{Name: name, Status: model.StatusUnsupported, Detail: "socket unavailable"}
}
