//go:build !linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultOutputRoot 为尚未定义系统目录的平台返回可执行文件同级 output。
func DefaultOutputRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位可执行文件: %w", err)
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("解析可执行文件: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("解析可执行文件符号链接: %w", err)
	}
	return filepath.Join(filepath.Dir(resolved), "output"), nil
}
