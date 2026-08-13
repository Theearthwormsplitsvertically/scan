// Package cli 从动态模块注册信息生成命令并执行扫描。
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
	"github.com/Theearthwormsplitsvertically/scan/internal/report"
)

// Run 分派基础命令和动态模块参数并返回进程退出码。
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime) int {
	return runWithEnvironment(ctx, args, stdout, stderr, runtime, productionEnvironment())
}

func runWithEnvironment(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime, env environment) int {
	if len(args) == 1 && args[0] == "version" {
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
	}
	if runtime == nil {
		fmt.Fprintln(stderr, "runtime unavailable")
		return 1
	}
	if len(args) == 1 && args[0] == "doctor" {
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
	}
	infos, err := runtime.Modules(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "读取模块信息: %v\n", err)
		return 1
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		writeHelp(stdout, infos)
		return 0
	}
	if len(args) == 1 && args[0] == "modules" {
		if err := writeModules(stdout, infos); err != nil {
			fmt.Fprintf(stderr, "写入模块列表: %v\n", err)
			return 1
		}
		return 0
	}
	invocation, err := parseScanInvocation(args, infos)
	if err != nil {
		fmt.Fprintln(stderr, err)
		writeHelp(stderr, infos)
		return 2
	}
	outcome, err := runtime.Scan(ctx, invocation.selection)
	if err != nil {
		fmt.Fprintf(stderr, "扫描失败: %v\n", err)
		return 1
	}
	if err := writeScanResult(stdout, outcome, invocation.selected, invocation.outputRoot, env); err != nil {
		fmt.Fprintf(stderr, "写入扫描结果: %v\n", err)
		return 1
	}
	return 0
}

func writeModules(writer io.Writer, infos []coremodule.Info) error {
	items := append([]coremodule.Info{}, infos...)
	sort.Slice(items, func(i, j int) bool { return items[i].Descriptor.Name < items[j].Descriptor.Name })
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "MODULE\tSTATUS\tINTERVAL\tRESOURCE\tTIMEOUT\tDEPENDENCIES"); err != nil {
		return err
	}
	for _, info := range items {
		status := string(info.Support.Status)
		if status == "ok" || status == "complete" {
			status = "supported"
		}
		dependencies := append([]string{}, info.Descriptor.HardDependencies...)
		sort.Strings(dependencies)
		dependencyText := strings.Join(dependencies, ",")
		if dependencyText == "" {
			dependencyText = "-"
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", info.Descriptor.Name, status,
			info.Descriptor.DefaultInterval, info.Descriptor.ResourceClass, info.Descriptor.Timeout, dependencyText); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeHelp(writer io.Writer, infos []coremodule.Info) {
	flags := make([]string, 0, len(infos))
	for _, info := range infos {
		flags = append(flags, "-"+info.Descriptor.Name)
	}
	sort.Strings(flags)
	fmt.Fprintln(writer, "用法:")
	fmt.Fprintf(writer, "  asset-agent %s [-output <目录>]\n", strings.Join(flags, " "))
	fmt.Fprintln(writer, "  asset-agent scan [-output <目录>]")
	fmt.Fprintln(writer, "  asset-agent <modules|doctor|version|help>")
}
