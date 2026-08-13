// asset-agent 是 Linux 资产采集 Agent 的可执行入口。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Theearthwormsplitsvertically/scan/internal/cli"
)

// main 将操作系统参数和标准流交给命令分派器，并返回其退出码。
func main() {
	runtime, err := newRuntime()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化扫描器失败: %v\n", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, runtime))
}
