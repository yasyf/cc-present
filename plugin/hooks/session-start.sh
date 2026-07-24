#!/usr/bin/env bash
# SUBSTRATE — keep this hook. SessionStart: record the session's facts
# (best-effort — does nothing if the daemon isn't up). Reads the hook JSON on
# stdin and passes it to `cc-present session-record`; invoking bin/cc-present
# resolves the version-exact binary via binrun on first use, so this doubles as
# the pre-warm.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

exec "$ROOT/bin/cc-present" session-record
