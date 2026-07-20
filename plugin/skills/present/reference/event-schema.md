# Event schema

`cc-present watch` prints one JSON object per line; the channel delivers the same JSON inside `<channel source="cc-present">` tags; the dispatched `present-handler` agent receives the same payloads as mailbox directives with origin `event`. Every frame is the event's **payload**, self-describing via an embedded `type` field — no envelope, no seq. Route on `type`. The **React** column below describes the dispatched handler's moves — the main session only routes (SKILL.md step 4). Your own agent-origin events are filtered out of every feed and never echo back; the one lifecycle event you do receive is the `system`-origin `present.closed`, on which `watch` exits.

## Events you receive

| `type` | Exact payload | Semantics | React |
|---|---|---|---|
| `decision.created` | `{"blockId":"cli-approval","type":"decision.created","verdict":"approved"}` — `verdict` is `approved`, `rejected`, or `cleared`; a `note` key appears when the human attached one | Last-write-wins per block; `cleared` removes the decision, returning the block to undecided | `rejected`: redraft — upsert the card with alternates folded in. `approved`: optionally upsert with `"status": "resolved"`. `cleared`: nothing. |
| `choice.selected` | `{"blockId":"opener-choice","optionIds":["punchy"],"type":"choice.selected"}` — `optionIds` is always an array; an `other` key appears when the human wrote in past the authored options | Last-write-wins per block; a re-pick replaces the whole selection, dropping any prior `other` | Informational until submit; the write-in surfaces as `other` in `outcomes`. |
| `feedback.created` | `{"blockId":"cli-approval","id":"9f2c11ab","text":"mention the exit code","type":"feedback.created"}` | Append-only per block — an approval or a choice alike, so a choice carries a note thread beside its selection; `id` is the entry's stable identity | `reply --block` under it; redraft the card via `update-block` with `"status": "redrafted"` when warranted. |
| `input.submitted` | `{"blockId":"board-notes","text":"also check the docs site","type":"input.submitted"}` | Last-write-wins per block | Informational until submit. |
| `pack.interaction` | `{"blockId":"ex-rating","payload":{"value":4},"type":"pack.interaction"}` — `payload` is the pack block's own interaction shape, validated against the pack's interaction schema at the REST edge | Last-write-wins per block; the reducer stores the payload verbatim under `interactions.packs`, never inspecting its shape | Informational until submit; the pack's reference fragment says what the payload means. |
| `submit` | `{"revision":1,"type":"submit"}` | Marks submitted with the revision; when the round is dirty (some top-level block was agent-touched this round) it also snapshots the round into `rounds.history` (with `submittedRevision`) and advances the round; either way it clears the revising working set. Never closes the artifact | Run `outcomes`, summarize in chat, apply, then start the next round or `close`. |
| `channel.changed` | `{"type":"channel.changed","connected":true}` | Presence frame (system origin); skipped by the reducer | Informational — a browser tab connected or dropped. Needs no reply. |
| `present.closed` | `{"summary":"Both drafts approved.","type":"present.closed"}` — `summary` only when `close --summary` passed one | Terminal (system origin): your own `close` echoing back; every later event is a no-op in the reduction | Nothing — `watch` exits on it, so its Monitor completes on its own. |

Delivery is at-least-once: `watch` resumes from a persisted cursor, and a channel/Monitor overlap can duplicate a frame. Re-delivery is harmless by construction — decisions, choices, and inputs are last-write-wins; feedback dedupes by its `id`; a re-delivered `submit` just re-runs `outcomes`.

## Agent-origin events (never delivered to you)

For completeness — these are what your own CLI calls append. The browser reduces them; you never receive them back. The exception is `close`: its `present.closed` event is recorded with a `system` origin, so it does come back to you as the terminal frame above.

| Event | Payload | Appended by |
|---|---|---|
| `doc.replaced` | `{doc, revision}` | `start --doc`, `push` |
| `block.upserted` | `{block, after?}` — replaces the block in place as a whole (nothing from the old block survives) wherever its id lives, top level or inside a card; a new id inserts after a top-level `after`, into a card after a card-child `after`, appending top-level when `after` is absent (the daemon rejects unknown `after` ids; the reducer keeps the append fallback so old logs replay). A child write stamps the enclosing card's round | `update-block` |
| `block.removed` | `{id}` — removes a top-level block or splices a card child (restamping the card's round); unknown id stays a reducer no-op, though the daemon now rejects it at the edge | `remove-block` |
| `reply.created` | `{id, blockId, md}` — append-only thread under the block | `reply` |
| `round.started` | `{title?}` — dirty round: snapshots it into `rounds.history` (no `submittedRevision`) and advances, then titles the new round; clean round: only titles the current one. Either way it clears the revising working set. The revision is untouched; it counts `doc.replaced` alone | `round` |
| `revising.changed` | `{blockIds, note?}` — declares the top-level block ids you're rewriting plus an optional shared note; replace-set last-write-wins. A `block.upserted` or `block.removed` clears its id, and draining the last id clears the note; `doc.replaced` clears all, and so does a `submit` or `round.started`, note included. Warn-only, so the human's controls stay live | `revising` |

## The reduced state (`outcomes`)

`outcomes` prints the full reduction — the current document plus every human interaction keyed by block id:

```json
{
  "doc": { "version": 1, "title": "…", "blocks": [] },
  "interactions": {
    "decisions": { "cli-approval": { "verdict": "approved" } },
    "choices":   { "opener-choice": { "optionIds": ["punchy"] } },
    "inputs":    { "board-notes": { "text": "also check the docs site", "round": 1 } },
    "packs":     { "ex-rating": { "payload": { "value": 4 } } },
    "feedback":  { "cli-approval": [ { "id": "9f2c11ab", "text": "mention the exit code" } ] },
    "replies":   { "cli-approval": [ { "id": "4dd66d6b", "md": "Adding it." } ] },
    "submitted": { "value": true, "revision": 1 },
    "closed":    { "value": false }
  },
  "rounds": {
    "current": 2,
    "blockRounds": { "card-cli": 2, "card-opener": 1 },
    "history": [
      { "number": 1, "blocks": [], "decisions": {}, "choices": {}, "inputs": {}, "packs": {}, "feedback": {}, "submittedRevision": 1 }
    ]
  },
  "revising": { "blockIds": [] }
}
```

`rounds` partitions the board over time. `current` is 1-based; `blockRounds` maps each top-level block id to the round of its last agent touch — an upsert stamps the block into the current round, a full push stamps the entire document. Each closed round lands in `history` as a frozen record: deep copies of that round's blocks plus the decisions, choices, inputs, and feedback filtered to those blocks (card children included, one level deep). `submittedRevision` appears only when a submit closed the round, not a `round` call. An `InputValue` carries the round it was entered in; a carried-forward input renders empty each round with a dim "last round" hint, so never ask the human to clear a field.

`revising` is your own declared working set reflected back in state (see Agent-origin events): the top-level block ids you announced you're rewriting, plus an optional shared `note`. It clears itself as you upsert those blocks, and any submit or `round` boundary clears the whole set, so it reads `{ "blockIds": [] }` at rest — it is your announcement, not a human interaction. A rewrite that lands under a new id clears nothing on its own: send `revising --clear` or remove the superseded block. On the human side, a `choices` entry gains an `other` string when the human wrote in past the options, and `feedback` keys any block, so a choice's note thread lands there beside its selection.

Document state and human state never mix: the document carries only agent-owned display state (`card.status`, `progress`), while verdicts live in `interactions`, keyed by block id — which is why re-upserting a block never clobbers a human's decision, and why a redrafted card's approval block keeps its id if you want the standing verdict to survive the redraft (give it a fresh id to demand a fresh verdict).

The flip side: interactions outlive their blocks, so `outcomes` may hold keys for blocks a later redraft removed — match interaction keys against the current `doc.blocks` when applying results.
