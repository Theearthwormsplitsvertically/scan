//go:build !linux

package main

import "github.com/Theearthwormsplitsvertically/scan/internal/agent"

// newRuntime keeps development builds explicit: collection is unavailable outside Linux.
func newRuntime() agent.Runtime {
	return agent.UnavailableRuntime{}
}
