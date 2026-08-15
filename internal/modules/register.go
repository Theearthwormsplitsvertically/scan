package modules

import (
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	connectionmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/connection"
	hostmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/host"
	networkmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/network"
	packagemodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/packages"
	portmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/port"
	processmodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/process"
	servicemodule "github.com/Theearthwormsplitsvertically/scan/internal/modules/service"
)

// NewRegistry 创建已注册当前实现模块的默认注册表。
func NewRegistry() (*coremodule.Registry, error) {
	registry := coremodule.NewRegistry()
	if err := registerAll(registry,
		hostmodule.New(),
		networkmodule.New(),
		processmodule.New(),
		portmodule.New(),
		connectionmodule.New(),
		servicemodule.New(),
		packagemodule.New(),
	); err != nil {
		return nil, err
	}
	return registry, nil
}
