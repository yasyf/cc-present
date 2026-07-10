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
| agent | `round.started` | `{title?}` | When the round is dirty (a live top-level block is stamped with the current round), snapshot it into `rounds.history` without a `submittedRevision` and advance `rounds.current`; then set the current round's title. When clean, only the title changes — so a `round.started` right after a submit names the round the submit already opened. Never bumps the revision, which counts `doc.replaced` events only. |
| system | `present.closed` | `{summary?}` | Set closed. Terminal for the reduction: any event ordered after it is a no-op (see below). Recorded with a `system` origin, not `agent`, so it survives the agent-side `watch`/channel `exclude_origin=agent` filter — `watch` terminates on it. |
| human | `decision.created` | `{blockId, verdict, note?}` | Last-write-wins per block. `verdict` is one of `approved`, `rejected`, `cleared`; `cleared` removes the decision, returning the block to undecided. |
| human | `choice.selected` | `{blockId, optionIds}` | Last-write-wins per block. |
| human | `feedback.created` | `{id, blockId, text}` | Append to the block's feedback list. |
| human | `input.submitted` | `{blockId, text}` | Last-write-wins per block. |
| human | `submit` | `{revision}` | Set submitted with the revision. When the round is dirty, additionally snapshot the current round into `rounds.history` with `submittedRevision` set, advance `rounds.current`, and clear the title; a clean submit records only the revision. Does not close the document, so rounds continue. The REST plane rejects a revision the log never produced (below 0 or past the current revision). |

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

`internal/state.State` holds the document, the keyed human interactions, and the
round partition:

```
State = { doc, interactions, rounds }
interactions = {
  decisions: { [blockId]: {verdict, note?} },     // last-write-wins
  choices:   { [blockId]: {optionIds} },          // last-write-wins
  inputs:    { [blockId]: {text, round} },        // last-write-wins; round-stamped
  feedback:  { [blockId]: {id, text}[] },         // append-only
  replies:   { [blockId]: {id, md}[] },           // append-only
  submitted: {value, revision},
  closed:    {value, summary?}
}
rounds = {
  current: number,                                // 1-based
  currentTitle?: string,
  blockRounds: { [topLevelBlockId]: number },     // round of the block's last agent touch
  history: RoundRecord[]                          // closed rounds, ascending
}
RoundRecord = { number, title?, blocks, decisions, choices, inputs, feedback, submittedRevision? }
```

`Reduce` starts from an empty document with `version 1`, no title, and no blocks, so
a `block.upserted` before any `doc.replaced` appends to it. All five interaction maps
are always present, empty when unused. A fixture's `expected` may omit an empty map,
and the reducer treats the omission as empty.

The fixtures in `internal/state/testdata/*.json` are this contract in executable
form. The Go reducer (`internal/state`) and the TypeScript reducer
(`web/src/reduce.ts`) read the same files.

## Rounds

A round is reducer-derived: there is no round entity in the store, only the
`round.started` event and the `blockRounds` stamps the reducer maintains.

**Boundaries.** A round closes on a human `submit` or an agent `round.started`,
and only when it is dirty — a live top-level block is stamped with the current
round. A clean submit records only the revision; a clean `round.started` only
retitles the current round.

**Carry-forward.** `block.upserted` stamps its block into the current round —
re-upserting a block, even byte-identical, is how it stays actionable across a
boundary. `doc.replaced` stamps the entire new document. Closing a round never
prunes `doc.blocks`: untouched blocks stay in the document, stamped with the
closed round, and the REST plane rejects interactions on them (below).

**Snapshots.** Each closed round lands in `history` as a `RoundRecord`: deep
copies of the blocks stamped with that round, plus the decisions, choices,
inputs, and feedback filtered to those blocks' ids, including one level of card
children. `submittedRevision` is set only when a submit closed the round.

**Revision is not round.** The revision counts `doc.replaced` events only;
`round.started` never bumps it, and a round can span many revisions or none.

## REST surface

| Method | Path | Body | Purpose |
|---|---|---|---|
| `POST` | `/api/interactions` | `{subject, nonce, interaction}` | Submit one human interaction. `interaction` is a discriminated union over the human event payloads. The handler validates `blockId` and type against the reduced document, then appends. |
| `POST` | `/api/assets` | image bytes | Store an image content-addressed; returns its `asset:<sha256>`. A body that does not sniff as an image is rejected with 415. |
| `GET` | `/assets/{sha}` | none | Fetch a stored asset by its sha256. |
| `GET` | `/api/sessions` | none | List the open artifacts, most-recently-updated first (see Session listing). |

A block-scoped interaction (`decision.created`, `choice.selected`,
`feedback.created`, `input.submitted`) whose enclosing top-level block belongs
to a closed round is rejected with `400 block "<id>" belongs to closed round
<n>`. `submit` is exempt from the round guard — it is what closes a round.

The event stream is `GET /events` over SSE, replaying the log from seq 0.

## Authentication

The HTTP plane binds per `~/.cc-present/config.json`: an absent file or empty
`bind` is loopback-only `127.0.0.1`, and `"bind": "0.0.0.0"` exposes the plane to
the LAN. The bearer token lives at `~/.cc-present/token` — 32 crypto-random bytes
as lowercase hex, written 0600. `cc-present pair` writes both; an absent or empty
token disables the check entirely. The daemon refuses to start the HTTP plane
on a non-loopback bind with no token; it never serves an off-host request
unauthenticated.

The auth middleware wraps the whole plane: `GET /events`, the REST routes,
`/assets/{sha}`, and the SPA.

- A loopback request always passes, token or no token. The check reads the
  immediate TCP peer (a v4-in-v6 `::ffff:127.0.0.1` counts as loopback), so a
  reverse proxy in front of the daemon arrives as a loopback peer and bypasses
  the token — the plane is dialed directly, never reverse-proxied.
- A non-loopback request must carry the token, in an `Authorization: Bearer
  <token>` header or the `?token=` query fallback that browser `EventSource`
  needs, since it cannot set headers. The header wins when both are present. The
  comparison is constant-time; anything else is 401. A `?token=` parameter is
  stripped from the request before any handler runs, so it never reaches a
  redirect `Location` or access log.

## Session listing

`GET /api/sessions` returns the open artifacts, most-recently-updated first, so a
paired client can pick one to open:

```ts
SessionSummary = { subject, slug, title, status, updatedAt, revision }
```

`title` is the reduced document's title; `status` is always `open`, since closed
artifacts are not listed. `updatedAt` is an RFC 3339 string. `revision` counts
the artifact's `doc.replaced` events — 0 for a document never replaced.

## Discovery

A daemon bound non-loopback advertises the HTTP plane over mDNS; a loopback bind
advertises nothing. The TXT records carry the protocol version and the host
name, never the token.

| Field | Value |
|---|---|
| Service type | `_cc-present._tcp` |
| Domain | `local.` |
| Instance name | the host name |
| Port | the HTTP plane's bound port |
| TXT records | `v=1`, `name=<hostname>` |

`cc-present pair` carries the secret the advertisement omits. It writes
`"bind": "0.0.0.0"` to the host config, ensures the token, restarts the daemon
when its effective bind differs or the token changed (open browser tabs
reconnect on their own), and prints a terminal QR code plus the same payload as
copyable text:

```ts
PairPayload = { v: 1, url: "http://<lan-ip>:<port>", token: string }
```

`v` is the pairing-payload schema version. `--reset-token` regenerates the token
before pairing; `--off` rebinds the daemon to loopback, taking the plane off the
LAN.

## Asset garbage collection

The content-addressed asset store is swept on every `close`. After the
`present.closed` append and the status flip succeed, the daemon reduces each
**open** subject's log, collects every `asset:<sha256>` its live document
references — walking top-level blocks and card children — and deletes every
stored file that is both unreferenced and older than a **15-minute mtime grace
window**.

- Assets referenced only by closed subjects are collectable, so an open browser
  tab of a closed artifact loses its images on reload by design. The log and its
  reduction are untouched; only the bytes behind `GET /assets/{sha}` go away.
- The grace window guards the race where a fresh upload lands between the sweep's
  subject enumeration and its deletion pass: a file written within the last 15
  minutes survives even while still unreferenced.
- A sweep failure surfaces in the `close` reply as an error, but the close has
  already taken effect — the append and status flip happen first.

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
