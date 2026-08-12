package cli

import (
	"fmt"
	"strings"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
)

type scanOptions struct {
	module         agent.Module
	output         string
	explicitOutput bool
	help           bool
}

var plannedModules = map[string]struct{}{
	"service": {}, "package": {}, "container": {}, "application": {}, "file": {}, "security": {},
}

func parseScanOptions(args []string) (scanOptions, error) {
	options := scanOptions{module: agent.ModuleAll}
	index := 0
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "all":
			options.module = agent.ModuleAll
		case "host":
			options.module = agent.ModuleHost
		case "network":
			options.module = agent.ModuleNetwork
		case "process":
			options.module = agent.ModuleProcess
		case "socket":
			options.module = agent.ModuleSocket
		default:
			if _, planned := plannedModules[args[0]]; planned {
				return options, fmt.Errorf("scan module %q is not implemented", args[0])
			}
			return options, fmt.Errorf("unknown scan module %q", args[0])
		}
		index++
	}

	for index < len(args) {
		switch args[index] {
		case "-h", "--help":
			if len(args) != 1 {
				return options, fmt.Errorf("scan help accepts no other arguments")
			}
			options.help = true
			index++
		case "-o", "--output":
			if options.explicitOutput {
				return options, fmt.Errorf("scan output may be specified only once")
			}
			if index+1 >= len(args) {
				return options, fmt.Errorf("%s requires a path", args[index])
			}
			options.output = args[index+1]
			options.explicitOutput = true
			index += 2
		default:
			return options, fmt.Errorf("unknown scan option %q", args[index])
		}
	}
	return options, nil
}
