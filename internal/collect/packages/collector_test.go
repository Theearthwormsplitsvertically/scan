package packages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestCollectParsesDPKGStatus(t *testing.T) {
	rootPath := t.TempDir()
	writePackageFixture(t, rootPath, "/var/lib/dpkg/status", "Package: nginx\nArchitecture: amd64\nVersion: 1.24.0\n")

	packages, status := Collect(context.Background(), platform.NewRoot(rootPath))
	if status.Status != model.StatusOK {
		t.Fatalf("status = %+v", status)
	}
	if len(packages) != 1 || packages[0].Name != "nginx" || packages[0].Source != "dpkg" {
		t.Fatalf("packages = %+v", packages)
	}
}

func TestCollectUnsupportedWithoutPackageDatabase(t *testing.T) {
	packages, status := Collect(context.Background(), platform.NewRoot(t.TempDir()))
	if status.Status != model.StatusUnsupported {
		t.Fatalf("status = %+v", status)
	}
	if len(packages) != 0 {
		t.Fatalf("packages = %d, want 0", len(packages))
	}
}

func TestCollectUnsupportedForRPMDatabase(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "var", "lib", "rpm"), 0o755); err != nil {
		t.Fatal(err)
	}

	packages, status := Collect(context.Background(), platform.NewRoot(rootPath))
	if status.Status != model.StatusUnsupported {
		t.Fatalf("status = %+v", status)
	}
	if len(packages) != 0 {
		t.Fatalf("packages = %d, want 0", len(packages))
	}
}

func writePackageFixture(t *testing.T, rootPath, absolutePath, content string) {
	t.Helper()
	path := filepath.Join(rootPath, filepath.FromSlash(absolutePath[1:]))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
