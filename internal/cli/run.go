// cli 包解析本地 Agent 命令并写出 JSON 结果。
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

// Run 分派 version、doctor、scan 命令并返回进程退出码。
// 退出码 2 表示命令用法错误；退出码 1 表示运行时或输出失败。
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime) int {
	return runWithEnvironment(ctx, args, stdout, stderr, runtime, productionEnvironment())
}

func runWithEnvironment(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "unknown command \"\"; use version, doctor, or scan")
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		writeHelp(stdout)
		return 0
	case "version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "version accepts no options")
			return 2
		}
		result := struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			BuildTime string `json:"build_time"`
		}{
			Name:      "asset-agent",
			Version:   buildinfo.Version,
			Commit:    buildinfo.Commit,
			BuildTime: buildinfo.BuildTime,
		}
		if err := report.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "write version: %v\n", err)
			return 1
		}
		return 0
	case "watch":
		fmt.Fprintln(stderr, "watch is not implemented in this milestone")
		return 2
	case "doctor":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "doctor accepts no options")
			return 2
		}
		if runtime == nil {
			fmt.Fprintln(stderr, "runtime unavailable")
			return 1
		}
		value, err := runtime.Doctor(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "doctor: %v\n", err)
			return 1
		}
		if err := report.WriteJSON(stdout, value); err != nil {
			fmt.Fprintf(stderr, "write doctor report: %v\n", err)
			return 1
		}
		return 0
	case "scan":
		return runScan(ctx, args[1:], stdout, stderr, runtime, env)
	default:
		fmt.Fprintf(stderr, "unknown command %q; use version, doctor, or scan\n", args[0])
		return 2
	}
}

// runScan 校验 scan 专用参数、执行一次扫描，并选择标准输出或指定文件。
func runScan(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	options, err := parseScanOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if options.help {
		writeScanHelp(stdout)
		return 0
	}
	if runtime == nil {
		fmt.Fprintln(stderr, "runtime unavailable")
		return 1
	}
	var value any
	if options.module == agent.ModuleAll {
		value, err = runtime.Scan(ctx)
	} else {
		value, err = runtime.ScanModule(ctx, options.module)
	}
	if err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return 1
	}
	if err := writeScanResult(stdout, value, string(options.module), options.output, options.explicitOutput, env); err != nil {
		fmt.Fprintf(stderr, "write scan report: %v\n", err)
		return 1
	}
	return 0
}

func writeHelp(writer io.Writer) {
	fmt.Fprintln(writer, "usage: asset-agent <version|doctor|scan>\n       asset-agent scan [all|host|network|process|socket] [-o path]")
}

func writeScanHelp(writer io.Writer) {
	fmt.Fprintln(writer, "usage: asset-agent scan [all|host|network|process|socket] [-o path]\nplanned: service, package, container, application, file, security")
}
