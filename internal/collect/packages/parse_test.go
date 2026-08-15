package packages

import "testing"

func TestParseDPKGStatusExtractsPackages(t *testing.T) {
	t.Parallel()

	data := []byte("Package: nginx\nStatus: install ok installed\nInstalled-Size: 2048\nMaintainer: Ubuntu Developers\nArchitecture: amd64\nVersion: 1.24.0-1\nDescription: high performance web server\n light and fast\n\nPackage: curl\nArchitecture: amd64\nVersion: 8.0.1\n")
	packages, err := ParseDPKGStatus(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(packages))
	}
	first := packages[0]
	if first.Name != "nginx" || first.Version != "1.24.0-1" || first.Architecture != "amd64" ||
		first.Maintainer != "Ubuntu Developers" || first.InstalledSizeBytes != 2_097_152 {
		t.Fatalf("nginx = %+v", first)
	}
	if first.Description != "high performance web server" {
		t.Fatalf("description = %q", first.Description)
	}
}

func TestParseAPKInstalledExtractsPackages(t *testing.T) {
	t.Parallel()

	data := []byte("P:busybox\nV:1.34.1-r7\nA:x86_64\nS:943718\nT:The Swiss Army Knife\n\nP:musl\nV:1.2.3\nA:x86_64\n")
	packages, err := ParseAPKInstalled(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(packages))
	}
	if packages[0].Name != "busybox" || packages[0].Version != "1.34.1-r7" ||
		packages[0].Architecture != "x86_64" || packages[0].InstalledSizeBytes != 943718 {
		t.Fatalf("busybox = %+v", packages[0])
	}
}
