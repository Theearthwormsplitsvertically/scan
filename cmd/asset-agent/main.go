// Command asset-agent is the Linux asset collection Agent entry point.
package main

import (
	"context"
	"os"

	"github.com/Theearthwormsplitsvertically/scan/internal/cli"
)

// main connects operating-system inputs to the command dispatcher and returns its exit code.
func main() {
	// The CLI owns process exit codes; main only connects OS arguments and streams.
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, newRuntime()))
}
