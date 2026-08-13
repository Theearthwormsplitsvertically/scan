package cli

import "fmt"

type scanOptions struct {
	outputDir    string
	output       string
	explicitDir  bool
	explicitFile bool
}

func parseScanOptions(args []string) (scanOptions, error) {
	options := scanOptions{}
	for index := 0; index < len(args); {
		switch args[index] {
		case "--output-dir":
			if options.explicitDir {
				return options, fmt.Errorf("--output-dir 只能指定一次")
			}
			if index+1 >= len(args) {
				return options, fmt.Errorf("--output-dir requires a path")
			}
			options.outputDir = args[index+1]
			options.explicitDir = true
			index += 2
		case "-o", "--output":
			if options.explicitFile {
				return options, fmt.Errorf("-o/--output 只能指定一次")
			}
			if index+1 >= len(args) {
				return options, fmt.Errorf("%s requires a path", args[index])
			}
			options.output = args[index+1]
			options.explicitFile = true
			index += 2
		default:
			return options, fmt.Errorf("unknown scan option %q", args[index])
		}
	}
	if options.explicitDir && options.explicitFile {
		return options, fmt.Errorf("--output-dir 与 -o/--output 不能同时使用")
	}
	return options, nil
}
