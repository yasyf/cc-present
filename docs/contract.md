# The cc-present contract

This is the wire contract shared by the Go daemon and the browser SPA. The Go
structs live in `internal/doc` and `internal/state`; the canonical TypeScript
declarations live in `web/src/schema.ts` and `web/src/events.ts`, and the Go structs
mirror them field for field with camelCase JSON tags.

Two rules shape everything below.

Document state and human state never mix. The document carries only agent-owned
display state such as `card.status` and `progress`. Human verdicts live in a second
reduction of the same log, keyed by block id, so an agent re-upserting a block never
clobbers a human's decision.

Document state is a pure reduction of the append-only log. Replaying the log from
seq 0 reconstructs a fresh tab's state; there is no get-document endpoint.

## Document envelope

```ts
Doc = { version: 1, title, intro?, stats?: {label, value}[], submit?: {label, note?}, blocks: Block[] }
```

`blocks` is a flat list of blocks. A `section`, a `card`, or any of the nine leaf
blocks may appear directly in `blocks`; a card nests leaf blocks only.

## Block schema

| Block | Level | Fields | Notes |
|---|---|---|---|
| `section` | top | `id`, `type`, `title`, `md?` | Header marker with optional prose. |
| `card` | top | `id`, `type`, `title?`, `chips?`, `flagged?`, `status?`, `children` | `chips[].tone` is one of `default`, `flag`, `demo`. `status` is one of `open`, `resolved`, `redrafted` and is agent-owned. `children` nests one level of leaf blocks. |
| `approval` | top or child | `id`, `type`, `prompt?`, `allowFeedback?` | `allowFeedback` defaults to true at render time. |
| `choice` | top or child | `id`, `type`, `prompt?`, `multi?`, `options` | `options[]` is `{id, label, md?}`; option ids are unique within the block. |
| `input` | top or child | `id`, `type`, `label`, `placeholder?`, `multiline?` | Free-text field. |
| `markdown` | top or child | `id`, `type`, `md`, `struck?` | `struck` applies the "was:" treatment. |
| `code` | top or child | `id`, `type`, `lang`, `code`, `title?` | |
| `diff` | top or child | `id`, `type`, `diff`, `title?` | Unified diff text. |
| `image` | top or child | `id`, `type`, `src`, `alt`, `caption?` | `src` is `https://…`, `asset:<sha256>`, or `data:…`. |
| `table` | top or child | `id`, `type`, `columns`, `rows` | `columns[]` is `{key, label, align?}` where `align` is `left` or `right`; `rows[]` is a `Record<string,string>` of inline-markdown cells. |
| `progress` | top or child | `id`, `type`, `label`, `value`, `max`, `state?` | `state` is one of `active`, `done`, `error` and is agent-owned. |

### Validation

`Doc.Validate` returns the first violation, naming the offending block id:

- `version` must be 1; `title` must be non-empty.
- Every block id is globally unique, including card children. Choice option ids are
  unique within their block.
- A `section`, `card`, or leaf block may appear at the top level. A card may not
  contain a section or another card, so the tree is at most one nesting level deep.
- Per-type required fields are present and non-empty, such as `input.label`,
  `markdown.md`, `code.lang` and `code.code`, `image.src` and `image.alt`, and
  `progress.label`.
- On `progress`, `max` is greater than 0 and `value` is within `[0, max]`.
- `image.src` is `https://…`, `asset:` followed by 64 lowercase hex characters, or
  `data:…`. A `data:` URI is at most **32 KiB**.
- The serialized document is at most **1 MiB**.

Unknown block types are rejected at decode time, before `Validate`, with a message
naming the offending block id.

## Event taxonomy

The event `type` is the reduction discriminant. Each log entry is
`{ origin, type, seq, payload }`, and the reducer orders by `seq`. Document state is
the reduction of these events. The append chokepoint also injects a top-level
`"type"` into every payload, so a frame delivered bare — a `watch` line, a channel
tag, an SSE `data:` field — is self-describing without the envelope. The payload
columns below omit that injected field; both reducers tolerate it.

| Origin | Type | Payload | Reduction |
|---|---|---|---|
| agent | `doc.replaced` | `{doc, revision}` | Replace the whole document. Interaction state survives. `revision` is transport metadata and is not part of the reduced state. |
| agent | `block.upserted` | `{block, after?}` | If a block with `block.id` exists, replace it in place as a whole block, so nothing from the old block survives. Otherwise insert after `after`, or append when `after` is absent or unknown. |
| agent | `block.removed` | `{id}` | Remove the top-level block with that id. An unknown id is a no-op. |
| agent | `reply.created` | `{id, blockId, md}` | Append to the block's reply thread. |
| system | `present.closed` | `{summary?}` | Set closed. Terminal for the reduction: any event ordered after it is a no-op (see below). Recorded with a `system` origin, not `agent`, so it survives the agent-side `watch`/channel `exclude_origin=agent` filter — `watch` terminates on it. |
| human | `decision.created` | `{blockId, verdict, note?}` | Last-write-wins per block. `verdict` is one of `approved`, `rejected`, `cleared`; `cleared` removes the decision, returning the block to undecided. |
| human | `choice.selected` | `{blockId, optionIds}` | Last-write-wins per block. |
| human | `feedback.created` | `{id, blockId, text}` | Append to the block's feedback list. |
| human | `input.submitted` | `{blockId, text}` | Last-write-wins per block. |
| human | `submit` | `{revision}` | Set submitted with the revision. Does not close the document, so rounds continue. |

Post-close events are no-ops, not errors, by design. A human click can race an
agent's close, with the browser POSTing an interaction at the same moment
`present.closed` is appended. Turning that into a hard error would permanently
poison every future reduction of the subject, including fresh-tab replay and
recorded outcomes, so the reducer leaves state unchanged. Rejecting new
interactions after close is enforced at the edges: the REST handler answers 409
and the CLI refuses the append.

The framework appends `channel.changed` presence frames, the cc-interact
Connectivity type emitted with a `system` origin, into the same subject log.
`Reduce` explicitly skips them regardless of origin, so state is unaffected. Every
other unknown event type is still an error.

### Reduced state

`internal/state.State` holds the document plus the keyed human interactions:

```
State = { doc, interactions }
interactions = {
  decisions: { [blockId]: {verdict, note?} },     // last-write-wins
  choices:   { [blockId]: {optionIds} },          // last-write-wins
  inputs:    { [blockId]: {text} },               // last-write-wins
  feedback:  { [blockId]: {id, text}[] },         // append-only
  replies:   { [blockId]: {id, md}[] },           // append-only
  submitted: {value, revision},
  closed:    {value, summary?}
}
```

`Reduce` starts from an empty document with `version 1`, no title, and no blocks, so
a `block.upserted` before any `doc.replaced` appends to it. All five interaction maps
are always present, empty when unused. A fixture's `expected` may omit an empty map,
and the reducer treats the omission as empty.

The fixtures in `internal/state/testdata/*.json` are this contract in executable
form. The Go reducer (`internal/state`) and the TypeScript reducer
(`web/src/reduce.ts`) read the same files.

## REST surface

| Method | Path | Body | Purpose |
|---|---|---|---|
| `POST` | `/api/interactions` | `{subject, nonce, interaction}` | Submit one human interaction. `interaction` is a discriminated union over the human event payloads. The handler validates `blockId` and type against the reduced document, then appends. |
| `POST` | `/api/assets` | image bytes | Store an image content-addressed; returns its `asset:<sha256>`. |
| `GET` | `/assets/{sha}` | none | Fetch a stored asset by its sha256. |

The event stream is `GET /events` over SSE, replaying the log from seq 0.

## DedupKey

The dedup key exists for retry idempotency only:

```
DedupKey = <type>:<blockId>:<browser-nonce>
```

A retried request reuses its nonce, so the append is idempotent. A mind-change is a
new nonce and a new event, and the reducer resolves the winner by last-write-wins on
`seq`, not by dedup. Keying on `<type>:<blockId>:<verdict>` would collapse an
approve, reject, then approve-again sequence into a single event, so the nonce, not
the verdict, is the dedup key.

## Origin filtering

Browser tabs stream `GET /events` unfiltered, so a tab sees its own echoed events and
reconciles its optimistic state against them. The agent-side `watch` and channel use
`exclude_origin=agent`, so the agent is notified of human events and the
`system`-origin `present.closed` close — but never of its own `agent` writes. `watch`
terminates when the `present.closed` frame arrives.
