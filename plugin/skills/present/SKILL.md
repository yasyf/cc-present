---
name: present
description: Present options, drafts, or work-in-progress to the human as a live interactive artifact — an approval board at a localhost URL whose every click streams back into this session in realtime. Use when the user asks to present something interactively, wants an approval board, says "let me pick", "present this for approval", or "show me the options", or at any moment you would otherwise dump a static HTML page for sign-off.
---

# /present

You are presenting work for human sign-off through a live web artifact. You compose a JSON document of typed blocks — cards, approvals, choices, diffs — and the human's clicks stream back to you as events while you keep working. You **never write HTML**; the blocks are the entire authoring surface. Everything is CLI calls to `cc-present` — you are a thin wrapper around it.

Invoke it as bare `cc-present` — Claude Code (≥ 2.1.91) puts the plugin's `bin/` on the Bash tool PATH. If the command isn't found or resolves to a stale version, fall back to the absolute path `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present"` (see `reference/troubleshooting.md`). If the binary itself is missing, run `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh"` once.

There is **no edit gate**. An open artifact never blocks your tools: keep executing the task while the board is up, reacting to events as they arrive. Never park the session waiting for a click.

## 1. Compose the document

Write the document JSON to a file in your **session scratchpad** and pass the path — never inline the JSON in a Bash argument or heredoc. Full block reference: `reference/blocks.md`. The rules that matter most:

- One **card** per decision unit; **sections** group cards into tiers.
- Every block id is globally unique, kebab-case; a card nests leaf blocks only (one level deep).
- Put an `approval` inside each card that needs a verdict; add a `choice` when you're offering alternates.
- A top-level `submit` bar states what submitting commits the human to.
- Write for the fold — the UI clamps long prose behind "Show more". Option labels stay short (~6 words); the one-line why or cost goes in the option's `hint`; option `md` holds only a short must-read lede and clamps at ~3 lines, so front-load its first sentence.
- Never present an option blind — every option carries the tradeoffs a person needs to decide it: comparable numbers in `facts` (`{value, label?, tone?}`, aligned across options), the full why (`pros`, `cons`, longer `md`) in `detail`, one tap away. Clamping keeps the row scannable; it is not license to omit.
- A content-heavy card gets a one-sentence `summary` — the dim lede under its title.
- Comparisons go in a `table` block, not parallel markdown paragraphs.
- A card whose children run past ~2 screens splits into multiple cards.
- Local image paths in `image` blocks are fine — `push`, `update-block`, and `start --doc` inline them automatically.

**Choosing a presentation.** Omit `presentation` and the client decides: a board with any decision unit opens in focus mode — one card at a time — and a content-only board opens as the classic board. Set `"presentation": "board"` for a reference or dashboard push the human should scan freely; set `"focus"` to force the deck for a decision-heavy review. Either way it is a hint — the viewer's own toggle wins.

**Pack blocks.** When no built-in fits, installed block packs may supply extra types. Run `cc-present pack list` to see them: each block prints as a dotted type (`example.rating`), and each pack prints the absolute path of its reference fragment — read that fragment before first use; it documents the pack's fields the way `reference/blocks.md` documents the built-ins. Compose a dotted type like any other block. `push --dry-run` validates pack blocks against their schemas too; an uninstalled dotted type fails the dry run with `pack block type "example.rating" is not installed`.

Validate offline before starting (no daemon needed):

```bash
cc-present push --dry-run "$DOC"
```

`ok` on stdout means valid; otherwise **every** violation prints at once, one per line, each naming its offending block id — compose, validate once, fix all in a single pass. A malformed JSON file fails with the line and column of the offending byte.

## 2. Start the artifact and give the user the URL

```bash
cc-present start --session "$CLAUDE_CODE_SESSION_ID" --doc "$DOC"
```

It prints exactly three lines:

```
session: <subject-id>
url: http://127.0.0.1:<port>/p/<slug>--<hash>
channel: active|pending|inactive
```

**Show the URL to the user verbatim** and tell them to open it, click through the board, and press Submit when done. By default `start` resumes this window's open artifact (across `/clear` and resume); `--new` forces a fresh one, and a previously closed artifact forces fresh automatically. The artifact belongs to this window, not a directory — every `cc-present` command resolves it from any cwd. For an unrelated presentation pass `--new`: a resumed board keeps prior interaction state, and a reused block id inherits stale verdicts. To seed later instead, run `start --title "..."` now and `push "$DOC"` when the document is ready.

To preview a visually heavy board yourself — many images, diffs, or pack blocks — screenshot the live URL with the agent-browser skill and read the result. There is no CLI preview verb; the running artifact is the preview.

## 3. Wire up event delivery — then keep working

Route by the `channel:` line from step 2:

- **`active`** — this window's channel is proven and streaming. Do **not** arm a Monitor (you'd receive every event twice). Events arrive as `<channel source="cc-present">` tags carrying the event JSON.
- **`pending`** or **`inactive`** — launch a **Monitor** (`persistent: true`, description `cc-present events`) wrapping:

  ```bash
  cc-present watch --session "$CLAUDE_CODE_SESSION_ID"
  ```

  Each line it prints is one JSON event. `--session` is required — `watch` does not read the environment, and without it it polls forever.

If a `<channel source="cc-present">` tag arrives while the Monitor is armed, the channel went live: run `cc-present channel-ack --session "$CLAUDE_CODE_SESSION_ID"`, stop the Monitor with **TaskStop**, and rely on tags from then on. Delivery is at-least-once; re-delivery is harmless — decisions, choices, and inputs are last-write-wins, and feedback carries its own `id`.

Either way: **do not block waiting.** Tell the user you're watching and continue the underlying task.

## 4. React to each event live

Each event (Monitor line or channel tag) is the event's JSON payload, self-describing via its embedded `type` field — route on that (full schema: `reference/event-schema.md`):

| `type` | Payload | React |
|---|---|---|
| `feedback.created` | `{blockId, id, text, type}` | Reply under the block, and when the feedback warrants a redraft, upsert the card with `"status": "redrafted"`. |
| `decision.created` | `{blockId, note?, type, verdict}` | `rejected` — redraft: upsert the card with alternates folded in. `approved` — optionally upsert with `"status": "resolved"`. `cleared` — the human withdrew their verdict; nothing to do. |
| `choice.selected` | `{blockId, optionIds, type}` | Informational until submit. |
| `input.submitted` | `{blockId, text, type}` | Informational until submit. |
| `pack.interaction` | `{blockId, payload, type}` | A pack block's interaction; `payload`'s shape is the pack's own — see its reference fragment. Informational until submit. |
| `submit` | `{revision, type}` | Also closes the current round when you've touched a block this round. Go to step 5. |
| `channel.changed` | `{type, connected}` | Presence — a browser tab connected or dropped. Informational. |
| `present.closed` | `{summary?, type}` | Your own `close` echoing back — terminal. `watch` exits on it, completing its Monitor; nothing to do. |

To reply — it renders under the block in realtime, on any block type (approvals, inputs, tables, pack blocks alike); an unknown block id is rejected:

```bash
cc-present reply --block <blockId> --body "Good catch — redrafting now."
```

To redraft, write the revised block JSON to the scratchpad and upsert it:

```bash
cc-present update-block "$BLOCK" [--after <id>]
```

**Prefer `update-block` over a full `push`** — it keeps the event log lean and never disturbs the rest of the board. Upserting replaces the block wholesale (nothing from the old block survives), and human verdicts live outside the document, so a redraft never clobbers a decision. `remove-block <id>` drops a block you no longer want on the board.

## 5. On the `submit` event — the round lifecycle

A submit on a board you've touched this round also closes the round: those blocks collapse into a read-only "Round N" group in the browser, and the next round opens on the same URL. A submit on an untouched board records only the revision. Either way, drain the outcomes:

```bash
cc-present outcomes --no-doc --session "$CLAUDE_CODE_SESSION_ID"
```

This prints every human interaction keyed by block id — `decisions`, `choices`, `inputs`, `feedback`, your `replies`, the `submitted` marker, and `rounds` (the closed-round history). `--no-doc` omits the reduced document: you authored it, so re-printing it every drain only burns context. Drop the flag on the rare drain where you need the current document too. Then:

1. **Summarize the verdicts, picks, and feedback in chat** so the user sees what you took away.
2. **Apply them to the underlying task** — the artifact is the approval surface, not the deliverable.
3. Either **close** (below), or **start the next round**: optionally name it, then upsert the blocks the round is about.

```bash
cc-present round --title "Redrafts"   # → round: 2 — names the round the submit just opened
cc-present update-block "$BLOCK"      # each upsert pulls its block into the new round
```

**Carry-forward is explicit.** After a submit, a block stays actionable only if you re-upsert it — even unchanged. Touching an old block pulls it into the current round; that *is* the redraft flow, and the old version stays frozen in the collapsed group. Blocks you don't touch stay in the closed round, read-only. A full `push` pulls the entire new document into the current round. Inputs come back fresh each round automatically (the UI shows a dim "last round" hint) — never ask the human to clear a field.

To finish instead:

```bash
cc-present close --summary "All 5 openers approved; two redrafted per feedback."
```

`close` is **terminal**: the board greys out, later interactions get a 409, and no event un-closes it. The `present.closed` event streams back to you as the final frame, and `watch` exits on it — a Monitor wrapping `watch` completes on its own, no TaskStop needed. A later `start` in this window creates a fresh artifact.

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
          { "id": "punchy", "label": "Validate first. Ship faster.", "hint": "my pick — command form",
            "facts": [ { "label": "frame", "value": "command" }, { "label": "words", "value": "4" } ],
            "detail": { "pros": [ "Names the benefit, not the feature" ], "cons": [ "Says nothing about what shipped" ] } },
          { "id": "plain", "label": "v2.1 adds offline validation.", "hint": "literal, safer",
            "facts": [ { "label": "frame", "value": "changelog", "tone": "warn" }, { "label": "words", "value": "4" } ],
            "detail": { "pros": [ "States the actual change" ], "cons": [ "Reads like any other release note" ] } }
        ]},
        { "id": "opener-approval", "type": "approval", "prompt": "Approve the selected opener?" }
      ]
    }
  ]
}
```

```bash
cc-present push --dry-run "$DOC"     # → ok
cc-present start --session "$CLAUDE_CODE_SESSION_ID" --doc "$DOC"
# session: 4f3a…  /  url: http://127.0.0.1:54713/p/release-note-drafts--b6cc453c  /  channel: pending
```

Channel is `pending`, so arm the Monitor on `watch` and keep working. Events arrive:

- `<channel source="cc-present">{"blockId":"cli-approval","id":"9f2c11ab","text":"mention the exit code","type":"feedback.created"}</channel>` — feedback. React: `reply --block cli-approval --body "Adding it."`, then upsert `card-cli` with the reworded draft and `"status": "redrafted"`.
- `{"blockId":"opener-choice","optionIds":["punchy"],"type":"choice.selected"}` — choice. Informational; note it.
- `{"blockId":"opener-approval","note":"neither lands","type":"decision.created","verdict":"rejected"}` — rejection. React: upsert `card-opener` with two fresh alternates in the choice.
- `{"blockId":"opener-approval","type":"decision.created","verdict":"approved"}` — the redraft landed. Optionally upsert with `"status": "resolved"`.
- `{"revision":1,"type":"submit"}` — submit. Both cards were upserted this round, so the submit also closes round 1: they collapse into a read-only "Round 1" group. Run `outcomes`, summarize in chat ("CLI wording approved with the exit-code mention; opener: 'Validate first. Ship faster.'"), write the approved text into `CHANGELOG.md`.
- Round 2: the exit-code mention deserves its own sign-off. `round --title "Exit-code wording"` prints `round: 2`, then upsert a fresh `card-exit-code` — the live board shows only that card, with round 1 collapsed above it. On `{"blockId":"exit-approval","type":"decision.created","verdict":"approved"}` and the next submit, `close --summary "Both drafts approved."` — the Monitor's `watch` prints `{"summary":"Both drafts approved.","type":"present.closed"}` and exits on its own.

## Reference

- `reference/blocks.md` — every block type, composition rules, size caps, and a full worked document.
- `reference/event-schema.md` — every event with exact payloads and reduction semantics.
- `reference/cli-cheatsheet.md` — every command and flag.
- `reference/troubleshooting.md` — missing binary, stale daemon, dead URL, rejected push, 409s.
