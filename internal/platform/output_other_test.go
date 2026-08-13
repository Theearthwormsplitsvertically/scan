//go:build !linux

package platform

import (
	"path/filepath"
	"testing"
)

func TestDefaultOutputRootIsSafeAbsoluteDirectory(t *testing.T) {
	root, err := DefaultOutputRoot()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(root) || filepath.Base(root) != "output" || filepath.Dir(root) == root {
		t.Fatalf("root = %q", root)
	}
}
