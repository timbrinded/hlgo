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
	if err := cmd.NewRootCommand(version, commit, date).Execute(); err != nil {
		os.Exit(output.WriteError(os.Stderr, err))
	}
}
