package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Theearthwormsplitsvertically/scan/internal/agent"
	"github.com/Theearthwormsplitsvertically/scan/internal/buildinfo"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime) int {
	if len(args) == 1 && args[0] == "version" {
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
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "write version: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) == 1 && args[0] == "watch" {
		fmt.Fprintln(stderr, "watch is not implemented in this milestone")
		return 2
	}
	if len(args) == 1 && (args[0] == "doctor" || args[0] == "scan") {
		if runtime == nil {
			fmt.Fprintln(stderr, "runtime unavailable")
			return 1
		}

		var value any
		var err error
		if args[0] == "doctor" {
			value, err = runtime.Doctor(ctx)
		} else {
			value, err = runtime.Scan(ctx)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", args[0], err)
			return 1
		}
		if err := json.NewEncoder(stdout).Encode(value); err != nil {
			fmt.Fprintf(stderr, "write %s report: %v\n", args[0], err)
			return 1
		}
		return 0
	}

	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	fmt.Fprintf(stderr, "unknown command %q; use version, doctor, or scan\n", command)
	return 2
}
