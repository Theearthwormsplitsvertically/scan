package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakeServiceProvider struct {
	services []model.Service
}

func (fakeServiceProvider) Capability() string { return provider.CapabilityService }

func (item fakeServiceProvider) Collect(_ context.Context) ([]model.Service, model.CollectorStatus) {
	return item.services, model.CollectorStatus{Collector: "service", Status: model.StatusOK, Errors: []string{}}
}

const testDMIUUID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

func TestServiceModulePublishesStaticFactsWithCgroupRunningState(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux", fakeServiceProvider{services: []model.Service{
		{UnitName: "nginx", Description: "Web", ExecStart: "/usr/sbin/nginx", LoadState: "loaded"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	host := model.Host{Hostname: "server-1", DMIUUID: testDMIUUID}
	processes := []model.Process{
		{ID: "boot:10:20", PID: 10, Cgroups: []string{"0::/system.slice/nginx.service"}},
		{ID: "boot:11:20", PID: 11, Cgroups: []string{"0::/system.slice/other.service"}},
	}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host":    {Internal: host},
		"process": {Internal: processes},
	}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	record := result.Data.Records[0]
	if record.RecordType != "service" || record.Name != "nginx.service" || !record.States.Running || !record.States.Installed || !record.States.Loaded {
		t.Fatalf("record = %+v", record)
	}
	if _, exists := record.Attributes["main_pid"]; exists {
		t.Fatalf("main_pid should not be published: %+v", record.Attributes)
	}
	if record.Attributes["service_manager"] != "systemd" || record.Attributes["native_service_key"] != "nginx" {
		t.Fatalf("attributes = %+v", record.Attributes)
	}
	if len(result.Data.Relationships) != 0 {
		t.Fatalf("relationships should be empty without MainPID: %+v", result.Data.Relationships)
	}
}

func TestServiceModuleRunsWithoutProcessSoftDependency(t *testing.T) {
	t.Parallel()

	providers, _ := provider.NewSet("linux", fakeServiceProvider{services: []model.Service{
		{UnitName: "nginx", ExecStart: "/usr/sbin/nginx"},
	}})
	host := model.Host{Hostname: "server-1", DMIUUID: testDMIUUID}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{
		"host": {Internal: host},
	}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 1 {
		t.Fatalf("result = %+v", result.Data)
	}
	if result.Data.Records[0].States.Running {
		t.Fatal("service marked running without process dependency")
	}
}

func TestServiceModuleDescriptorDeclaresHostHardAndProcessSoft(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.Name != "service" ||
		!reflect.DeepEqual(descriptor.HardDependencies, []string{"host"}) ||
		!reflect.DeepEqual(descriptor.SoftDependencies, []string{"process"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
