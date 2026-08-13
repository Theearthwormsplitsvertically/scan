// Package cli 从动态模块注册信息生成命令并执行扫描。
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

// Run 分派全局命令和动态模块命令并返回进程退出码。
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime) int {
	return runWithEnvironment(ctx, args, stdout, stderr, runtime, productionEnvironment())
}

func runWithEnvironment(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "缺少命令；使用 help 查看可用命令")
		return 2
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "version 不接受参数")
			return 2
		}
		value := struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			BuildTime string `json:"build_time"`
		}{"asset-agent", buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime}
		if err := report.WriteJSON(stdout, value); err != nil {
			fmt.Fprintf(stderr, "写入版本信息: %v\n", err)
			return 1
		}
		return 0
	case "doctor":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "doctor 不接受参数")
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
			fmt.Fprintf(stderr, "写入 doctor 报告: %v\n", err)
			return 1
		}
		return 0
	case "scan":
		return runLegacyScan(ctx, args[1:], stdout, stderr, runtime, env)
	}

	if runtime == nil {
		fmt.Fprintln(stderr, "runtime unavailable")
		return 1
	}
	infos, err := runtime.Modules(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "读取模块信息: %v\n", err)
		return 1
	}
	infoByName := makeInfoMap(infos)
	switch args[0] {
	case "help", "--help", "-h":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "help 不接受参数")
			return 2
		}
		writeHelp(stdout, infos)
		return 0
	case "modules":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "modules 不接受参数")
			return 2
		}
		if err := report.WriteJSON(stdout, infos); err != nil {
			fmt.Fprintf(stderr, "写入模块列表: %v\n", err)
			return 1
		}
		return 0
	case "all":
		if len(args) < 2 || args[1] != "scan" {
			fmt.Fprintln(stderr, "all 只支持 scan 动作")
			return 2
		}
		return runTargetScan(ctx, "all", args[2:], stdout, stderr, runtime, env)
	}
	info, ok := infoByName[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "未知模块或命令 %q\n", args[0])
		return 2
	}
	if len(args) < 2 {
		fmt.Fprintf(stderr, "模块 %s 缺少动作；使用 scan、describe、status 或 schedule\n", args[0])
		return 2
	}
	switch args[1] {
	case "scan":
		return runTargetScan(ctx, args[0], args[2:], stdout, stderr, runtime, env)
	case "describe":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "describe 不接受参数")
			return 2
		}
		return writeValue(stdout, stderr, info.Descriptor, "模块描述")
	case "status":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "status 不接受参数")
			return 2
		}
		return writeValue(stdout, stderr, info.Support, "模块状态")
	case "schedule":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "schedule 不接受参数")
			return 2
		}
		value := struct {
			Module          string `json:"module"`
			DefaultInterval string `json:"default_interval"`
			ResourceClass   string `json:"resource_class"`
			Timeout         string `json:"timeout"`
		}{info.Descriptor.Name, info.Descriptor.DefaultInterval, info.Descriptor.ResourceClass, info.Descriptor.Timeout}
		return writeValue(stdout, stderr, value, "模块周期")
	default:
		fmt.Fprintf(stderr, "未知模块动作 %q\n", args[1])
		return 2
	}
}

func runLegacyScan(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	if runtime == nil {
		fmt.Fprintln(stderr, "runtime unavailable")
		return 1
	}
	target := "all"
	if len(args) > 0 && args[0] != "-o" && args[0] != "--output" && args[0] != "--output-dir" {
		target = args[0]
		args = args[1:]
	}
	if target == "socket" {
		fmt.Fprintln(stderr, "旧命令 scan socket 已拆分为 port scan 和 connection scan，请明确选择或分别执行两者")
		return 2
	}
	infos, err := runtime.Modules(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "读取模块信息: %v\n", err)
		return 1
	}
	if target != "all" {
		if _, ok := makeInfoMap(infos)[target]; !ok {
			fmt.Fprintf(stderr, "未知扫描模块 %q\n", target)
			return 2
		}
	}
	fmt.Fprintln(stderr, "deprecated: 旧 scan 前缀语法将在后续版本删除，请改用 <module> scan 或 all scan")
	return runTargetScan(ctx, target, args, stdout, stderr, runtime, env)
}

func runTargetScan(ctx context.Context, target string, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	options, err := parseScanOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if target == "all" && options.explicitFile {
		fmt.Fprintln(stderr, "all scan 不支持 -o/--output，请使用 --output-dir")
		return 2
	}
	batch, err := runtime.ScanTarget(ctx, target)
	if err != nil {
		fmt.Fprintf(stderr, "扫描失败: %v\n", err)
		return 1
	}
	if err := writeScanResult(stdout, batch, target, options, env); err != nil {
		fmt.Fprintf(stderr, "写入扫描结果: %v\n", err)
		return 1
	}
	return 0
}

func makeInfoMap(infos []coremodule.Info) map[string]coremodule.Info {
	result := make(map[string]coremodule.Info, len(infos))
	for _, info := range infos {
		result[info.Descriptor.Name] = info
	}
	return result
}

func writeHelp(writer io.Writer, infos []coremodule.Info) {
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Descriptor.Name)
	}
	sort.Strings(names)
	fmt.Fprintln(writer, "用法: asset-agent <version|doctor|modules|all scan|模块 动作>")
	for _, name := range names {
		fmt.Fprintf(writer, "  asset-agent %s <scan|describe|status|schedule>\n", name)
	}
}

func writeValue(stdout, stderr io.Writer, value any, label string) int {
	if err := report.WriteJSON(stdout, value); err != nil {
		fmt.Fprintf(stderr, "写入%s: %v\n", label, err)
		return 1
	}
	return 0
}
