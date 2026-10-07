#!/usr/bin/env bash
# Deny every tool in the Tools pane in a throwaway project and ask the installed
# Claude Code to start there. It warns about each name it doesn't know.
# Sends one short prompt, so it needs a logged-in `claude`.
set -euo pipefail
cd "$(dirname "$0")"
names=$(sed -n 's/^\t\t{"\([A-Za-z]*\)", ".*/"\1"/p' ../internal/tui/tools.go | paste -sd, -)
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
mkdir "$dir/.claude"
echo "{\"permissions\":{\"deny\":[$names]}}" >"$dir/.claude/settings.json"
warnings=$(cd "$dir" && claude -p "reply ok" --model haiku --setting-sources project 2>&1 >/dev/null </dev/null | grep '^Permission deny rule' || true)
if [ -n "$warnings" ]; then
  echo "$warnings"
  exit 1
fi
echo "$(claude --version) accepts every tool name"
