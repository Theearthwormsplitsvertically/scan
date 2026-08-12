package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

type fakeRuntime struct {
	doctorReport model.DoctorReport
	doctorErr    error
	snapshot     model.Snapshot
	scanErr      error
}

func (f fakeRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return f.doctorReport, f.doctorErr
}

func (f fakeRuntime) Scan(context.Context) (model.Snapshot, error) {
	return f.snapshot, f.scanErr
}

func TestRunVersionWritesMachineReadableVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"version"}, &stdout, &stderr, nil)

	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), `"name":"asset-agent"`) {
		t.Fatalf("stdout = %q, want JSON asset-agent name", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"destroy"}, &stdout, &stderr, nil)

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command error", stderr.String())
	}
}

func TestRunDoctorWritesRuntimeReport(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{doctorReport: model.DoctorReport{
		SchemaVersion: model.SchemaVersion,
		OS:            "linux",
		Kernel:        "6.12.0",
	}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kernel":"6.12.0"`) {
		t.Fatalf("stdout = %q, want doctor report", stdout.String())
	}
}

func TestRunScanWritesRuntimeSnapshot(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{snapshot: model.Snapshot{SchemaVersion: model.SchemaVersion}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan"}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":"1.0"`) {
		t.Fatalf("stdout = %q, want snapshot", stdout.String())
	}
}

func TestRunCollectionErrorReturnsOne(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{doctorErr: errors.New("procfs unavailable")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr, runtime)

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "procfs unavailable") {
		t.Fatalf("stderr = %q, want runtime error", stderr.String())
	}
}

func TestRunCollectionWithoutRuntimeFailsSafely(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan"}, &stdout, &stderr, nil)

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "runtime unavailable") {
		t.Fatalf("stderr = %q, want runtime unavailable", stderr.String())
	}
}

func TestRunWatchIsExplicitlyReserved(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"watch"}, &stdout, &stderr, fakeRuntime{})

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "not implemented in this milestone") {
		t.Fatalf("stderr = %q, want milestone message", stderr.String())
	}
}

func TestRunScanOutputDashWritesStdout(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{snapshot: model.Snapshot{SchemaVersion: model.SchemaVersion}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan", "--output", "-"}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":"1.0"`) {
		t.Fatalf("stdout = %q, want snapshot", stdout.String())
	}
}

func TestRunScanOutputPathWritesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "snapshot.json")
	runtime := fakeRuntime{snapshot: model.Snapshot{SchemaVersion: model.SchemaVersion}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan", "--output", path}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Contains(content, []byte(`"schema_version":"1.0"`)) {
		t.Fatalf("content = %q, want snapshot", content)
	}
}

func TestRunScanRejectsMissingOutputValue(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan", "--output"}, &stdout, &stderr, fakeRuntime{})

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "requires a path") {
		t.Fatalf("stderr = %q, want missing path error", stderr.String())
	}
}

func TestRunScanRejectsUnknownOption(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan", "--upload"}, &stdout, &stderr, fakeRuntime{})

	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown scan option") {
		t.Fatalf("stderr = %q, want unknown option error", stderr.String())
	}
}

func TestRunScanSerializesAllCollectionsAsArrays(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{snapshot: model.Snapshot{SchemaVersion: model.SchemaVersion}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"scan"}, &stdout, &stderr, runtime)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, field := range []string{"network_interfaces", "addresses", "routes", "processes", "sockets", "services", "packages", "containers", "files", "applications", "relationships", "collector_status"} {
		if got := string(document[field]); got != "[]" {
			t.Fatalf("%s = %s, want []", field, got)
		}
	}
}
