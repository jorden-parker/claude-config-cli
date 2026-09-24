// Command ccfg is the short name for claude-config-cli.
package main

import "github.com/jorden-parker/claude-config-cli/internal/cli"

var version = "dev"

func main() { cli.Main("ccfg", version) }
