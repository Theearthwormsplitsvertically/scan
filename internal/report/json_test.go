package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

func TestWriteJSONProducesReadableSingleDocument(t *testing.T) {
	t.Parallel()

	value := struct {
		SchemaVersion string `json:"schema_version"`
		Detail        string `json:"detail"`
	}{SchemaVersion: model.SchemaVersion, Detail: "load < 10% & healthy"}
	var output bytes.Buffer

	if err := WriteJSON(&output, value); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if !strings.HasSuffix(output.String(), "\n") || strings.HasSuffix(output.String(), "\n\n") {
		t.Fatalf("output = %q, want exactly one trailing newline", output.String())
	}
	if strings.Contains(output.String(), `\u003c`) || strings.Contains(output.String(), `\u0026`) {
		t.Fatalf("output = %q, want readable characters", output.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if decoded["schema_version"] != "1.0" {
		t.Fatalf("schema_version = %v, want 1.0", decoded["schema_version"])
	}
}

func TestWriteJSONFilePublishesCompleteFileWithoutTemporaryArtifacts(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "snapshot.json")
	value := model.Snapshot{SchemaVersion: model.SchemaVersion}

	if err := WriteJSONFile(path, value); err != nil {
		t.Fatalf("WriteJSONFile() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Contains(content, []byte(`"schema_version":"1.0"`)) {
		t.Fatalf("content = %q, want schema", content)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "snapshot.json" {
		t.Fatalf("entries = %v, want only snapshot.json", entries)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("mode = %o, want 600", got)
		}
	}
}
