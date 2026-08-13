package network

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeNetworkProvider struct{}

func (fakeNetworkProvider) Capability() string { return provider.CapabilityNetwork }

func (fakeNetworkProvider) Collect(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
	return []model.NetworkInterface{{Index: 2, Name: "eth0", MAC: "00:11:22:33:44:55", Flags: []string{"up"}}},
		[]model.Address{{InterfaceIndex: 2, InterfaceName: "eth0", CIDR: "192.0.2.10/24", Family: 4}},
		[]model.Route{{Interface: "eth0", Destination: "0.0.0.0/0", Gateway: "192.0.2.1", Family: 4}},
		model.CollectorStatus{Collector: "network", Status: model.StatusOK, Errors: []string{}}
}

func TestNetworkModuleProducesStableUnifiedRecords(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux", fakeNetworkProvider{})
	if err != nil {
		t.Fatal(err)
	}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host": {Internal: model.Host{ID: "machine:dmi", Hostname: "server-1"}},
	}}
	item := New()
	first := item.Collect(context.Background(), providers, request)
	second := item.Collect(context.Background(), providers, request)
	if first.Data.Status != model.StatusComplete {
		t.Fatalf("status = %q, errors = %+v", first.Data.Status, first.Data.Errors)
	}
	if len(first.Data.Records) != 3 {
		t.Fatalf("records = %+v", first.Data.Records)
	}
	wantTypes := []string{"network_interface", "address", "route"}
	gotTypes := make([]string, 0, len(first.Data.Records))
	firstIDs := make([]string, 0, len(first.Data.Records))
	secondIDs := make([]string, 0, len(second.Data.Records))
	for index, record := range first.Data.Records {
		gotTypes = append(gotTypes, record.RecordType)
		firstIDs = append(firstIDs, record.RecordID)
		secondIDs = append(secondIDs, second.Data.Records[index].RecordID)
		if record.HostID == "" || record.ScopeID != record.HostID {
			t.Errorf("record identity = %+v", record)
		}
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("record types = %v", gotTypes)
	}
	if !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Fatalf("record IDs changed: %v != %v", firstIDs, secondIDs)
	}
}

func TestNetworkModuleDescriptorDeclaresHostDependencyAndSchedule(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.Name != "network" || descriptor.DefaultInterval != "6h" ||
		!reflect.DeepEqual(descriptor.HardDependencies, []string{"host"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
