---
name: present
description: Present options, drafts, or work-in-progress to the human as a live interactive artifact — an approval board at a localhost URL whose every click streams back into this session in realtime. Use when the user asks to present something interactively, wants an approval board, says "let me pick", "present this for approval", or "show me the options", or at any moment you would otherwise dump a static HTML page for sign-off.
---

# /present

You are presenting work for human sign-off through a live web artifact. You compose a JSON document of typed blocks — cards, approvals, choices, diffs — and the human's clicks stream back to you as events while you keep working. You **never write HTML**; the blocks are the entire authoring surface. Everything is CLI calls to `cc-present` — you are a thin wrapper around it.

Invoke it as bare `cc-present` — Claude Code (≥ 2.1.91) puts the plugin's `bin/` on the Bash tool PATH. If the command isn't found or resolves to a stale version, fall back to the absolute path `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present"` (see `reference/troubleshooting.md`). If the binary itself is missing, run `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh"` once.

There is **no edit gate**. An open artifact never blocks your tools: keep executing the task while the board is up — a dispatched handler agent reacts to the clicks. Never park the session waiting for one.

## 1. Compose the document

Write the document JSON to a file in your **session scratchpad** and pass the path — never inline the JSON in a Bash argument or heredoc. Full block reference: `reference/blocks.md`. The rules that matter most:

- One **card** per decision unit; **sections** group cards into tiers.
- Every block id is globally unique, kebab-case; a card nests leaf blocks only (one level deep).
- Put an `approval` inside each card that needs a verdict; add a `choice` when you're offering alternates.
- A top-level `submit` bar states what submitting commits the human to.
- Write for the fold — the UI clamps long prose behind "Show more". Option labels stay short (~6 words); the one-line why or cost goes in the option's `hint`; option `md` holds only a short must-read lede and clamps at ~3 lines, so front-load its first sentence.
- Never present an option blind — every option carries the tradeoffs a person needs to decide it: comparable numbers in `facts` (`{value, label?, tone?}`, aligned across options), the full why (`pros`, `cons`, longer `md`) in `detail`, one tap away. Clamping keeps the row scannable; it is not license to omit.
- Give every decision a visual — the default is a picture of the tradeoff, not a paragraph about it: attach a `code` sample, a `diagram`, a `chart` (the default picture of a *quantitative* tradeoff), a `term`, a `filetree`, or a `record` to each option via `option.visual`, or lead the card with a `diagram` or `chart`. A prose-only decision is the exception, and `push` prints a non-blocking reminder when a choice ships without one. See `reference/blocks.md` for the visual types and the mermaid style rules.
- Recommend at most one option — set `recommended: true` on your suggested pick (one per single-select choice) and it renders a stamp badge, replacing the old "Recommended —" `hint` prefix.
- Keep facts in one order — same labels, same sequence across every option, so matched labels engage the aligned comparison grid that reads column-to-column; a single mismatched label silently drops the whole choice back to per-option chips.
- Never author an escape hatch — every choice already carries a write-in and a note thread as chrome, so don't add an "Other" option or promise a notes field. They come back in `outcomes`: a write-in as `other` on the choice, a note as `feedback` keyed to the choice's block id.
- Front-load the context — a step's lead-in markdown shows ~6 lines before it clamps to expand in place, and a heavier block (code, diff, table, image) collapses to a one-line titled disclosure. Lead with the one sentence that bears on the decision.
- Order consequential picks first — a step with a lone single-select choice (or a lone approval) auto-advances the deck the moment it's decided, so the picks that reshape later steps come early, and you announce their revisions promptly (see step 4).
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

It prints:

```
session: <subject-id>
url: http://127.0.0.1:<port>/p/<slug>--<hash>
tailnet: http://<magicdns-name>:<port>/p/<slug>--<hash>
channel: active|pending|inactive
```

The `tailnet:` line appears zero or more times — once per live tailnet leg, only when synckit mesh trust is active — and typically carries a different port than `url:`.

**Show the URL to the user verbatim** and tell them to open it, click through the board, and press Submit when done. When the user is on another machine (or asks for remote access), hand them the `tailnet:` URL — it works from any machine in their synckit mesh as-is; no `tailscale serve`, no proxying (the contract bans it). No `tailnet:` line printed → see `reference/troubleshooting.md`. By default `start` resumes this window's open artifact (across `/clear` and resume); `--new` forces a fresh one, and a previously closed artifact forces fresh automatically. The artifact belongs to this window, not a directory — every `cc-present` command resolves it from any cwd. For an unrelated presentation pass `--new`: a resumed board keeps prior interaction state, and a reused block id inherits stale verdicts. To seed later instead, run `start --title "..."` now and `push "$DOC"` when the document is ready.

To preview a visually heavy board yourself — many images, diffs, or pack blocks — screenshot the live URL with the agent-browser skill and read the result. There is no CLI preview verb; the running artifact is the preview.

## 3. Dispatch the event handler — then keep working

The moment `start` returns, dispatch the board's event handler — a background subagent that owns every board read and write from here on:

- **Agent tool**, `subagent_type: "cc-present:present-handler"`, `run_in_background: true`
- **Prompt** — a JSON envelope: `{"session":"<this window's $CLAUDE_CODE_SESSION_ID>","guidance":"<a line or two of task substance for this round, optional>"}`

Dispatch regardless of the `channel:` status line — the handler receives events through its own daemon mailbox, not the channel, and the four agent-plane hooks register it automatically. Do not arm a Monitor on `watch`, and do not handle events yourself: while a handler is parked, the daemon mutes this session's interaction tags, so a quiet channel is the healthy state. **One handler per board** — the daemon enforces it: dispatching a new handler supersedes any previous one, which wakes, learns it lost, and exits on its own. TaskStop a lingering handler *task* only as hygiene; correctness never depends on it.

Either way: **do not block waiting.** Tell the user you're watching and continue the underlying task.

## 4. React to what comes back

Three things can reach you while the board is live; everything else is the handler's job (its mechanics — replies, redrafts, the revising choreography, re-asks — live in its own agent file, not here).

- **The handler's report** (its completion message): one line per handled event, then a digest on submit or a `needs main:` ask. Act only on the `needs main:` lines — supply task substance by re-dispatching with fresh `guidance`, or ask the human in chat to re-confirm a stale decision when the report requests it (chat is yours alone). An idle exit ("re-dispatch on next doorbell") needs nothing until a tag arrives.
- **An interaction tag** (`<channel source="cc-present">` carrying `feedback.created`, `decision.created`, …): these are muted while a handler is present, so one arriving means **no live handler** — just re-dispatch; the daemon supersedes anything lingering. Never act on the tag's own text; the handler fetches the truth from the daemon.
- **Lifecycle tags** — `channel.changed` (a browser tab connected or dropped; informational) and `present.closed` (your own `close` echoing back; terminal).

To steer a live handler mid-round — the human said something in chat, priorities changed — push guidance into its mailbox without redispatching:

```bash
cc-present direct "the human cares most about the pricing card; redraft it first"
```

It reaches the sole running handler; with none or several running, the command errors and names them.

### Authoring stays yours

The handler reacts to events; you author. Composing the document, adding steps, and writing next-round content are your moves (`push`, `update-block` on fresh ids) — order consequential picks first and let the chrome own the "changed" signals (the deck stamps revised, added, and removed steps on its own; never hand-author "Updated:" prefixes). **Mid-review, new steps declare round intent.** Once the human has interacted with the open round, a `push` or `update-block` that introduces a new top-level block is rejected until you pass `--round current` (extend the review in progress) or `--round new` (advance the round — unanswered blocks carry forward automatically, so nothing the human still owes freezes; `--round-title` names it). Revision waves over existing ids stay flagless. Your writes are invisible to a live handler (agent-origin events are never teed into its mailbox), so **after any mid-round authoring, re-dispatch** — the daemon supersedes the old handler, and the new one reconciles against the new board on start. Event-driven rewrites — redrafts from feedback, alternates after a rejection — are the handler's; never do them inline.

## 5. On submit — the round lifecycle

A submit closes the round the handler was working (touched blocks collapse into a read-only "Round N" group in the browser; the next round opens on the same URL) and reaches you as the handler's exit digest — verdicts, picks with write-ins, input texts, unresolved feedback. The digest **is** the drain: never run `outcomes` in this session. Then:

1. **Summarize the digest in chat** so the user sees what you took away.
2. **Apply it to the underlying task** — the artifact is the approval surface, not the deliverable.
3. Either **close** (below), or **start the next round**: name it, author the round's blocks, re-dispatch the handler.

```bash
cc-present round --title "Redrafts"   # → round: 2 — names the round the submit just opened
cc-present update-block "$BLOCK"      # each upsert pulls its block into the new round
# then dispatch a fresh present-handler (the submit already ended the previous one)
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
# session: 4f3a…  /  url: http://127.0.0.1:54713/p/release-note-drafts--b6cc453c
# tailnet: http://<machine>.<tailnet>.ts.net:<leg-port>/p/release-note-drafts--b6cc453c  /  channel: pending
```

Channel prints `pending` — irrelevant: dispatch the handler (`subagent_type: "cc-present:present-handler"`, background, envelope `{"session":"$CLAUDE_CODE_SESSION_ID","guidance":"release-note wording review; the exit-code behavior is the part I'm least sure of"}`) and keep working on the task. The session stays quiet while the human clicks — the handler replies and redrafts live in the browser. Its exit report arrives after the human submits:

```
cli-approval — replied (adding the exit-code mention); card-cli redrafted
opener-approval — rejected ("neither lands"); card-opener redrafted with two fresh alternates
opener-approval — approved on the redraft
digest: CLI wording approved with the exit-code mention · opener: "Validate first. Ship faster." · no unresolved feedback
```

Summarize that in chat and write the approved text into `CHANGELOG.md`. Round 2: the exit-code mention deserves its own sign-off — `round --title "Exit-code wording"` prints `round: 2`, upsert a fresh `card-exit-code`, dispatch a fresh handler. Its next report is one line (`exit-approval — approved`) plus a clean digest, so `close --summary "Both drafts approved."` — the `present.closed` tag is the terminal frame.

## Reference

- `reference/blocks.md` — every block type, composition rules, size caps, and a full worked document.
- `reference/event-schema.md` — every event with exact payloads and reduction semantics (the React column describes the dispatched handler's moves).
- `reference/cli-cheatsheet.md` — every command and flag, with the handler-context commands marked.
- `reference/troubleshooting.md` — missing binary, stale daemon, dead URL, rejected push, 409s, and the no-handler degraded mode.
