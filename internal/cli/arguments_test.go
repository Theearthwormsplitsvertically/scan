package cli

import (
	"reflect"
	"strings"
	"testing"

	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

func TestParseScanInvocationUsesRegisteredModuleFlags(t *testing.T) {
	infos := []coremodule.Info{moduleInfo("host"), moduleInfo("network"), moduleInfo("custom")}
	got, err := parseScanInvocation([]string{"-network", "-host", "-network", "-output", "/data/cmdb"}, infos)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.selected, []string{"host", "network"}) {
		t.Fatalf("selected = %v", got.selected)
	}
	if got.outputRoot != "/data/cmdb" || got.selection.All {
		t.Fatalf("invocation = %+v", got)
	}
}

func TestParseScanInvocationDiscoversFutureModule(t *testing.T) {
	got, err := parseScanInvocation([]string{"-service"}, []coremodule.Info{moduleInfo("service")})
	if err != nil || !reflect.DeepEqual(got.selection.Modules, []string{"service"}) {
		t.Fatalf("invocation=%+v err=%v", got, err)
	}
}

func TestParseScanInvocationAcceptsFullScanOutput(t *testing.T) {
	got, err := parseScanInvocation([]string{"scan", "-output", "/data/cmdb"}, []coremodule.Info{moduleInfo("host")})
	if err != nil || !got.selection.All || got.outputRoot != "/data/cmdb" {
		t.Fatalf("invocation=%+v err=%v", got, err)
	}
}

func TestParseScanInvocationRejectsInvalidAndLegacySyntax(t *testing.T) {
	infos := []coremodule.Info{moduleInfo("custom"), moduleInfo("host"), moduleInfo("network")}
	tests := [][]string{
		{}, {"-docker"}, {"-output"}, {"-host", "-output", "/a", "-output", "/b"},
		{"scan", "-host"}, {"scan", "host"}, {"scan", "all"}, {"scan", "socket"},
		{"host", "scan"}, {"host", "describe"}, {"host", "status"}, {"host", "schedule"},
		{"all", "scan"}, {"-host", "-o", "x"}, {"-host", "--output", "x"},
		{"-host", "--output-dir", "x"}, {"-host", "extra"},
	}
	for _, args := range tests {
		if _, err := parseScanInvocation(args, infos); err == nil {
			t.Fatalf("args %v accepted", args)
		}
	}
}

func TestParseScanInvocationUnknownFlagListsDynamicOptions(t *testing.T) {
	_, err := parseScanInvocation([]string{"-docker"}, []coremodule.Info{moduleInfo("custom"), moduleInfo("host"), moduleInfo("network")})
	if err == nil {
		t.Fatal("unknown flag accepted")
	}
	for _, want := range []string{"-custom", "-host", "-network"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}
