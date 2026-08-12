package model

import (
	"encoding/json"
	"testing"
)

func TestModuleReportSelectedEmptyCollectionsAreArrays(t *testing.T) {
	t.Parallel()

	report := ModuleReport{
		SchemaName:    ModuleReportSchemaName,
		SchemaVersion: SchemaVersion,
		Module:        "socket",
		Data: ModuleData{
			Sockets:       []Socket{},
			Relationships: []Relationship{},
		},
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if string(document.Data["sockets"]) != "[]" || string(document.Data["relationships"]) != "[]" {
		t.Fatalf("data = %s, want selected empty arrays", encoded)
	}
	if _, exists := document.Data["processes"]; exists {
		t.Fatalf("data = %s, internal dependency must be omitted", encoded)
	}
}
