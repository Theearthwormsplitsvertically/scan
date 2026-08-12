//go:build !linux

package main

import "github.com/Theearthwormsplitsvertically/scan/internal/agent"

func newRuntime() agent.Runtime {
	return agent.UnavailableRuntime{}
}
