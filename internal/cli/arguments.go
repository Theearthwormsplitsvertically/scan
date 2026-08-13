package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	coremodule "github.com/Theearthwormsplitsvertically/scan/internal/module"
)

type scanInvocation struct {
	selection  agent.ScanSelection
	selected   []string
	outputRoot string
}

func parseScanInvocation(args []string, infos []coremodule.Info) (scanInvocation, error) {
	known := make(map[string]struct{}, len(infos))
	valid := make([]string, 0, len(infos))
	for _, info := range infos {
		name := info.Descriptor.Name
		known[name] = struct{}{}
		valid = append(valid, "-"+name)
	}
	sort.Strings(valid)
	if len(args) == 0 {
		return scanInvocation{}, fmt.Errorf("缺少扫描参数；有效模块参数: %s", strings.Join(valid, ", "))
	}
	if args[0] == "scan" {
		output, err := parseOutputOnly(args[1:])
		if err != nil {
			return scanInvocation{}, err
		}
		return scanInvocation{selection: agent.ScanSelection{All: true}, selected: []string{}, outputRoot: output}, nil
	}

	selected := make(map[string]bool)
	output := ""
	for index := 0; index < len(args); {
		value := args[index]
		if value == "-output" {
			if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
				return scanInvocation{}, fmt.Errorf("-output requires a path")
			}
			candidate := args[index+1]
			if output != "" && output != candidate {
				return scanInvocation{}, fmt.Errorf("-output 指定了冲突路径")
			}
			output = candidate
			index += 2
			continue
		}
		if strings.HasPrefix(value, "-") {
			name := strings.TrimPrefix(value, "-")
			if _, ok := known[name]; ok {
				selected[name] = true
				index++
				continue
			}
		}
		return scanInvocation{}, fmt.Errorf("未知参数 %q；有效模块参数: %s", value, strings.Join(valid, ", "))
	}
	if len(selected) == 0 {
		return scanInvocation{}, fmt.Errorf("缺少模块参数；有效模块参数: %s", strings.Join(valid, ", "))
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return scanInvocation{selection: agent.ScanSelection{Modules: append([]string{}, names...)}, selected: names, outputRoot: output}, nil
}

func parseOutputOnly(args []string) (string, error) {
	output := ""
	for index := 0; index < len(args); {
		if args[index] != "-output" {
			return "", fmt.Errorf("scan 不接受参数 %q；仅支持 -output <path>", args[index])
		}
		if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
			return "", fmt.Errorf("-output requires a path")
		}
		candidate := args[index+1]
		if output != "" && output != candidate {
			return "", fmt.Errorf("-output 指定了冲突路径")
		}
		output = candidate
		index += 2
	}
	return output, nil
}
