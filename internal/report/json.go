package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func WriteJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}

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
