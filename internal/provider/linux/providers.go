// Package linux 将现有只读 Linux 采集器绑定到平台无关 Provider 契约。
package linux

import (
	"context"
	"runtime"

	"github.com/Theearthwormsplitsvertically/scan/internal/capability"
	collectcontainer "github.com/Theearthwormsplitsvertically/scan/internal/collect/container"
	collecthost "github.com/Theearthwormsplitsvertically/scan/internal/collect/host"
	collectnetwork "github.com/Theearthwormsplitsvertically/scan/internal/collect/network"
	collectpackages "github.com/Theearthwormsplitsvertically/scan/internal/collect/packages"
	collectprocess "github.com/Theearthwormsplitsvertically/scan/internal/collect/process"
	collectservice "github.com/Theearthwormsplitsvertically/scan/internal/collect/service"
	collectsocket "github.com/Theearthwormsplitsvertically/scan/internal/collect/socket"
	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// New 注册当前第一阶段可用的 Linux 能力。
func New(root platform.Root) (*provider.Set, error) {
	return provider.NewSet("linux",
		profileProvider{root: root},
		hostProvider{root: root},
		networkProvider{root: root},
		processProvider{root: root},
		socketProvider{root: root},
		serviceProvider{root: root},
		packageProvider{root: root},
		containerProvider{source: collectcontainer.NewDockerSource("/var/run/docker.sock")},
	)
}

type profileProvider struct {
	root platform.Root
}

func (profileProvider) Capability() string { return provider.CapabilitySystemProfile }

func (item profileProvider) Detect(ctx context.Context) model.DoctorReport {
	return capability.Detect(ctx, item.root, runtime.GOARCH)
}

type hostProvider struct {
	root platform.Root
}

func (hostProvider) Capability() string { return provider.CapabilityHost }

func (item hostProvider) Collect(ctx context.Context) (model.Host, model.CollectorStatus) {
	return collecthost.Collect(ctx, item.root)
}

type networkProvider struct {
	root platform.Root
}

func (networkProvider) Capability() string { return provider.CapabilityNetwork }

func (item networkProvider) Collect(ctx context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus) {
	return collectnetwork.Collect(ctx, item.root, collectnetwork.SystemInterfaceSource{})
}

type processProvider struct {
	root platform.Root
}

func (processProvider) Capability() string { return provider.CapabilityProcess }

func (item processProvider) Collect(ctx context.Context, bootID string) ([]model.Process, model.CollectorStatus) {
	return collectprocess.Collect(ctx, item.root, bootID)
}

type socketProvider struct {
	root platform.Root
}

func (socketProvider) Capability() string { return provider.CapabilitySocket }

func (item socketProvider) Collect(ctx context.Context, processes []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus) {
	return collectsocket.Collect(ctx, item.root, processes)
}

type serviceProvider struct {
	root platform.Root
}

func (serviceProvider) Capability() string { return provider.CapabilityService }

func (item serviceProvider) Collect(ctx context.Context) ([]model.Service, model.CollectorStatus) {
	return collectservice.Collect(ctx, item.root)
}

type packageProvider struct {
	root platform.Root
}

func (packageProvider) Capability() string { return provider.CapabilityPackage }

func (item packageProvider) Collect(ctx context.Context) ([]model.Package, model.CollectorStatus) {
	return collectpackages.Collect(ctx, item.root)
}

type containerProvider struct {
	source collectcontainer.Source
}

func (containerProvider) Capability() string { return provider.CapabilityContainer }

func (item containerProvider) Collect(ctx context.Context) ([]model.Container, []model.ContainerImage, model.CollectorStatus) {
	return collectcontainer.Collect(ctx, item.source)
}
