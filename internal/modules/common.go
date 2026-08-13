// Package modules 组装内置模块，同时保持注册表本身不知道任何模块名称。
package modules

import (
	"fmt"

	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

func registerAll(registry *coremodule.Registry, items ...coremodule.Module) error {
	for _, item := range items {
		if err := registry.Register(item); err != nil {
			return fmt.Errorf("register built-in module: %w", err)
		}
	}
	return nil
}
