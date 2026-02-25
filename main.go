package main

import (
	"os"

	"github.com/timbrinded/hlgo/cmd"
)

// version is set at build time via:
//
//	go build -ldflags "-X main.version=x.y.z"
var version = "dev"

func main() {
	if err := cmd.NewRootCommand(version).Execute(); err != nil {
		os.Exit(1)
	}
}
