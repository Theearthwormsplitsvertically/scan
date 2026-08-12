//go:build linux

package main

import (
	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

// newRuntime creates the real read-only Linux runtime for a Linux build.
func newRuntime() agent.Runtime {
	return agent.NewLocalRuntime(platform.NewRoot("/"))
}
