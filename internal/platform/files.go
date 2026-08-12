// Package platform provides bounded, read-only access to a Linux-shaped filesystem root.
package platform

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// Root maps Linux absolute paths into one trusted filesystem root.
type Root struct {
	base string
}

// NewRoot creates a cleaned absolute root for production or fixture reads.
func NewRoot(base string) Root {
	absolute, err := filepath.Abs(base)
	if err != nil {
		absolute = base
	}
	return Root{base: filepath.Clean(absolute)}
}

// Path maps an absolute Linux path into Root and rejects paths that escape it.
func (root Root) Path(absoluteLinuxPath string) (string, error) {
	if !strings.HasPrefix(absoluteLinuxPath, "/") {
		return "", fmt.Errorf("path must be absolute: %q", absoluteLinuxPath)
	}
	cleaned := pathpkg.Clean(absoluteLinuxPath)
	relative := filepath.FromSlash(strings.TrimPrefix(cleaned, "/"))
	resolved := filepath.Join(root.base, relative)
	boundary := root.base
	volumeRoot := filepath.VolumeName(root.base) + string(filepath.Separator)
	if root.base != volumeRoot {
		boundary += string(filepath.Separator)
	}
	if resolved != root.base && !strings.HasPrefix(resolved, boundary) {
		return "", fmt.Errorf("path escapes root: %q", absoluteLinuxPath)
	}
	return resolved, nil
}

// ReadFile reads at most maximum bytes from a permitted path.
func (root Root) ReadFile(path string, maximum int64) ([]byte, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return data, nil
}

// ReadDir returns direct directory entries from a permitted path.
func (root Root) ReadDir(path string) ([]fs.DirEntry, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(resolved)
}

// Readlink resolves a permitted symbolic link without following it.
func (root Root) Readlink(path string) (string, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return "", err
	}
	return os.Readlink(resolved)
}

// Stat returns metadata for a permitted path.
func (root Root) Stat(path string) (fs.FileInfo, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(resolved)
}
