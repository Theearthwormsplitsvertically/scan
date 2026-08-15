package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestCollectReadsServiceUnitsAcrossDirectories(t *testing.T) {
	rootPath := t.TempDir()
	writeServiceFixture(t, rootPath, "/etc/systemd/system/nginx.service", "[Unit]\nDescription=Web Server\n\n[Service]\nExecStart=/usr/sbin/nginx\nUser=www-data\n")
	writeServiceFixture(t, rootPath, "/lib/systemd/system/ssh.service", "[Unit]\nDescription=SSH Server\n\n[Service]\nExecStart=/usr/sbin/sshd\n")

	services, status := Collect(context.Background(), platform.NewRoot(rootPath))
	if status.Status != model.StatusOK {
		t.Fatalf("status = %+v", status)
	}
	if len(services) != 2 {
		t.Fatalf("services = %d, want 2", len(services))
	}
	if services[0].UnitName != "nginx" || services[1].UnitName != "ssh" {
		t.Fatalf("services = %+v", services)
	}
	if services[0].Description != "Web Server" || services[0].FragmentPath != "/etc/systemd/system/nginx.service" {
		t.Fatalf("nginx = %+v", services[0])
	}
}

func TestCollectPrefersHigherPriorityDirectory(t *testing.T) {
	rootPath := t.TempDir()
	writeServiceFixture(t, rootPath, "/etc/systemd/system/nginx.service", "[Unit]\nDescription=Override\n")
	writeServiceFixture(t, rootPath, "/lib/systemd/system/nginx.service", "[Unit]\nDescription=Base\n")

	services, _ := Collect(context.Background(), platform.NewRoot(rootPath))
	if len(services) != 1 {
		t.Fatalf("services = %d, want 1", len(services))
	}
	if services[0].Description != "Override" || services[0].FragmentPath != "/etc/systemd/system/nginx.service" {
		t.Fatalf("service = %+v", services[0])
	}
}

func TestCollectUnsupportedWhenNoUnitDirectoriesExist(t *testing.T) {
	services, status := Collect(context.Background(), platform.NewRoot(t.TempDir()))
	if status.Status != model.StatusUnsupported {
		t.Fatalf("status = %+v", status)
	}
	if len(services) != 0 {
		t.Fatalf("services = %d, want 0", len(services))
	}
}

func writeServiceFixture(t *testing.T, rootPath, absolutePath, content string) {
	t.Helper()
	path := filepath.Join(rootPath, filepath.FromSlash(absolutePath[1:]))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
