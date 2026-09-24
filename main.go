// Command claude-config-cli is a Charm-powered terminal UI and CLI for editing
// every Claude Code setting.
package main

import "github.com/jorden-parker/claude-config-cli/internal/cli"

var version = "dev"

func main() { cli.Main("claude-config-cli", version) }
