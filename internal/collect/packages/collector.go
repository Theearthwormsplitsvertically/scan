// packages 包采集已安装软件包事实，只读包数据库文件，不执行包管理器命令。
package packages

import (
	"context"
	"sort"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// packageDBLimit 限制包数据库文件的最大读取字节数（真实服务器 dpkg status 可达几十 MB）。
const packageDBLimit = 64 << 20

// Collect 按发行版选择 dpkg / apk 包数据库并解析已安装软件包。
// rpm 二进制数据库无法用只读文件方式解析，检测到后标记 unsupported，不静默跳过。
func Collect(ctx context.Context, root platform.Root) ([]model.Package, model.CollectorStatus) {
	started := time.Now().UTC()
	status := model.CollectorStatus{Collector: "package", Status: model.StatusOK, StartedAt: started, Errors: []string{}}
	packages := make([]model.Package, 0)

	switch {
	case fileExists(root, "/var/lib/dpkg/status"):
		packages = collectFrom(root, "dpkg", "/var/lib/dpkg/status", ParseDPKGStatus, &status)
	case fileExists(root, "/lib/apk/db/installed"):
		packages = collectFrom(root, "apk", "/lib/apk/db/installed", ParseAPKInstalled, &status)
	default:
		if directory, ok := rpmDatabasePath(root); ok {
			status.Status = model.StatusUnsupported
			status.Errors = append(status.Errors, directory+": rpm 二进制数据库需 rpm 工具链，文件解析暂不支持")
		} else {
			status.Status = model.StatusUnsupported
			status.Errors = append(status.Errors, "未检测到 dpkg/rpm/apk 包数据库")
		}
	}

	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	if len(status.Errors) > 0 && status.Status == model.StatusOK {
		status.Status = model.StatusPartial
	}
	status.FinishedAt = time.Now().UTC()
	status.DurationMS = status.FinishedAt.Sub(started).Milliseconds()
	status.Objects = len(packages)
	return packages, status
}

// collectFrom 读取并解析一个包数据库文件，填充 source 字段。
func collectFrom(root platform.Root, source, path string, parse func([]byte) ([]model.Package, error), status *model.CollectorStatus) []model.Package {
	data, err := root.ReadFile(path, packageDBLimit)
	if err != nil {
		status.Errors = append(status.Errors, path+": "+err.Error())
		return []model.Package{}
	}
	packages, err := parse(data)
	if err != nil {
		status.Errors = append(status.Errors, path+": "+err.Error())
		return []model.Package{}
	}
	for index := range packages {
		packages[index].Source = source
	}
	return packages
}

// fileExists 判断允许路径是否是一个普通文件。
func fileExists(root platform.Root, path string) bool {
	info, err := root.Stat(path)
	return err == nil && !info.IsDir()
}

// rpmDatabasePath 返回检测到的 rpm 数据库目录路径。
func rpmDatabasePath(root platform.Root) (string, bool) {
	for _, directory := range []string{"/var/lib/rpm", "/usr/lib/sysimage/rpm"} {
		if info, err := root.Stat(directory); err == nil && info.IsDir() {
			return directory, true
		}
	}
	return "", false
}
