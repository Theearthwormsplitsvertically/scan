package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

type fakeRuntime struct {
	doctorReport  model.DoctorReport
	doctorErr     error
	infos         []coremodule.Info
	modulesErr    error
	outcome       agent.ScanOutcome
	scanErr       error
	selectionSeen *agent.ScanSelection
}

func (runtime fakeRuntime) Doctor(context.Context) (model.DoctorReport, error) {
	return runtime.doctorReport, runtime.doctorErr
}
func (runtime fakeRuntime) Modules(context.Context) ([]coremodule.Info, error) {
	return runtime.infos, runtime.modulesErr
}
func (runtime fakeRuntime) Scan(_ context.Context, selection agent.ScanSelection) (agent.ScanOutcome, error) {
	if runtime.selectionSeen != nil {
		*runtime.selectionSeen = selection
	}
	return runtime.outcome, runtime.scanErr
}

func TestRunDynamicModuleFlagsAndFullScan(t *testing.T) {
	tests := []struct {
		name string
		args []string
		all  bool
		mods []string
	}{
		{name: "future module", args: []string{"-custom"}, mods: []string{"custom"}},
		{name: "combined", args: []string{"-network", "-host", "-network"}, mods: []string{"host", "network"}},
		{name: "full", args: []string{"scan"}, all: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seen := agent.ScanSelection{}
			root := filepath.Join(t.TempDir(), "output")
			batch := testCLIBatch()
			if test.all {
				batch.Type, batch.RequestedModule = model.BatchTypeSnapshot, "all"
			}
			var stdout, stderr bytes.Buffer
			env := environment{executablePath: func() (string, error) { return filepath.Join(t.TempDir(), "agent"), nil }, now: time.Now}
			args := append(append([]string{}, test.args...), "-output", root)
			code := runWithEnvironment(context.Background(), args, &stdout, &stderr, fakeRuntime{
				infos:   []coremodule.Info{moduleInfo("custom"), moduleInfo("host"), moduleInfo("network")},
				outcome: agent.ScanOutcome{Batch: batch}, selectionSeen: &seen,
			}, env)
			if code != 0 || seen.All != test.all || strings.Join(seen.Modules, ",") != strings.Join(test.mods, ",") {
				t.Fatalf("code=%d seen=%+v stdout=%q stderr=%q", code, seen, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunModulesWritesConsolidatedTable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	info := moduleInfo("custom")
	info.Descriptor.HardDependencies = []string{"host"}
	code := Run(context.Background(), []string{"modules"}, &stdout, &stderr, fakeRuntime{infos: []coremodule.Info{info}})
	for _, want := range []string{"MODULE", "STATUS", "INTERVAL", "RESOURCE", "TIMEOUT", "DEPENDENCIES", "custom", "supported", "host"} {
		if code != 0 || !strings.Contains(stdout.String(), want) {
			t.Fatalf("code=%d missing=%q stdout=%q stderr=%q", code, want, stdout.String(), stderr.String())
		}
	}
}

func TestRunRejectsLegacyAndInvalidCommands(t *testing.T) {
	runtime := fakeRuntime{infos: []coremodule.Info{moduleInfo("host")}}
	for _, args := range [][]string{{"host", "scan"}, {"all", "scan"}, {"scan", "host"}, {"scan", "socket"}, {"-host", "-o", "x"}, {"missing"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, &stdout, &stderr, runtime); code != 2 || strings.Contains(stderr.String(), "deprecated") {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunVersionDoctorHelpAndFatalErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr, nil); code != 0 || !strings.Contains(stdout.String(), "asset-agent") {
		t.Fatalf("version code=%d output=%q", code, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr, fakeRuntime{doctorErr: errors.New("unavailable")}); code != 1 {
		t.Fatalf("doctor code=%d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"help"}, &stdout, &stderr, fakeRuntime{infos: []coremodule.Info{moduleInfo("custom")}}); code != 0 || !strings.Contains(stdout.String(), "-custom") {
		t.Fatalf("help code=%d output=%q", code, stdout.String())
	}
}

func moduleInfo(name string) coremodule.Info {
	return coremodule.Info{Descriptor: coremodule.Descriptor{Name: name, DefaultInterval: "6h", ResourceClass: "light", Timeout: "30s"}, Support: coremodule.SupportResult{Status: model.StatusOK}}
}

func testCLIBatch() model.Batch {
	started := time.Date(2026, time.August, 13, 3, 0, 0, 0, time.UTC)
	return model.Batch{SchemaName: model.BatchSchemaName, SchemaVersion: model.BatchSchemaVersion, ID: "scan-cli", Type: model.BatchTypeModule,
		RequestedModule: "multi", Platform: "linux", StartedAt: started, FinishedAt: started.Add(time.Second), Results: []model.ModuleResult{{
			Module: "custom", SchemaVersion: model.BatchSchemaVersion, Status: model.StatusComplete, Published: true,
			Coverage: model.Coverage{}, Errors: []model.ErrorDetail{}, Records: []model.AssetRecord{}, Relationships: []model.RelationshipRecord{},
		}}}
}
