// report 包将 Agent 结果序列化为本地 JSON 文档。
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteJSON 写出一个以单个换行结束的可读 JSON 文档。
// 关闭 HTML 转义，使运维人员看到的路径和文本保持可读。
func WriteJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}

// WriteJSONFile 将 JSON 报告原子发布到 path。
// 它先写入同目录的 0600 临时文件，完成同步后再重命名。
func WriteJSONFile(path string, value any) (resultErr error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".tmp-asset-agent-*")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		if !published {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set report permissions: %w", err)
	}
	if err := WriteJSON(temporary, value); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush report: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close report: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish report: %w", err)
	}
	published = true
	return nil
}
