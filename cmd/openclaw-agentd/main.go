// Package main is the entry point for the openclaw-agentd CLI.
package main

import (
	"os"

	"github.com/burmaster/openclaw-agentd/cmd/openclaw-agentd/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
