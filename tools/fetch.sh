#!/usr/bin/env bash
# Re-download the official docs this schema is generated from, then regenerate.
set -euo pipefail
cd "$(dirname "$0")"
curl -sSL https://code.claude.com/docs/en/settings-reference.md -o ref.md
curl -sSL https://code.claude.com/docs/en/env-vars.md -o env.md
python3 gen.py
