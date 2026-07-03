#!/usr/bin/env bash
# Regenerates docs/assets/demo-quickstart.png from a real run of examples/quickstart-board.json.
# Runs under a throwaway HOME so it never touches your real daemon. Requires cc-present and
# agent-browser (npm) on PATH. Short temp path: the daemon's unix socket caps path length.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
home="$(mktemp -d /tmp/ccp-demo.XXXXXX)"
trap 'HOME="$home" cc-present stop >/dev/null 2>&1 || true; rm -rf "$home"' EXIT

url="$(HOME="$home" cc-present start --session demo --cwd "$home" --doc "$root/examples/quickstart-board.json" | awk '/^url:/ {print $2}')"
npx --yes agent-browser open "$url"
npx --yes agent-browser wait --load networkidle
npx --yes agent-browser screenshot "$root/docs/assets/demo-quickstart.png"
npx --yes agent-browser close
HOME="$home" cc-present close --session demo --cwd "$home" >/dev/null
echo "wrote $root/docs/assets/demo-quickstart.png"
