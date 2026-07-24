#!/usr/bin/env bash
# MCP entrypoint for the opt-in channel server. bin/cc-present resolves the
# version-exact binary via binrun on first use — its resolution diagnostics go
# to stderr, keeping stdout clean for the MCP stdio transport — then execs it.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

exec "$ROOT/bin/cc-present" channel
