package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultOutputPath 在可执行文件同级创建私有 output 目录并返回防覆盖报告路径。
func DefaultOutputPath(executablePath, module string, now time.Time) (string, error) {
	if executablePath == "" {
		return "", fmt.Errorf("resolve executable path: empty path")
	}
	absolute, err := filepath.Abs(executablePath)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}
	outputDir := filepath.Join(filepath.Dir(resolved), "output")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	if err := os.Chmod(outputDir, 0o700); err != nil {
		return "", fmt.Errorf("set output directory permissions: %w", err)
	}

	base := fmt.Sprintf("%s-%s", module, now.UTC().Format("20060102T150405Z"))
	for index := 0; ; index++ {
		name := base + ".json"
		if index > 0 {
			name = fmt.Sprintf("%s-%d.json", base, index)
		}
		candidate := filepath.Join(outputDir, name)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect output path: %w", err)
		}
	}
}
