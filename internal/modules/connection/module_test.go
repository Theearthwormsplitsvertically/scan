package connection

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	portmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/port"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

func TestConnectionModuleReusesPortFactsWithoutSocketProvider(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux")
	if err != nil {
		t.Fatal(err)
	}
	facts := portmodule.Facts{
		Sockets: []model.Socket{
			{ID: "listen", Protocol: "tcp", State: "LISTEN", LocalAddress: "0.0.0.0", LocalPort: 443, PIDs: []int{10}},
			{ID: "conn", Protocol: "tcp", State: "ESTABLISHED", LocalAddress: "10.0.0.1", LocalPort: 50000, RemoteAddress: "10.0.0.2", RemotePort: 5432, PIDs: []int{10}, ProcessIDs: []string{"boot:10:1"}},
		},
		Status:           model.CollectorStatus{Collector: "socket", Status: model.StatusOK, Errors: []string{}},
		HostID:           "host:public",
		ProcessRecordIDs: map[string]string{"boot:10:1": "process:public"},
		PIDRecordIDs:     map[int]string{10: "process:public"},
	}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"port": {Data: model.ModuleResult{Status: model.StatusComplete}, Internal: facts},
	}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	record := result.Data.Records[0]
	if record.RecordType != "connection" || record.Attributes["state"] != "ESTABLISHED" {
		t.Fatalf("record = %+v", record)
	}
	if len(result.Data.Relationships) != 1 {
		t.Fatalf("relationships = %+v", result.Data.Relationships)
	}
	relationship := result.Data.Relationships[0]
	if relationship.RelationshipType != "connects_to" || relationship.FromID != "process:public" || relationship.ToID != record.RecordID {
		t.Fatalf("relationship = %+v", relationship)
	}
}

func TestConnectionModuleDescriptor(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.DefaultInterval != "1h" || !reflect.DeepEqual(descriptor.HardDependencies, []string{"port"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
