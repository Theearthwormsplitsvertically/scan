package platform

import (
	"path/filepath"
	"testing"
)

func TestRootPathAcceptsLinuxFactsWhenRootIsFilesystemRoot(t *testing.T) {
	t.Parallel()

	root := NewRoot(string(filepath.Separator))
	if _, err := root.Path("/proc/version"); err != nil {
		t.Fatalf("Path(/proc/version) error = %v", err)
	}
}
