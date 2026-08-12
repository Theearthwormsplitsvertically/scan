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
