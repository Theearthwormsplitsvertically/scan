package module

import (
	"fmt"
	"sort"
	"strings"
)

var reservedModuleNames = map[string]struct{}{
	"all": {}, "output": {}, "help": {}, "version": {},
	"doctor": {}, "modules": {}, "scan": {},
}

// Registry 按模块自己的描述符动态保存模块。
type Registry struct {
	modules map[string]Module
}

// NewRegistry 创建空模块注册表；all 是查询计划的虚拟目标，不是模块。
func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]Module)}
}

// Register 注册模块并拒绝空名称、保留名称和重复名称。
func (registry *Registry) Register(item Module) error {
	if registry == nil {
		return fmt.Errorf("module registry is nil")
	}
	if item == nil {
		return fmt.Errorf("module is nil")
	}
	descriptor := item.Descriptor()
	name := strings.TrimSpace(descriptor.Name)
	if name == "" {
		return fmt.Errorf("module name is empty")
	}
	if name != descriptor.Name {
		return fmt.Errorf("module name %q contains surrounding whitespace", descriptor.Name)
	}
	if _, reserved := reservedModuleNames[name]; reserved {
		return fmt.Errorf("module name %q is reserved", name)
	}
	if registry.modules == nil {
		registry.modules = make(map[string]Module)
	}
	if _, exists := registry.modules[name]; exists {
		return fmt.Errorf("module %q is already registered", name)
	}
	registry.modules[name] = item
	return nil
}

// Lookup 按名称查询真实模块；虚拟 all 不会返回模块。
func (registry *Registry) Lookup(name string) (Module, bool) {
	if registry == nil {
		return nil, false
	}
	item, ok := registry.modules[name]
	return item, ok
}

// List 按名称返回全部模块描述符。
func (registry *Registry) List() []Descriptor {
	if registry == nil {
		return []Descriptor{}
	}
	names := make([]string, 0, len(registry.modules))
	for name := range registry.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	descriptors := make([]Descriptor, 0, len(names))
	for _, name := range names {
		descriptors = append(descriptors, registry.modules[name].Descriptor())
	}
	return descriptors
}

// PlanSelected 合并多个目标的硬、软依赖闭包，并返回依赖优先、同层名称有序的执行计划。
func (registry *Registry) PlanSelected(names []string) ([]Module, error) {
	if registry == nil {
		return nil, fmt.Errorf("module registry is nil")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no modules selected")
	}
	selected := make(map[string]bool)
	for _, name := range names {
		if _, ok := registry.modules[name]; !ok {
			return nil, fmt.Errorf("module %q is not registered", name)
		}
		if err := registry.selectDependencies(name, selected); err != nil {
			return nil, err
		}
	}
	return registry.plan(selected)
}

// PlanAll 返回当前注册表全部模块的动态执行计划。
func (registry *Registry) PlanAll() ([]Module, error) {
	if registry == nil {
		return nil, fmt.Errorf("module registry is nil")
	}
	selected := make(map[string]bool, len(registry.modules))
	for name := range registry.modules {
		selected[name] = true
	}
	return registry.plan(selected)
}

func (registry *Registry) plan(selected map[string]bool) ([]Module, error) {
	indegree := make(map[string]int, len(selected))
	dependents := make(map[string][]string, len(selected))
	for name := range selected {
		item := registry.modules[name]
		descriptor := item.Descriptor()
		if err := registry.validateDependencies(name, "hard", descriptor.HardDependencies); err != nil {
			return nil, err
		}
		if err := registry.validateDependencies(name, "soft", descriptor.SoftDependencies); err != nil {
			return nil, err
		}
		seenDependencies := make(map[string]bool)
		for _, dependency := range allDependencies(descriptor) {
			if !selected[dependency] || seenDependencies[dependency] {
				continue
			}
			seenDependencies[dependency] = true
			indegree[name]++
			dependents[dependency] = append(dependents[dependency], name)
		}
	}

	ready := make([]string, 0, len(selected))
	for name := range selected {
		if indegree[name] == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	result := make([]Module, 0, len(selected))
	for len(ready) > 0 {
		next := make([]string, 0)
		for _, name := range ready {
			result = append(result, registry.modules[name])
			for _, dependent := range dependents[name] {
				indegree[dependent]--
				if indegree[dependent] == 0 {
					next = append(next, dependent)
				}
			}
		}
		sort.Strings(next)
		ready = next
	}
	if len(result) != len(selected) {
		return nil, fmt.Errorf("module dependency cycle detected")
	}
	return result, nil
}

func (registry *Registry) selectDependencies(name string, selected map[string]bool) error {
	if selected[name] {
		return nil
	}
	selected[name] = true
	descriptor := registry.modules[name].Descriptor()
	if err := registry.validateDependencies(name, "hard", descriptor.HardDependencies); err != nil {
		return err
	}
	if err := registry.validateDependencies(name, "soft", descriptor.SoftDependencies); err != nil {
		return err
	}
	for _, dependency := range allDependencies(descriptor) {
		if err := registry.selectDependencies(dependency, selected); err != nil {
			return err
		}
	}
	return nil
}

func (registry *Registry) validateDependencies(moduleName, kind string, dependencies []string) error {
	for _, dependency := range dependencies {
		if _, ok := registry.modules[dependency]; !ok {
			return fmt.Errorf("module %q has unknown %s dependency %q", moduleName, kind, dependency)
		}
	}
	return nil
}

func allDependencies(descriptor Descriptor) []string {
	result := make([]string, 0, len(descriptor.HardDependencies)+len(descriptor.SoftDependencies))
	result = append(result, descriptor.HardDependencies...)
	result = append(result, descriptor.SoftDependencies...)
	return result
}
