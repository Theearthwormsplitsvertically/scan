package host

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/capability"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// factLimit prevents a malformed proc or sys fact file from consuming unbounded memory.
const factLimit = 1 << 20

// Collect gathers fixed, read-only host facts and marks missing optional DMI data as partial.
func Collect(ctx context.Context, root platform.Root) (model.Host, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "host", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	result := model.Host{CPUCount: runtime.NumCPU(), Architecture: runtime.GOARCH}
	defer func() {
		status.FinishedAt = time.Now().UTC()
		status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	}()
	if err := ctx.Err(); err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		return result, status
	}

	readTrimmed := func(path string) string {
		data, err := root.ReadFile(path, factLimit)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	if data, err := root.ReadFile("/etc/os-release", factLimit); err == nil {
		values := capability.ParseOSRelease(data)
		result.Distribution = values["PRETTY_NAME"]
		result.DistributionID = values["ID"]
		result.OSVersion = values["VERSION_ID"]
	} else {
		status.Errors = append(status.Errors, "/etc/os-release: "+err.Error())
	}
	result.Kernel = readTrimmed("/proc/sys/kernel/osrelease")
	result.Hostname = readTrimmed("/proc/sys/kernel/hostname")
	result.MachineID = readTrimmed("/etc/machine-id")
	result.BootID = readTrimmed("/proc/sys/kernel/random/boot_id")
	result.DMIUUID = readTrimmed("/sys/class/dmi/id/product_uuid")
	result.Vendor = readTrimmed("/sys/class/dmi/id/sys_vendor")
	result.Model = readTrimmed("/sys/class/dmi/id/product_name")
	if data, err := root.ReadFile("/proc/cpuinfo", 8<<20); err == nil {
		result.CPUModel = ParseCPUModel(data)
	}
	if data, err := root.ReadFile("/proc/meminfo", factLimit); err == nil {
		result.MemoryBytes = ParseMemoryBytes(data)
	}
	if result.MachineID != "" || result.DMIUUID != "" {
		result.ID = result.MachineID + ":" + result.DMIUUID
	}
	if len(status.Errors) > 0 || result.DMIUUID == "" {
		status.Status = model.StatusPartial
	}
	if result.Distribution == "" && result.Kernel == "" {
		status.Status = model.StatusFailed
	}
	status.Objects = 1
	return result, status
}
