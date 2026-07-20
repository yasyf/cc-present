#!/usr/bin/env bash
# SUBSTRATE — keep this hook. PostToolUse(Task|Agent): record a subagent's result
# on its window's subject. Forwards hook JSON to `cc-present agent-report`.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/cc-present"
[ -x "$BIN" ] || exit 0
exec "$BIN" agent-report
