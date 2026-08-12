//go:build linux

package main

import (
	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// newRuntime 在 Linux 构建中创建以根目录为采集根的只读运行时。
func newRuntime() agent.Runtime {
	return agent.NewLocalRuntime(platform.NewRoot("/"))
}
