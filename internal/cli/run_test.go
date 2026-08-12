package cli

import (
	"bytes"
	"context"
	"errors"
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
