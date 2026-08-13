//go:build !linux

package main

import (
	"runtime"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/modules"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

// newRuntime 在其他系统保留相同模块命令，并让缺失能力明确返回 unsupported。
func newRuntime() (agent.Runtime, error) {
	providers, err := provider.NewSet(runtime.GOOS)
	if err != nil {
		return nil, err
	}
	registry, err := modules.NewRegistry()
	if err != nil {
		return nil, err
	}
	return agent.NewScanner(registry, providers), nil
}
