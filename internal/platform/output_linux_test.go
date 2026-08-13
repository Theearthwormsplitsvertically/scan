//go:build linux

package platform

import "testing"

func TestDefaultOutputRootIsLinuxSystemDirectory(t *testing.T) {
	root, err := DefaultOutputRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != "/var/lib/asset-agent/output" {
		t.Fatalf("root = %q", root)
	}
}
