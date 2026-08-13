package provider

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
)

const (
	CapabilitySystemProfile = "system_profile"
	CapabilityHost          = "host"
	CapabilityNetwork       = "network"
	CapabilityProcess       = "process"
	CapabilitySocket        = "socket"
)

// ProfileProvider 检测当前平台及可用数据源。
type ProfileProvider interface {
	Provider
	Detect(context.Context) model.DoctorReport
}

// HostProvider 采集主机身份、系统和基础硬件事实。
type HostProvider interface {
	Provider
	Collect(context.Context) (model.Host, model.CollectorStatus)
}

// NetworkProvider 采集网络接口、地址和路由事实。
type NetworkProvider interface {
	Provider
	Collect(context.Context) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus)
}

// ProcessProvider 以启动 ID 为作用域采集进程事实。
type ProcessProvider interface {
	Provider
	Collect(context.Context, string) ([]model.Process, model.CollectorStatus)
}

// SocketProvider 采集 socket，并将其关联到已采集进程。
type SocketProvider interface {
	Provider
	Collect(context.Context, []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus)
}
