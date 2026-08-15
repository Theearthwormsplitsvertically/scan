// service 包采集 systemd *.service Unit 的静态文件事实。
package service

import (
	"context"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// unitFileLimit 限制单个 Unit 文件的最大读取字节数。
const unitFileLimit = 1 << 20

// unitDirectories 按优先级列出 systemd Unit 搜索目录。
var unitDirectories = []string{
	"/etc/systemd/system",
	"/run/systemd/system",
	"/usr/lib/systemd/system",
	"/lib/systemd/system",
}

// Collect 枚举并解析 *.service Unit 文件，按目录优先级去重。
func Collect(ctx context.Context, root platform.Root) ([]model.Service, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "service", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	seen := make(map[string]model.Service)
	readableDirs := 0
	for _, directory := range unitDirectories {
		if err := ctx.Err(); err != nil {
			status.Status = model.StatusPartial
			status.Errors = append(status.Errors, err.Error())
			break
		}
		entries, err := root.ReadDir(directory)
		if err != nil {
			if !os.IsNotExist(err) {
				status.Errors = append(status.Errors, directory+": "+err.Error())
			}
			continue
		}
		readableDirs++
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".service") {
				continue
			}
			unitName := strings.TrimSuffix(name, ".service")
			if _, exists := seen[unitName]; exists {
				continue
			}
			filePath := path.Join(directory, name)
			data, err := root.ReadFile(filePath, unitFileLimit)
			if err != nil {
				status.Errors = append(status.Errors, filePath+": "+err.Error())
				continue
			}
			unit, err := ParseUnit(data, unitName)
			if err != nil {
				status.Errors = append(status.Errors, filePath+": "+err.Error())
				continue
			}
			unit.LoadState = "loaded"
			unit.FragmentPath = filePath
			seen[unitName] = unit
		}
	}
	services := make([]model.Service, 0, len(seen))
	for _, unit := range seen {
		services = append(services, unit)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].UnitName < services[j].UnitName })
	if readableDirs == 0 {
		status.Status = model.StatusUnsupported
	}
	if len(status.Errors) > 0 && status.Status == model.StatusOK {
		status.Status = model.StatusPartial
	}
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = len(services)
	return services, status
}
