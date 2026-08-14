// Package module 定义扫描模块契约和动态注册边界。
package module

import (
	"context"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// Descriptor 声明模块身份、能力依赖、周期和资源预算。
type Descriptor struct {
	Name                 string   `json:"name"`
	SchemaVersion        string   `json:"schema_version"`
	RecordTypes          []string `json:"record_types"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OptionalCapabilities []string `json:"optional_capabilities"`
	HardDependencies     []string `json:"hard_dependencies"`
	SoftDependencies     []string `json:"soft_dependencies"`
	DefaultInterval      string   `json:"default_interval"`
	ResourceClass        string   `json:"resource_class"`
	Timeout              string   `json:"timeout"`
}

// SupportResult 是模块对当前平台 Provider 集合的能力判断。
type SupportResult struct {
	Status model.Status        `json:"status"`
	Reason string              `json:"reason,omitempty"`
	Errors []model.ErrorDetail `json:"errors"`
}

// Info 汇总模块静态描述和当前平台支持状态。
type Info struct {
	Descriptor Descriptor    `json:"descriptor"`
	Support    SupportResult `json:"support"`
}

// Request 向模块提供已完成的依赖结果；依赖数据不会自动重复发布。
type Request struct {
	Dependencies map[string]Result
}

// Result 同时携带公开协议数据和供后续依赖复用的内部强类型事实。
type Result struct {
	Data     model.ModuleResult
	Internal any
}

// Module 是所有内置扫描模块必须实现的边界。
type Module interface {
	Descriptor() Descriptor
	Probe(context.Context, provider.Lookup) SupportResult
	Collect(context.Context, provider.Lookup, Request) Result
}
