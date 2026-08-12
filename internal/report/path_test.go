package report

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefaultOutputPathCreatesPrivateDirectoryBesideExecutable(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	executable := filepath.Join(installDir, "asset-agent")
	if err := os.WriteFile(executable, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 9, 1, 2, 0, time.FixedZone("local", 8*60*60))

	path, err := DefaultOutputPath(executable, "socket", now)

	if err != nil {
		t.Fatalf("DefaultOutputPath() error = %v", err)
	}
	want := filepath.Join(installDir, "output", "socket-20260812T010102Z.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	info, err := os.Stat(filepath.Join(installDir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o, want 700", info.Mode().Perm())
	}
}

func TestDefaultOutputPathDoesNotOverwriteExistingReport(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	executable := filepath.Join(installDir, "asset-agent")
	if err := os.WriteFile(executable, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 1, 2, 3, 0, time.UTC)
	first, err := DefaultOutputPath(executable, "host", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	second, err := DefaultOutputPath(executable, "host", now)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(second, "host-20260812T010203Z-1.json") {
		t.Fatalf("second path = %q", second)
	}
}

func TestDefaultOutputPathRejectsEmptyExecutablePath(t *testing.T) {
	t.Parallel()

	_, err := DefaultOutputPath("", "host", time.Now())

	if err == nil {
		t.Fatal("DefaultOutputPath() error = nil, want error")
	}
}
