//go:build !linux

package main

import "github.com/Theearthwormsplitsvertically/scan/internal/agent"

// newRuntime 让非 Linux 开发构建明确返回“无法采集”，避免误采集当前主机。
func newRuntime() agent.Runtime {
	return agent.UnavailableRuntime{}
}
