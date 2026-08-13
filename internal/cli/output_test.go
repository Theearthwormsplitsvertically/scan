package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

func TestWriteScanSummaryShowsModulesStatusesCountsAndOutput(t *testing.T) {
	outcome := agent.ScanOutcome{
		Batch: model.Batch{Type: model.BatchTypeModule, Results: []model.ModuleResult{
			{Module: "host", Status: model.StatusComplete, DurationMS: 8, Errors: []model.ErrorDetail{}},
			{Module: "network", Status: model.StatusPartial, DurationMS: 23, Errors: []model.ErrorDetail{{Code: "collection_error", Message: "route source unavailable"}}},
		}},
		RecordCounts: map[string]int{"host": 1, "network": 6},
	}
	var output bytes.Buffer
	if err := writeScanSummary(&output, outcome, []string{"host", "network"}, "/data/cmdb/inbox/module-multi-test"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Asset Agent Scan", "Modules: host, network", "Status: partial", "host", "complete", "1", "8ms", "network", "6", "23ms", "route source unavailable", "Output: /data/cmdb/inbox/module-multi-test"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("summary missing %q:\n%s", want, output.String())
		}
	}
}

func TestWriteScanSummaryCompressesLongMultilineErrors(t *testing.T) {
	message := strings.Repeat("x", 130) + "\nsecond line"
	outcome := agent.ScanOutcome{Batch: model.Batch{Type: model.BatchTypeSnapshot, Results: []model.ModuleResult{{
		Module: "host", Status: model.StatusFailed, Errors: []model.ErrorDetail{{Message: message}},
	}}}, RecordCounts: map[string]int{"host": 0}}
	var output bytes.Buffer
	if err := writeScanSummary(&output, outcome, nil, "/tmp/out"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\nsecond line") || !strings.Contains(output.String(), "Modules: all") || !strings.Contains(output.String(), "Status: failed") {
		t.Fatalf("summary = %q", output.String())
	}
}
