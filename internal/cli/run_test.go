package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

type fakeRuntime struct {
	doctorReport model.DoctorReport
	doctorErr    error
	infos        []coremodule.Info
	modulesErr   error
	batch        model.Batch
	scanErr      error
	targetSeen   *string
}

func (runtime fakeRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return runtime.doctorReport, runtime.doctorErr
}

func (runtime fakeRuntime) Modules(context.Context) ([]coremodule.Info, error) {
	return runtime.infos, runtime.modulesErr
}

func (runtime fakeRuntime) ScanTarget(_ context.Context, target string) (model.Batch, error) {
	if runtime.targetSeen != nil {
		*runtime.targetSeen = target
	}
	batch := runtime.batch
	batch.RequestedModule = target
	if target == "all" {
		batch.Type = model.BatchTypeSnapshot
	} else {
		batch.Type = model.BatchTypeModule
	}
	return batch, runtime.scanErr
}

func TestRunVersionWritesMachineReadableVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, &stdout, &stderr, nil)
	if code != 0 || !strings.Contains(stdout.String(), `"name":"asset-agent"`) || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDoctorWritesRuntimeReportAndError(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr, fakeRuntime{
			doctorReport: model.DoctorReport{SchemaVersion: model.SchemaVersion, OS: "linux", Kernel: "6.12.0"},
		})
		if code != 0 || !strings.Contains(stdout.String(), `"kernel":"6.12.0"`) {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
	t.Run("failure", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr, fakeRuntime{doctorErr: errors.New("procfs unavailable")})
		if code != 1 || !strings.Contains(stderr.String(), "procfs unavailable") {
			t.Fatalf("code=%d stderr=%q", code, stderr.String())
		}
	})
}

func TestRunModuleFirstScanUsesRegisteredModule(t *testing.T) {
	t.Parallel()

	seen := ""
	runtime := fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}, targetSeen: &seen, batch: testCLIBatch()}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"custom", "scan", "-o", "-"}, &stdout, &stderr, runtime)
	if code != 0 || seen != "custom" || !strings.Contains(stdout.String(), `"schema_name":"asset-agent.batch"`) {
		t.Fatalf("code=%d target=%q stdout=%q stderr=%q", code, seen, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("formal command emitted migration warning: %q", stderr.String())
	}
}

func TestRunModulesDescribeStatusAndScheduleUseRegistryInfo(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}}
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"modules"}, want: `"name":"custom"`},
		{args: []string{"custom", "describe"}, want: `"default_interval":"6h"`},
		{args: []string{"custom", "status"}, want: `"status":"ok"`},
		{args: []string{"custom", "schedule"}, want: `"resource_class":"light"`},
	}
	for _, test := range tests {
		test := test
		t.Run(strings.Join(test.args, "-"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), test.args, &stdout, &stderr, runtime)
			if code != 0 || !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunHelpListsOnlyRegisteredModules(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"help"}, &stdout, &stderr, fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}})
	if code != 0 || !strings.Contains(stdout.String(), "custom") || strings.Contains(stdout.String(), "host") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunLegacyScanAliasesAndMigrationWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantTarget string
	}{
		{name: "bare scan", args: []string{"scan", "--output-dir"}, wantTarget: "all"},
		{name: "scan all", args: []string{"scan", "all", "--output-dir"}, wantTarget: "all"},
		{name: "scan host", args: []string{"scan", "host", "-o", "-"}, wantTarget: "host"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			seen := ""
			runtime := fakeRuntime{infos: []coremodule.Info{moduleInfo("host")}, targetSeen: &seen, batch: testCLIBatch()}
			args := append([]string{}, test.args...)
			if args[len(args)-1] == "--output-dir" {
				args = append(args, filepath.Join(t.TempDir(), "output"))
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr, runtime)
			if code != 0 || seen != test.wantTarget || !strings.Contains(stderr.String(), "deprecated") {
				t.Fatalf("code=%d target=%q stdout=%q stderr=%q", code, seen, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunLegacySocketRequiresExplicitPortConnectionMigration(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"scan", "socket"}, &stdout, &stderr, fakeRuntime{infos: []coremodule.Info{moduleInfo("port"), moduleInfo("connection")}})
	if code != 2 || !strings.Contains(stderr.String(), "port") || !strings.Contains(stderr.String(), "connection") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunScanOutputModes(t *testing.T) {
	t.Parallel()

	t.Run("module output directory", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "output")
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"custom", "scan", "--output-dir", root}, &stdout, &stderr,
			fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}, batch: testCLIBatch()})
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr.String())
		}
		published := strings.TrimSpace(stdout.String())
		if !strings.HasPrefix(published, filepath.Join(root, "inbox")) {
			t.Fatalf("published=%q", published)
		}
		if _, err := os.Stat(filepath.Join(published, "manifest.json")); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("module output file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "custom.json")
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"custom", "scan", "-o", path}, &stdout, &stderr,
			fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}, batch: testCLIBatch()})
		if code != 0 || strings.TrimSpace(stdout.String()) != path {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunScanRejectsInvalidOutputCombinations(t *testing.T) {
	t.Parallel()

	runtime := fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}, batch: testCLIBatch()}
	tests := [][]string{
		{"all", "scan", "-o", "-"},
		{"all", "scan", "-o", filepath.Join(t.TempDir(), "all.json")},
		{"custom", "scan", "--output-dir", t.TempDir(), "-o", "-"},
		{"custom", "scan", "--output-dir"},
		{"custom", "scan", "-o"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, &stdout, &stderr, runtime); code != 2 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunScanDefaultsToBatchOutputBesideExecutable(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	executable := filepath.Join(installDir, "asset-agent")
	if err := os.WriteFile(executable, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	env := environment{executablePath: func() (string, error) { return executable, nil }, now: time.Now}
	code := runWithEnvironment(context.Background(), []string{"custom", "scan"}, &stdout, &stderr,
		fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}, batch: testCLIBatch()}, env)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), filepath.Join(installDir, "output", "inbox")) {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunRejectsUnknownModuleActionAndUnavailableRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args    []string
		runtime agentRuntimeForTest
		code    int
	}{
		{args: []string{"missing", "scan"}, runtime: fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}}, code: 2},
		{args: []string{"custom", "destroy"}, runtime: fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}}, code: 2},
		{args: []string{"custom", "scan"}, runtime: nil, code: 1},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), test.args, &stdout, &stderr, test.runtime); code != test.code {
			t.Fatalf("args=%v code=%d stderr=%q", test.args, code, stderr.String())
		}
	}
}

type agentRuntimeForTest interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Modules(context.Context) ([]coremodule.Info, error)
	ScanTarget(context.Context, string) (model.Batch, error)
}

func moduleInfo(name string) coremodule.Info {
	return coremodule.Info{
		Descriptor: coremodule.Descriptor{
			Name: name, SchemaVersion: model.BatchSchemaVersion, RecordTypes: []string{name},
			Commands: coremodule.StandardCommands(), DefaultInterval: "6h", ResourceClass: "light", Timeout: "30s",
		},
		Support: coremodule.SupportResult{Status: model.StatusOK, Errors: []model.ErrorDetail{}},
	}
}

func testCLIBatch() model.Batch {
	started := time.Date(2026, time.August, 13, 3, 0, 0, 0, time.UTC)
	return model.Batch{
		SchemaName: model.BatchSchemaName, SchemaVersion: model.BatchSchemaVersion,
		ID: "scan-cli", Platform: "linux", Agent: model.AgentInfo{Name: "asset-agent"},
		StartedAt: started, FinishedAt: started.Add(time.Second),
		Results: []model.ModuleResult{{
			Module: "custom", SchemaVersion: model.BatchSchemaVersion, Status: model.StatusComplete,
			Authoritative: true, Coverage: model.Coverage{ExpectedScopes: []string{"host"}, CompletedScopes: []string{"host"}, FailedScopes: []string{}},
			Errors: []model.ErrorDetail{}, Records: []model.AssetRecord{{
				RecordID: "custom:1", RecordType: "custom", HostID: "host:1", ScopeID: "host:1", ScopeType: "host",
				Name: "custom", Platform: "linux", Evidence: []model.Evidence{},
			}}, Relationships: []model.RelationshipRecord{}, Published: true,
		}},
	}
}
