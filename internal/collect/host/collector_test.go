package host

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestCollectReturnsMinimalHostFactsAndAcceptsMissingDMI(t *testing.T) {
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/hostname", "server-1\n")
	writeHostFixture(t, rootPath, "/etc/machine-id", "machine-1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")

	got, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusOK {
		t.Fatalf("status = %q, errors = %v", status.Status, status.Errors)
	}
	want := model.Host{
		Hostname: "server-1", DistributionName: "Example Linux 1",
		DistributionID: "example", DistributionVersion: "1",
		KernelRelease: "6.8.0-test", Architecture: runtime.GOARCH,
		MemoryTotalBytes: 2_097_152, MachineID: "machine-1", BootID: "boot-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %#v, want %#v", got, want)
	}
}

func TestCollectFailsWhenNoHostIdentityCanBeEstablished(t *testing.T) {
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")

	_, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusFailed || status.Objects != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func writeHostFixture(t *testing.T, rootPath, absolutePath, content string) {
	t.Helper()
	path := filepath.Join(rootPath, filepath.FromSlash(absolutePath[1:]))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
