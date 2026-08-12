// Package buildinfo exposes build metadata that release builds can set with linker flags.
package buildinfo

// These defaults keep development builds identifiable when no linker flags are supplied.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)
