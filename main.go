package main

import (
	"os"

	"github.com/timbrinded/hlgo/cmd"
	"github.com/timbrinded/hlgo/pkg/output"
)

var (
	// Build metadata is injected via ldflags.
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	info := cmd.BuildInfo{Version: version, Commit: commit, Date: date}
	if err := cmd.NewRootCommand(info).Execute(); err != nil {
		os.Exit(output.WriteError(os.Stderr, err))
	}
}
