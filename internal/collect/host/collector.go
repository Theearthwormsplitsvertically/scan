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

const factLimit = 1 << 20

// Collect gathers fixed, read-only host facts and marks the result based on completeness.
func Collect(ctx context.Context, root platform.Root) (result model.Host, status model.CollectorStatus) {
	started := time.Now().UTC()
	status = model.CollectorStatus{Collector: "host", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	result = model.Host{Architecture: runtime.GOARCH}
	defer func() {
		status.FinishedAt = time.Now().UTC()
		status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	}()
	if err := ctx.Err(); err != nil {
		status.Status = model.StatusFailed
		status.Errors = append(status.Errors, err.Error())
		return result, status
	}

	readRequired := func(path string) string {
		data, err := root.ReadFile(path, factLimit)
		if err != nil {
			status.Errors = append(status.Errors, path+": "+err.Error())
			return ""
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			status.Errors = append(status.Errors, path+": empty value")
		}
		return value
	}
	readIdentity := func(path string) (string, error) {
		data, err := root.ReadFile(path, factLimit)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}

	if data, err := root.ReadFile("/etc/os-release", factLimit); err == nil {
		values := capability.ParseOSRelease(data)
		result.DistributionName = strings.TrimSpace(values["PRETTY_NAME"])
		result.DistributionID = strings.TrimSpace(values["ID"])
		result.DistributionVersion = strings.TrimSpace(values["VERSION_ID"])
		for name, value := range map[string]string{
			"distribution_name":    result.DistributionName,
			"distribution_id":      result.DistributionID,
			"distribution_version": result.DistributionVersion,
		} {
			if value == "" {
				status.Errors = append(status.Errors, "/etc/os-release: missing "+name)
			}
		}
	} else {
		status.Errors = append(status.Errors, "/etc/os-release: "+err.Error())
	}
	result.KernelRelease = readRequired("/proc/sys/kernel/osrelease")
	result.Hostname = readRequired("/proc/sys/kernel/hostname")
	result.BootID = readRequired("/proc/sys/kernel/random/boot_id")
	var dmiUUIDErr error
	result.DMIUUID, dmiUUIDErr = readIdentity("/sys/class/dmi/id/product_uuid")
	if data, err := root.ReadFile("/proc/meminfo", factLimit); err == nil {
		result.MemoryTotalBytes = ParseMemoryBytes(data)
		if result.MemoryTotalBytes == 0 {
			status.Errors = append(status.Errors, "/proc/meminfo: missing or invalid MemTotal")
		}
	} else {
		status.Errors = append(status.Errors, "/proc/meminfo: "+err.Error())
	}

	hasStableIdentity := result.DMIUUID != ""
	if !hasStableIdentity {
		status.Errors = append(status.Errors, "stable host identity unavailable")
		if dmiUUIDErr != nil {
			status.Errors = append(status.Errors, "/sys/class/dmi/id/product_uuid: "+dmiUUIDErr.Error())
		}
	}
	switch {
	case !hasStableIdentity && result.Hostname == "":
		status.Status = model.StatusFailed
		status.Objects = 0
	case len(status.Errors) > 0:
		status.Status = model.StatusPartial
		status.Objects = 1
	default:
		status.Status = model.StatusOK
		status.Objects = 1
	}
	return result, status
}
