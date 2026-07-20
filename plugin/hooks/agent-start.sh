#!/usr/bin/env bash
# SUBSTRATE — keep this hook. SubagentStart: register the child agent so the
# steering channel can address it. Forwards hook JSON to `cc-present agent-start`.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/cc-present"
[ -x "$BIN" ] || exit 0
exec "$BIN" agent-start
