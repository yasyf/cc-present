---
name: present
description: Present options, drafts, or work-in-progress to the human as a live interactive artifact — an approval board at a localhost URL whose every click streams back into this session in realtime. Use when the user asks to present something interactively, wants an approval board, says "let me pick", "present this for approval", or "show me the options", or at any moment you would otherwise dump a static HTML page for sign-off.
---

# /present

You are presenting work for human sign-off through a live web artifact. You compose a JSON document of typed blocks — cards, approvals, choices, diffs — and the human's clicks stream back to you as events while you keep working. You **never write HTML**; the blocks are the entire authoring surface. Everything is CLI calls to `cc-present` — you are a thin wrapper around it.

The binary is `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present"` — always invoke it by that absolute path, never as bare `cc-present`. If it's missing, run `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh"` once.

There is **no edit gate**. An open artifact never blocks your tools: keep executing the task while the board is up, reacting to events as they arrive. Never park the session waiting for a click.

## 1. Compose the document

Write the document JSON to a file in your **session scratchpad** and pass the path — never inline the JSON in a Bash argument or heredoc. Full block reference: `reference/blocks.md`. The rules that matter most:

- One **card** per decision unit; **sections** group cards into tiers.
- Every block id is globally unique, kebab-case; a card nests leaf blocks only (one level deep).
- Put an `approval` inside each card that needs a verdict; add a `choice` when you're offering alternates.
- A top-level `submit` bar states what submitting commits the human to.
- Local image paths in `image` blocks are fine — `push`, `update-block`, and `start --doc` inline them automatically.

Validate offline before starting (no daemon needed):

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" push --dry-run "$DOC"
```

`ok` on stdout means valid; otherwise the first error prints, naming the offending block id.

## 2. Start the artifact and give the user the URL

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" start --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD" --doc "$DOC"
```

It prints exactly three lines:

```
session: <subject-id>
url: http://127.0.0.1:<port>/p/<slug>--<hash>
channel: active|pending|inactive
```

**Show the URL to the user verbatim** and tell them to open it, click through the board, and press Submit when done. By default `start` resumes this window's open artifact (across `/clear` and resume); `--new` forces a fresh one, and a previously closed artifact forces fresh automatically. To seed later instead, run `start --title "..."` now and `push "$DOC"` when the document is ready.

## 3. Wire up event delivery — then keep working

Route by the `channel:` line from step 2:

- **`active`** — this window's channel is proven and streaming. Do **not** arm a Monitor (you'd receive every event twice). Events arrive as `<channel source="cc-present">` tags carrying the event JSON.
- **`pending`** or **`inactive`** — launch a **Monitor** (`persistent: true`, description `cc-present events`) wrapping:

  ```bash
  "${CLAUDE_PLUGIN_ROOT}/bin/cc-present" watch --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD"
  ```

  Each line it prints is one JSON event. `--session` is required — `watch` does not read the environment, and without it it polls forever.

If a `<channel source="cc-present">` tag arrives while the Monitor is armed, the channel went live: run `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" channel-ack --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD"`, stop the Monitor with **TaskStop**, and rely on tags from then on. Delivery is at-least-once; re-delivery is harmless — decisions, choices, and inputs are last-write-wins, and feedback carries its own `id`.

Either way: **do not block waiting.** Tell the user you're watching and continue the underlying task.

## 4. React to each event live

Each event (Monitor line or channel tag) is the event's JSON payload. Identify it by shape — the payload carries no `type` field (full schema: `reference/event-schema.md`):

| Payload shape | Event | React |
|---|---|---|
| `{blockId, text, id}` | `feedback.created` | Reply under the block, and when the feedback warrants a redraft, upsert the card with `"status": "redrafted"`. |
| `{blockId, verdict, note?}` | `decision.created` | `rejected` — redraft: upsert the card with alternates folded in. `approved` — optionally upsert with `"status": "resolved"`. `cleared` — the human withdrew their verdict; nothing to do. |
| `{blockId, optionIds}` | `choice.selected` | Informational until submit. |
| `{blockId, text}` (no `id`) | `input.submitted` | Informational until submit. |
| `{revision}` | `submit` | Go to step 5. |
| `{type: "channel.changed", connected}` | presence | Informational — a browser tab connected or dropped. |

To reply — it renders under the block in realtime:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" reply --block <blockId> --body "Good catch — redrafting now."
```

To redraft, write the revised block JSON to the scratchpad and upsert it:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" update-block "$BLOCK" [--after <id>]
```

**Prefer `update-block` over a full `push`** — it keeps the event log lean and never disturbs the rest of the board. Upserting replaces the block wholesale (nothing from the old block survives), and human verdicts live outside the document, so a redraft never clobbers a decision. `remove-block <id>` drops a block you no longer want on the board.

## 5. On the `submit` event — drain outcomes, then act

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" outcomes --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD"
```

This prints the reduced state: the current document plus every human interaction keyed by block id — `decisions`, `choices`, `inputs`, `feedback`, your `replies`, and the `submitted` marker. Then:

1. **Summarize the verdicts, picks, and feedback in chat** so the user sees what you took away.
2. **Apply them to the underlying task** — the artifact is the approval surface, not the deliverable.
3. Either **push a revised document** for round 2 (submit does not close the artifact; rounds continue on the same URL) or finish:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" close --summary "All 5 openers approved; two redrafted per feedback."
```

`close` is **terminal**: the board greys out, later interactions get a 409, and no event un-closes it. Stop the Monitor (TaskStop) when you close — your own `present.closed` never streams back to you. A later `start` in this window creates a fresh artifact.

## Worked example, end to end

The user asks: "present the two release-note drafts for approval." Write this to `$DOC` in the scratchpad:

```json
{
  "version": 1,
  "title": "Release-note drafts",
  "intro": "One card per draft. Approve, or leave feedback and I'll redraft live.",
  "submit": { "label": "Send verdicts", "note": "Approved wording goes into CHANGELOG.md as-is." },
  "blocks": [
    {
      "id": "card-cli",
      "type": "card",
      "title": "CLI section",
      "children": [
        { "id": "cli-draft", "type": "markdown", "md": "**New:** `--dry-run` validates a document offline, no daemon required." },
        { "id": "cli-approval", "type": "approval", "prompt": "Ship this wording?" }
      ]
    },
    {
      "id": "card-opener",
      "type": "card",
      "title": "Opening line",
      "children": [
        { "id": "opener-choice", "type": "choice", "prompt": "Which opener?", "options": [
          { "id": "punchy", "label": "Validate first. Ship faster." },
          { "id": "plain", "label": "v2.1 adds offline validation." }
        ]},
        { "id": "opener-approval", "type": "approval", "prompt": "Approve the selected opener?" }
      ]
    }
  ]
}
```

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" push --dry-run "$DOC"     # → ok
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" start --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD" --doc "$DOC"
# session: 4f3a…  /  url: http://127.0.0.1:54713/p/release-note-drafts--b6cc453c  /  channel: pending
```

Channel is `pending`, so arm the Monitor on `watch` and keep working. Events arrive:

- `<channel source="cc-present">{"id":"9f2c11ab","blockId":"cli-approval","text":"mention the exit code"}</channel>` — feedback. React: `reply --block cli-approval --body "Adding it."`, then upsert `card-cli` with the reworded draft and `"status": "redrafted"`.
- `{"blockId":"opener-choice","optionIds":["punchy"]}` — choice. Informational; note it.
- `{"blockId":"opener-approval","verdict":"rejected","note":"neither lands"}` — rejection. React: upsert `card-opener` with two fresh alternates in the choice.
- `{"blockId":"opener-approval","verdict":"approved"}` — the redraft landed. Optionally upsert with `"status": "resolved"`.
- `{"revision":1}` — submit. Run `outcomes`, summarize in chat ("CLI wording approved with the exit-code mention; opener: 'Validate first. Ship faster.'"), write the approved text into `CHANGELOG.md`, then `close --summary "Both drafts approved."` and TaskStop the Monitor.

## Reference

- `reference/blocks.md` — every block type, composition rules, size caps, and a full worked document.
- `reference/event-schema.md` — every event with exact payloads and reduction semantics.
- `reference/cli-cheatsheet.md` — every command and flag.
- `reference/troubleshooting.md` — missing binary, stale daemon, dead URL, rejected push, 409s.
