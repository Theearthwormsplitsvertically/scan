package capability

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestDetectReportsPresentAndMissingLinuxCapabilities(t *testing.T) {
	t.Parallel()

	rootPath := t.TempDir()
	writeFixture(t, rootPath, "/etc/os-release", "ID=ubuntu\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n")
	writeFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeFixture(t, rootPath, "/proc/self/status", "Uid:\t0\t0\t0\t0\nGid:\t0\t0\t0\t0\nCapEff:\t0000000000003fff\n")
	writeFixture(t, rootPath, "/proc/1/comm", "systemd\n")
	writeFixture(t, rootPath, "/sys/fs/cgroup/cgroup.controllers", "cpu memory io\n")
	writeFixture(t, rootPath, "/sys/module/apparmor/parameters/enabled", "Y\n")

	report := Detect(context.Background(), platform.NewRoot(rootPath), "amd64")

	if report.SchemaVersion != model.SchemaVersion {
		t.Fatalf("schema = %q, want %q", report.SchemaVersion, model.SchemaVersion)
	}
	if report.OS != "linux" || report.Distribution != "Ubuntu 24.04 LTS" {
		t.Fatalf("OS/distribution = %q/%q", report.OS, report.Distribution)
	}
	if report.Kernel != "6.8.0-test" || report.Architecture != "amd64" || !report.Root {
		t.Fatalf("kernel/arch/root = %q/%q/%v", report.Kernel, report.Architecture, report.Root)
	}
	assertCapability(t, report.Capabilities.Items, "init", model.StatusOK, "systemd")
	assertCapability(t, report.Capabilities.Items, "cgroup", model.StatusOK, "v2")
	assertCapability(t, report.Capabilities.Items, "apparmor", model.StatusOK, "enabled")
	assertCapability(t, report.Capabilities.Items, "docker", model.StatusUnsupported, "socket unavailable")
	assertCapability(t, report.Capabilities.Items, "selinux", model.StatusUnsupported, "not detected")
	assertCapability(t, report.Capabilities.Items, "sock_diag", model.StatusDegraded, "/proc/net fallback")
}

func writeFixture(t *testing.T, root, path, value string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path[1:]))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%s): %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(value), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", fullPath, err)
	}
}

func assertCapability(t *testing.T, items []model.Capability, name string, status model.Status, detail string) {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			if item.Status != status || item.Detail != detail {
				t.Fatalf("capability %s = %s/%q, want %s/%q", name, item.Status, item.Detail, status, detail)
			}
			return
		}
	}
	t.Fatalf("capability %s not found", name)
}
