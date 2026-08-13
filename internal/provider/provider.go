// Package provider 定义与操作系统实现解耦的动态数据源注册表。
package provider

import (
	"fmt"
	"strings"
)

// Provider 是可注册平台能力的最小契约。
type Provider interface {
	Capability() string
}

// Lookup 是模块查询当前平台能力所需的只读视图。
type Lookup interface {
	Platform() string
	Lookup(string) (Provider, bool)
}

// Set 保存一个平台实际提供的能力，不包含平台或模块名称分支。
type Set struct {
	platform  string
	providers map[string]Provider
}

// NewSet 创建平台能力集合，并按 Provider 自己声明的名称完成注册。
func NewSet(platform string, providers ...Provider) (*Set, error) {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil, fmt.Errorf("provider platform is empty")
	}
	set := &Set{platform: platform, providers: make(map[string]Provider, len(providers))}
	for _, item := range providers {
		if err := set.Register(item); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// Platform 返回该能力集合绑定的平台名称。
func (set *Set) Platform() string {
	if set == nil {
		return ""
	}
	return set.platform
}

// Register 增加一个能力；空名称或重复名称会被拒绝。
func (set *Set) Register(item Provider) error {
	if set == nil {
		return fmt.Errorf("provider set is nil")
	}
	if item == nil {
		return fmt.Errorf("provider is nil")
	}
	capability := strings.TrimSpace(item.Capability())
	if capability == "" {
		return fmt.Errorf("provider capability is empty")
	}
	if _, exists := set.providers[capability]; exists {
		return fmt.Errorf("provider capability %q is already registered", capability)
	}
	set.providers[capability] = item
	return nil
}

// Lookup 按能力名称返回平台 Provider。
func (set *Set) Lookup(capability string) (Provider, bool) {
	if set == nil {
		return nil, false
	}
	item, ok := set.providers[capability]
	return item, ok
}

// As 查询能力并验证它是否实现调用方需要的强类型契约。
func As[T Provider](lookup Lookup, capability string) (T, bool) {
	var zero T
	if lookup == nil {
		return zero, false
	}
	item, ok := lookup.Lookup(capability)
	if !ok {
		return zero, false
	}
	typed, ok := item.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}
