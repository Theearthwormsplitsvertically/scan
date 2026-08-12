// platform 包提供对 Linux 目录结构根的有界只读访问。
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

// Root 将 Linux 绝对路径映射到一个受信任的文件系统根。
type Root struct {
	base string
}

// NewRoot 创建经过清理的绝对根目录，用于生产读取或 fixture 读取。
func NewRoot(base string) Root {
	absolute, err := filepath.Abs(base)
	if err != nil {
		absolute = base
	}
	return Root{base: filepath.Clean(absolute)}
}

// Path 将 Linux 绝对路径映射到 Root 内，并拒绝越出该根目录的路径。
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

// ReadFile 从允许路径最多读取 maximum 字节。
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

// ReadDir 返回允许路径下的直接目录项。
func (root Root) ReadDir(path string) ([]fs.DirEntry, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(resolved)
}

// Readlink 读取允许路径的符号链接目标而不继续跟随链接。
func (root Root) Readlink(path string) (string, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return "", err
	}
	return os.Readlink(resolved)
}

// Stat 返回允许路径的元数据。
func (root Root) Stat(path string) (fs.FileInfo, error) {
	resolved, err := root.Path(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(resolved)
}
