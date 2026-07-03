# cc-present

![cc-present banner](docs/assets/readme-banner.webp)

[![Release](https://img.shields.io/github/v/release/yasyf/cc-present?sort=semver)](https://github.com/yasyf/cc-present/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/yasyf/cc-present/ci.yml?branch=main&label=ci)](https://github.com/yasyf/cc-present/actions/workflows/ci.yml)
[![License: PolyForm-Noncommercial-1.0.0](https://img.shields.io/badge/License-PolyForm--Noncommercial--1.0.0-blue.svg)](https://github.com/yasyf/cc-present/blob/main/LICENSE)

Ad-hoc live web artifacts for Claude sessions — approval boards, choices, and rich content whose every click streams back to the agent.

cc-present turns "here's a static page, reply in chat" into a live loop. A Claude session composes a web page out of typed blocks (approval cards, choices, feedback boxes, code, diffs, tables) and serves it at a localhost URL; every button you click streams back into the session as a typed event, and the agent patches individual blocks in your open tab without a reload.

## Install

Homebrew (macOS):

```bash
brew install yasyf/tap/cc-present
```

Or with the Go toolchain:

```bash
go install github.com/yasyf/cc-present/cmd/cc-present@latest
```

## Quickstart

Write a document of blocks, start it, and watch clicks stream back:

```bash
cat > board.json <<'EOF'
{
  "version": 1,
  "title": "Pick an opener",
  "submit": { "label": "Send decisions" },
  "blocks": [
    { "id": "opener", "type": "card", "title": "README opener", "children": [
      { "id": "pick", "type": "choice", "options": [
        { "id": "a", "label": "A", "md": "**Review Claude's diffs like a PR.**" },
        { "id": "b", "label": "B", "md": "**A PR-style review UI for agent edits.**" }
      ] },
      { "id": "verdict", "type": "approval", "prompt": "Ship the selected opener?" }
    ] }
  ]
}
EOF
cc-present start --session demo --cwd "$PWD" --title "Openers" --doc board.json
# session: demo
# url: http://127.0.0.1:52780/p/openers--3f9c21aa
cc-present watch --session demo --cwd "$PWD"
# {"type":"choice.selected","payload":{"blockId":"pick","optionIds":["a"]}}
# {"type":"decision.created","payload":{"blockId":"verdict","verdict":"approved"}}
# {"type":"submit","payload":{"revision":1}}
```

Open the URL, click, and each interaction appears on the stream as it happens. Inside a Claude Code session the plugin does this wiring for you — install it and say "present this as an approval board":

```
/plugin marketplace add yasyf/cc-present
/plugin install cc-present@cc-present
```

## What problems does this solve?

- **Static artifacts collect decisions out-of-band.** An agent that drafts 26 README openers as an HTML page still needs you to type "card 14, option B" back into chat. Here every card carries its own approve/reject/feedback controls, and each click arrives in the session as a typed event.
- **Approval state gets lost between rounds.** The document is a pure reduction of an append-only event log. Agent content and human decisions live in separate lanes of that log, so a redrafted card keeps your earlier verdicts intact.
- **One-shot UIs go stale mid-conversation.** The agent patches single blocks over the same stream your browser is subscribed to. A rejected opener becomes a redraft in your open tab, no reload.
- **Bespoke review UIs take a repo each.** Blocks compose (markdown, cards, choices, inputs, code, diffs, images, tables, progress), so one JSON document covers an approval board today and a triage dashboard tomorrow.
