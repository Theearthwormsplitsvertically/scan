package platform

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Root struct {
	base string
}

func NewRoot(base string) Root {
	absolute, err := filepath.Abs(base)
	if err != nil {
		absolute = base
	}
	return Root{base: filepath.Clean(absolute)}
}

func (root Root) Path(absoluteLinuxPath string) (string, error) {
	if !strings.HasPrefix(absoluteLinuxPath, "/") {
		return "", fmt.Errorf("path must be absolute: %q", absoluteLinuxPath)
	}
	relative := filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(filepath.Clean(absoluteLinuxPath)), "/"))
	resolved := filepath.Join(root.base, relative)
	if resolved != root.base && !strings.HasPrefix(resolved, root.base+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", absoluteLinuxPath)
	}
	return resolved, nil
}

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

func (root Root) ReadDir(path string) ([]fs.DirEntry, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(resolved)
}

func (root Root) Readlink(path string) (string, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return "", err
	}
	return os.Readlink(resolved)
}

func (root Root) Stat(path string) (fs.FileInfo, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(resolved)
}
