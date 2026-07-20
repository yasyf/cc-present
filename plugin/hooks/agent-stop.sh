#!/usr/bin/env bash
# SUBSTRATE — keep this hook. SubagentStop: report the child's result and let the
# stop-gate hold it for pending directives. Forwards hook JSON to `cc-present agent-stop`.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/cc-present"
[ -x "$BIN" ] || exit 0
exec "$BIN" agent-stop
