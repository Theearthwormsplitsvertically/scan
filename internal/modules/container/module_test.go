package container

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeContainerProvider struct {
	containers []model.Container
	images     []model.ContainerImage
	status     model.CollectorStatus
}

func (fakeContainerProvider) Capability() string { return provider.CapabilityContainer }

func (item fakeContainerProvider) Collect(context.Context) ([]model.Container, []model.ContainerImage, model.CollectorStatus) {
	return item.containers, item.images, item.status
}

const testDMIUUID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
const testContainerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestContainerModulePublishesRecordsAndRelationships(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux", fakeContainerProvider{
		containers: []model.Container{{
			ID: testContainerID, Name: "nginx", ImageID: "sha256:img1", ImageName: "nginx", ImageTag: "latest",
			State: "running", Ports: []model.ContainerPort{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
		}},
		images: []model.ContainerImage{{ID: "sha256:img1", RepoTags: []string{"nginx:latest"}}},
		status: model.CollectorStatus{Collector: "container", Status: model.StatusOK, Errors: []string{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	host := model.Host{Hostname: "server-1", DMIUUID: testDMIUUID}
	processes := []model.Process{
		{ID: "boot:1:1", PID: 1, Cgroups: []string{"0::/system.slice/docker-" + testContainerID + ".scope"}},
		{ID: "boot:2:1", PID: 2, Cgroups: []string{"0::/system.slice/other.scope"}},
	}
	processRecords := model.ModuleResult{Records: []model.AssetRecord{
		{RecordID: "process:1", RecordType: "process", HostID: "host:id", Attributes: map[string]any{"pid": 1}},
		{RecordID: "process:2", RecordType: "process", HostID: "host:id", Attributes: map[string]any{"pid": 2}},
	}}
	portRecords := model.ModuleResult{Records: []model.AssetRecord{
		{RecordID: "port:8080", RecordType: "port", Attributes: map[string]any{"local_port": 8080}},
	}}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host":    {Internal: host},
		"process": {Internal: processes, Data: processRecords},
		"port":    {Data: portRecords},
	}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 2 {
		t.Fatalf("result = %+v", result.Data)
	}
	relTypes := map[string]int{}
	for _, relationship := range result.Data.Relationships {
		relTypes[relationship.RelationshipType]++
	}
	if relTypes["based_on"] != 1 || relTypes["runs_on"] != 1 || relTypes["runs_in"] != 1 {
		t.Fatalf("relationship types = %v", relTypes)
	}
	var containerRecord model.AssetRecord
	for _, record := range result.Data.Records {
		if record.RecordType == "container" {
			containerRecord = record
		}
	}
	listening, ok := containerRecord.Attributes["listening_host_ports"].([]int)
	if !ok || len(listening) != 1 || listening[0] != 8080 {
		t.Fatalf("listening_host_ports = %#v", containerRecord.Attributes["listening_host_ports"])
	}
}

func TestContainerModuleRunsWithoutSoftDependencies(t *testing.T) {
	t.Parallel()

	providers, _ := provider.NewSet("linux", fakeContainerProvider{
		containers: []model.Container{{ID: "abc", Name: "nginx", State: "running"}},
		status:     model.CollectorStatus{Collector: "container", Status: model.StatusOK, Errors: []string{}},
	})
	host := model.Host{Hostname: "server-1", DMIUUID: testDMIUUID}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host": {Internal: host},
	}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	if len(result.Data.Relationships) != 1 || result.Data.Relationships[0].RelationshipType != "runs_on" {
		t.Fatalf("relationships = %+v, want single runs_on", result.Data.Relationships)
	}
	if _, exists := result.Data.Records[0].Attributes["listening_host_ports"]; exists {
		t.Fatal("listening_host_ports should be absent without port dependency")
	}
}

func TestContainerModuleDescriptorDeclaresHostHardAndProcessPortSoft(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.Name != "container" ||
		!reflect.DeepEqual(descriptor.HardDependencies, []string{"host"}) ||
		!reflect.DeepEqual(descriptor.SoftDependencies, []string{"process", "port"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
