package packages

import (
	"context"
	"reflect"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

type fakePackageProvider struct {
	packages []model.Package
}

func (fakePackageProvider) Capability() string { return provider.CapabilityPackage }

func (item fakePackageProvider) Collect(_ context.Context) ([]model.Package, model.CollectorStatus) {
	return item.packages, model.CollectorStatus{Collector: "package", Status: model.StatusOK, Errors: []string{}}
}

func TestPackageModuleUsesStableIDIndependentOfVersion(t *testing.T) {
	t.Parallel()

	providers, err := provider.NewSet("linux", fakePackageProvider{packages: []model.Package{
		{Name: "nginx", Version: "1.24.0", Architecture: "amd64", Maintainer: "Ubuntu", Source: "dpkg"},
		{Name: "nginx", Version: "1.26.0", Architecture: "amd64", Maintainer: "Ubuntu", Source: "dpkg"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	host := model.Host{DMIUUID: "d00da7e4-73db-4568-a1c7-93a8681714fc"}
	request := coremodule.Request{Dependencies: map[string]coremodule.Result{"host": {Internal: host}}}

	result := New().Collect(context.Background(), providers, request)
	if result.Data.Status != model.StatusComplete || len(result.Data.Records) != 2 {
		t.Fatalf("result = %+v", result.Data)
	}
	if result.Data.Records[0].RecordID != result.Data.Records[1].RecordID {
		t.Fatalf("版本变化不应改变 record_id: %q vs %q", result.Data.Records[0].RecordID, result.Data.Records[1].RecordID)
	}
	record := result.Data.Records[0]
	if record.Version != "1.24.0" || record.Vendor != "Ubuntu" || record.RecordType != "package" {
		t.Fatalf("record = %+v", record)
	}
	if !record.States.Installed || record.States.Running || record.States.Loaded || record.States.Exposed {
		t.Fatalf("states = %+v", record.States)
	}
	want := coremodule.StableRecordID("package", hostmodule.RecordID(host), "amd64", "nginx")
	if record.RecordID != want {
		t.Fatalf("record ID = %q, want %q", record.RecordID, want)
	}
}

func TestPackageModuleDescriptorDeclaresHostDependency(t *testing.T) {
	t.Parallel()

	descriptor := New().Descriptor()
	if descriptor.Name != "package" || !reflect.DeepEqual(descriptor.HardDependencies, []string{"host"}) {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
