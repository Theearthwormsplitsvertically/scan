//go:build linux

package main

import (
	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func newRuntime() agent.Runtime {
	return agent.NewLocalRuntime(platform.NewRoot("/"))
}
