//go:build linux

package main

import (
	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/modules"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
	linuxprovider "github.com/Theearthwormsplitsvertically/scan/internal/provider/linux"
)

// newRuntime 在 Linux 中将动态模块注册表绑定到只读根目录 Provider。
func newRuntime() (agent.Runtime, error) {
	providers, err := linuxprovider.New(platform.NewRoot("/"))
	if err != nil {
		return nil, err
	}
	registry, err := modules.NewRegistry()
	if err != nil {
		return nil, err
	}
	return agent.NewScanner(registry, providers), nil
}
