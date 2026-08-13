package model

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBatchProtocolUsesVersionTwoAndNoReservedFutureDomains(t *testing.T) {
	t.Parallel()

	batch := Batch{
		SchemaName:      BatchSchemaName,
		SchemaVersion:   BatchSchemaVersion,
		ID:              "scan-1",
		Type:            BatchTypeModule,
		RequestedModule: "host",
		Results:         []ModuleResult{},
	}

	encoded, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"schema_name":"asset-agent.batch"`) ||
		!strings.Contains(text, `"schema_version":"2.0"`) {
		t.Fatalf("batch = %s", text)
	}
	for _, forbidden := range []string{"services", "packages", "containers", "files", "applications"} {
		if strings.Contains(text, `"`+forbidden+`"`) {
			t.Fatalf("batch contains future domain %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"results":[]`) {
		t.Fatalf("batch results are not an empty array: %s", text)
	}
}

func TestAssetRecordKeepsEmptyEvidenceAsArray(t *testing.T) {
	t.Parallel()

	record := AssetRecord{
		RecordID: "host:1", RecordType: "host", HostID: "host:1",
		ScopeID: "host:1", ScopeType: "host", Name: "server-1",
		Platform: "linux", Evidence: []Evidence{},
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"evidence":[]`)) {
		t.Fatalf("record = %s", encoded)
	}
}

func TestModuleResultNormalizesCollectionsAndCompleteStatus(t *testing.T) {
	t.Parallel()

	result := ModuleResult{
		Module: "host",
		Status: StatusComplete,
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range [][]byte{
		[]byte(`"status":"complete"`),
		[]byte(`"coverage":{"expected_scopes":[],"completed_scopes":[],"failed_scopes":[]}`),
		[]byte(`"errors":[]`),
		[]byte(`"records":[]`),
		[]byte(`"relationships":[]`),
	} {
		if !bytes.Contains(encoded, literal) {
			t.Fatalf("result = %s, missing %s", encoded, literal)
		}
	}
}
