package host

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestCollectReturnsNineHostFacts(t *testing.T) {
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/hostname", "server-1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/sys/class/dmi/id/product_uuid", "dmi-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")

	got, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusOK {
		t.Fatalf("status = %q, errors = %v", status.Status, status.Errors)
	}
	want := model.Host{
		Hostname: "server-1", DistributionName: "Example Linux 1",
		DistributionID: "example", DistributionVersion: "1",
		KernelRelease: "6.8.0-test", Architecture: runtime.GOARCH,
		MemoryTotalBytes: 2_097_152, BootID: "boot-1", DMIUUID: "dmi-1",
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

func TestCollectFallsBackToHostnameWhenDMIReadFails(t *testing.T) {
	rootPath := writeCompleteHostFixture(t)
	dmiPath := filepath.Join(rootPath, "sys", "class", "dmi", "id", "product_uuid")
	if err := os.Remove(dmiPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dmiPath, 0o755); err != nil {
		t.Fatal(err)
	}

	host, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusPartial || status.Objects != 1 {
		t.Fatalf("status = %q, errors = %v", status.Status, status.Errors)
	}
	if host.DMIUUID != "" {
		t.Fatalf("dmi UUID = %q", host.DMIUUID)
	}
	if !containsHostError(status.Errors, "/sys/class/dmi/id/product_uuid:") {
		t.Fatalf("errors = %v", status.Errors)
	}
}

func TestCollectMarksMissingRequiredFactsPartial(t *testing.T) {
	tests := []struct {
		name        string
		prepare     func(t *testing.T, rootPath string)
		errorNeedle string
		assertFact  func(t *testing.T, host model.Host)
	}{
		{
			name: "hostname", prepare: func(t *testing.T, rootPath string) {
				removeHostFixture(t, rootPath, "/proc/sys/kernel/hostname")
			},
			errorNeedle: "/proc/sys/kernel/hostname:",
			assertFact: func(t *testing.T, host model.Host) {
				if host.Hostname != "" {
					t.Fatalf("hostname = %q", host.Hostname)
				}
			},
		},
		{
			name: "distribution name", prepare: func(t *testing.T, rootPath string) {
				writeHostFixture(t, rootPath, "/etc/os-release", "ID=example\nVERSION_ID=1\n")
			},
			errorNeedle: "/etc/os-release: missing distribution_name",
			assertFact: func(t *testing.T, host model.Host) {
				if host.DistributionName != "" || host.DistributionID != "example" || host.DistributionVersion != "1" {
					t.Fatalf("distribution = %#v", host)
				}
			},
		},
		{
			name: "distribution id", prepare: func(t *testing.T, rootPath string) {
				writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nVERSION_ID=1\n")
			},
			errorNeedle: "/etc/os-release: missing distribution_id",
			assertFact: func(t *testing.T, host model.Host) {
				if host.DistributionName != "Example Linux 1" || host.DistributionID != "" || host.DistributionVersion != "1" {
					t.Fatalf("distribution = %#v", host)
				}
			},
		},
		{
			name: "distribution version", prepare: func(t *testing.T, rootPath string) {
				writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\n")
			},
			errorNeedle: "/etc/os-release: missing distribution_version",
			assertFact: func(t *testing.T, host model.Host) {
				if host.DistributionName != "Example Linux 1" || host.DistributionID != "example" || host.DistributionVersion != "" {
					t.Fatalf("distribution = %#v", host)
				}
			},
		},
		{
			name: "kernel release", prepare: func(t *testing.T, rootPath string) {
				removeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease")
			},
			errorNeedle: "/proc/sys/kernel/osrelease:",
			assertFact: func(t *testing.T, host model.Host) {
				if host.KernelRelease != "" {
					t.Fatalf("kernel release = %q", host.KernelRelease)
				}
			},
		},
		{
			name: "boot id", prepare: func(t *testing.T, rootPath string) {
				removeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id")
			},
			errorNeedle: "/proc/sys/kernel/random/boot_id:",
			assertFact: func(t *testing.T, host model.Host) {
				if host.BootID != "" {
					t.Fatalf("boot ID = %q", host.BootID)
				}
			},
		},
		{
			name: "missing memory total", prepare: func(t *testing.T, rootPath string) {
				writeHostFixture(t, rootPath, "/proc/meminfo", "MemFree: 1024 kB\n")
			},
			errorNeedle: "/proc/meminfo: missing or invalid MemTotal",
			assertFact: func(t *testing.T, host model.Host) {
				if host.MemoryTotalBytes != 0 {
					t.Fatalf("memory total = %d", host.MemoryTotalBytes)
				}
			},
		},
		{
			name: "invalid memory total", prepare: func(t *testing.T, rootPath string) {
				writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: broken kB\n")
			},
			errorNeedle: "/proc/meminfo: missing or invalid MemTotal",
			assertFact: func(t *testing.T, host model.Host) {
				if host.MemoryTotalBytes != 0 {
					t.Fatalf("memory total = %d", host.MemoryTotalBytes)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootPath := writeCompleteHostFixture(t)
			test.prepare(t, rootPath)

			host, status := Collect(t.Context(), platform.NewRoot(rootPath))
			if status.Status != model.StatusPartial || status.Objects != 1 {
				t.Fatalf("status = %+v", status)
			}
			if host.DMIUUID != "dmi-1" || host.Architecture != runtime.GOARCH {
				t.Fatalf("successful facts = %#v", host)
			}
			if !containsHostError(status.Errors, test.errorNeedle) {
				t.Fatalf("errors = %v, want %q", status.Errors, test.errorNeedle)
			}
			test.assertFact(t, host)
			assertRemainingHostFacts(t, test.name, host)
		})
	}
}

func TestCollectSetsTimingOnSuccessfulAndFailedPaths(t *testing.T) {
	tests := []struct {
		name string
		ctx  func(t *testing.T) context.Context
		root func(t *testing.T) string
	}{
		{
			name: "success",
			ctx:  func(t *testing.T) context.Context { return t.Context() },
			root: func(t *testing.T) string { return writeCompleteHostFixture(t) },
		},
		{
			name: "cancelled",
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			root: func(t *testing.T) string { return t.TempDir() },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, status := Collect(test.ctx(t), platform.NewRoot(test.root(t)))
			if status.StartedAt.IsZero() || status.FinishedAt.IsZero() {
				t.Fatalf("timing = %+v", status)
			}
			if status.FinishedAt.Before(status.StartedAt) || status.DurationMS < 0 {
				t.Fatalf("invalid timing = %+v", status)
			}
			if status.DurationMS != status.FinishedAt.Sub(status.StartedAt).Milliseconds() {
				t.Fatalf("duration = %d, want %d", status.DurationMS, status.FinishedAt.Sub(status.StartedAt).Milliseconds())
			}
		})
	}
}

func writeCompleteHostFixture(t *testing.T) string {
	t.Helper()
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/hostname", "server-1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/sys/class/dmi/id/product_uuid", "dmi-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")
	return rootPath
}

func removeHostFixture(t *testing.T, rootPath, absolutePath string) {
	t.Helper()
	path := filepath.Join(rootPath, filepath.FromSlash(absolutePath[1:]))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func containsHostError(errors []string, needle string) bool {
	for _, message := range errors {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func assertRemainingHostFacts(t *testing.T, missing string, host model.Host) {
	t.Helper()
	if missing != "hostname" && host.Hostname != "server-1" {
		t.Fatalf("hostname = %q", host.Hostname)
	}
	if missing != "distribution name" && host.DistributionName != "Example Linux 1" {
		t.Fatalf("distribution name = %q", host.DistributionName)
	}
	if missing != "distribution id" && host.DistributionID != "example" {
		t.Fatalf("distribution ID = %q", host.DistributionID)
	}
	if missing != "distribution version" && host.DistributionVersion != "1" {
		t.Fatalf("distribution version = %q", host.DistributionVersion)
	}
	if missing != "kernel release" && host.KernelRelease != "6.8.0-test" {
		t.Fatalf("kernel release = %q", host.KernelRelease)
	}
	if missing != "boot id" && host.BootID != "boot-1" {
		t.Fatalf("boot ID = %q", host.BootID)
	}
	if missing != "missing memory total" && missing != "invalid memory total" && host.MemoryTotalBytes != 2_097_152 {
		t.Fatalf("memory total = %d", host.MemoryTotalBytes)
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
