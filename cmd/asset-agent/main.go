// asset-agent 是 Linux 资产采集 Agent 的可执行入口。
package main

import (
	"context"
	"os"

	"github.com/Theearthwormsplitsvertically/scan/internal/cli"
)

// main 将操作系统参数和标准流交给命令分派器，并返回其退出码。
func main() {
	// CLI 统一管理退出码；main 只负责连接操作系统输入输出。
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, newRuntime()))
}
