#!/usr/bin/env bash
# SUBSTRATE — keep this hook. PreToolUse: drain the child agent's mailbox and
# inject pending directives. Forwards hook JSON to `cc-present agent-inject`.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/cc-present"
[ -x "$BIN" ] || exit 0
exec "$BIN" agent-inject
