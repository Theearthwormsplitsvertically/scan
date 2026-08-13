package port

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeSocketProvider struct {
	calls *int
}

func (fakeSocketProvider) Capability() string { return provider.CapabilitySocket }

func (item fakeSocketProvider) Collect(_ context.Context, _ []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
	*item.calls++
	return []model.Socket{
			{ID: "listen", Protocol: "tcp", State: "LISTEN", LocalAddress: "0.0.0.0", LocalPort: 443, PIDs: []int{10}, ProcessIDs: []string{"boot:10:1"}},
			{ID: "conn", Protocol: "tcp", State: "ESTABLISHED", LocalAddress: "10.0.0.1", LocalPort: 50000, RemoteAddress: "10.0.0.2", RemotePort: 5432, PIDs: []int{10}, ProcessIDs: []string{"boot:10:1"}},
		}, []model.Relationship{{ID: "socket-process", Type: "socket_process", FromID: "listen", ToID: "boot:10:1", Confidence: "exact"}},
		model.CollectorStatus{Collector: "socket", Status: model.StatusOK, Errors: []string{}}
}

func TestPortModulePublishesOnlyListenersAndTheirProcessOwnership(t *testing.T) {
	t.Parallel()

	calls := 0
	providers, err := provider.NewSet("linux", fakeSocketProvider{calls: &calls})
	if err != nil {
		t.Fatal(err)
	}
	processes := []model.Process{{ID: "boot:10:1", PID: 10, StartTime: 1, Name: "web"}}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"process": {
			Data: model.ModuleResult{Records: []model.AssetRecord{{
				RecordID: "process:public", RecordType: "process", HostID: "host:public",
				Attributes: map[string]any{"process_id": "boot:10:1", "pid": 10},
			}}},
			Internal: processes,
		},
	}}
	result := New().Collect(context.Background(), providers, request)
	if calls != 1 {
		t.Fatalf("socket provider calls = %d", calls)
	}
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	record := result.Data.Records[0]
	if record.RecordType != "port" || !record.States.Exposed || record.Attributes["local_port"] != 443 {
		t.Fatalf("record = %+v", record)
	}
	if len(result.Data.Relationships) != 1 {
		t.Fatalf("relationships = %+v", result.Data.Relationships)
	}
	relationship := result.Data.Relationships[0]
	if relationship.RelationshipType != "listens_on" || relationship.FromID != "process:public" || relationship.ToID != record.RecordID {
		t.Fatalf("relationship = %+v", relationship)
	}
	facts, ok := result.Internal.(Facts)
	if !ok || len(facts.Sockets) != 2 || len(facts.SocketProcessRelationships) != 1 {
		t.Fatalf("internal = %#v", result.Internal)
	}
}

func TestPortModuleListeningRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		socket model.Socket
		want   bool
	}{
		{name: "tcp listener", socket: model.Socket{Protocol: "tcp", State: "LISTEN"}, want: true},
		{name: "tcp connection", socket: model.Socket{Protocol: "tcp", State: "ESTABLISHED"}, want: false},
		{name: "udp wildcard", socket: model.Socket{Protocol: "udp", RemoteAddress: "0.0.0.0", RemotePort: 0}, want: true},
		{name: "udp ipv6 wildcard", socket: model.Socket{Protocol: "udp", RemoteAddress: "::", RemotePort: 0}, want: true},
		{name: "udp peer", socket: model.Socket{Protocol: "udp", RemoteAddress: "192.0.2.1", RemotePort: 53}, want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if got := isListening(test.socket); got != test.want {
				t.Fatalf("isListening() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPortModuleDescriptor(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.DefaultInterval != "1h" || !reflect.DeepEqual(descriptor.HardDependencies, []string{"process"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
