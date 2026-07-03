# Event schema

`cc-present watch` prints one JSON object per line; the channel delivers the same JSON inside `<channel source="cc-present">` tags. Either way the frame is the event's **payload only** — no envelope, and (except for presence) no `type` field — so you identify each event by its payload shape. Your own agent-origin events are filtered out and never echo back.

## Events you receive

| Event | Exact payload | Semantics | React |
|---|---|---|---|
| `decision.created` | `{"blockId":"cli-approval","verdict":"approved"}` — `verdict` is `approved`, `rejected`, or `cleared`; a `note` key appears when the human attached one | Last-write-wins per block; `cleared` removes the decision, returning the block to undecided | `rejected`: redraft — upsert the card with alternates folded in. `approved`: optionally upsert with `"status": "resolved"`. `cleared`: nothing. |
| `choice.selected` | `{"blockId":"opener-choice","optionIds":["punchy"]}` — `optionIds` is always an array, possibly empty | Last-write-wins per block | Informational until submit. |
| `feedback.created` | `{"blockId":"cli-approval","id":"9f2c11ab","text":"mention the exit code"}` | Append-only per block; `id` is the entry's stable identity | `reply --block` under it; redraft the card via `update-block` with `"status": "redrafted"` when warranted. |
| `input.submitted` | `{"blockId":"board-notes","text":"also check the docs site"}` | Last-write-wins per block | Informational until submit. |
| `submit` | `{"revision":1}` | Marks submitted with the revision; does **not** close the artifact — rounds continue | Run `outcomes`, summarize in chat, apply, then push a revised doc or `close`. |
| `channel.changed` | `{"type":"channel.changed","connected":true}` | Presence frame (system origin); skipped by the reducer | Informational — a browser tab connected or dropped. Needs no reply. |

Shape discrimination, in one pass: a `revision` key means submit; `verdict` means decision; `optionIds` means choice; `blockId`+`text` **with** an `id` means feedback and **without** one means input; `"type":"channel.changed"` means presence.

Delivery is at-least-once: `watch` resumes from a persisted cursor, and a channel/Monitor overlap can duplicate a frame. Re-delivery is harmless by construction — decisions, choices, and inputs are last-write-wins; feedback dedupes by its `id`; a re-delivered `submit` just re-runs `outcomes`.

## Agent-origin events (never delivered to you)

For completeness — these are what your own CLI calls append. The browser reduces them; you never receive them back, so after `close` you stop your own Monitor:

| Event | Payload | Appended by |
|---|---|---|
| `doc.replaced` | `{doc, revision}` | `start --doc`, `push` |
| `block.upserted` | `{block, after?}` — replaces the block in place as a whole (nothing from the old block survives), or inserts after `after`, appending when absent or unknown | `update-block` |
| `block.removed` | `{id}` — unknown id is a no-op | `remove-block` |
| `reply.created` | `{id, blockId, md}` — append-only thread under the block | `reply` |
| `present.closed` | `{summary?}` — terminal; every later event is a no-op in the reduction, and the REST edge answers new interactions with 409 | `close` |

## The reduced state (`outcomes`)

`outcomes` prints the full reduction — the current document plus every human interaction keyed by block id:

```json
{
  "doc": { "version": 1, "title": "…", "blocks": [] },
  "interactions": {
    "decisions": { "cli-approval": { "verdict": "approved" } },
    "choices":   { "opener-choice": { "optionIds": ["punchy"] } },
    "inputs":    { "board-notes": { "text": "also check the docs site" } },
    "feedback":  { "cli-approval": [ { "id": "9f2c11ab", "text": "mention the exit code" } ] },
    "replies":   { "cli-approval": [ { "id": "4dd66d6b", "md": "Adding it." } ] },
    "submitted": { "value": true, "revision": 1 },
    "closed":    { "value": false }
  }
}
```

Document state and human state never mix: the document carries only agent-owned display state (`card.status`, `progress`), while verdicts live in `interactions`, keyed by block id — which is why re-upserting a block never clobbers a human's decision, and why a redrafted card's approval block keeps its id if you want the standing verdict to survive the redraft (give it a fresh id to demand a fresh verdict).
