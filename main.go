// Command claude-config is a Charm-powered terminal UI and CLI for editing
// every Claude Code setting.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"github.com/jorden-parker/claude-config-cli/internal/cli"
)

var version = "dev"

func main() {
	if err := fang.Execute(context.Background(), cli.Root(), fang.WithVersion(version)); err != nil {
		os.Exit(1)
	}
}
