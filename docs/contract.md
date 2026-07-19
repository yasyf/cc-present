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
Doc = { version: 1, title, intro?, stats?: {label, value}[], submit?: {label, note?}, presentation?: 'focus' | 'board', blocks: Block[] }
```

`blocks` is a flat list of blocks. A `section`, a `card`, any of the built-in
leaf blocks, or a pack block (see Block packs) may appear directly in `blocks`; a
card nests leaf blocks only.

`presentation` is a per-push hint for the client's default view; the viewer's own
toggle overrides it.

## Block schema

| Block | Level | Fields | Notes |
|---|---|---|---|
| `section` | top | `id`, `type`, `title`, `md?` | Header marker with optional prose. |
| `card` | top | `id`, `type`, `title?`, `chips?`, `flagged?`, `status?`, `children` | `chips[].tone` is one of `default`, `flag`, `demo`. `status` is one of `open`, `resolved`, `redrafted` and is agent-owned. `children` nests one level of leaf blocks. |
| `approval` | top or child | `id`, `type`, `prompt?`, `allowFeedback?`, `detail?` | `allowFeedback` defaults to true at render time. `detail` is a `Detail` (see Validation). |
| `choice` | top or child | `id`, `type`, `prompt?`, `multi?`, `options` | `options[]` is `{id, label, hint?, md?, facts?, detail?, recommended?, visual?}`; option ids are unique within the block. `facts` is `Fact[]`, `detail` a `Detail` (see Validation). `recommended` marks the author's suggested pick (at most one per single-select). `visual` is a restricted leaf (`code`, `diagram`, `image`, `diff`, `chart`, `term`, `filetree`, or `record`) with its own doc-unique id. |
| `input` | top or child | `id`, `type`, `label`, `placeholder?`, `multiline?` | Free-text field. |
| `markdown` | top or child | `id`, `type`, `md`, `struck?` | `struck` applies the "was:" treatment. |
| `code` | top or child | `id`, `type`, `lang`, `code`, `title?` | |
| `diff` | top or child | `id`, `type`, `diff`, `title?` | Unified diff text. |
| `diagram` | top or child | `id`, `type`, `kind`, `source`, `title?` | Text-to-diagram block rendered client-side. `kind` is `mermaid`; `source` is at most **8 KiB**. Also usable as an `option.visual`. |
| `chart` | top or child | `id`, `type`, `kind`, `title?`, `unit?`, `categories`, `series` | Structured data rendered client-side as a themed SVG. `kind` is `bar` or `line`. `categories` names the x-axis; `series[]` is `{label, values}` with one finite value per category. Also usable as an `option.visual`. |
| `term` | top or child | `id`, `type`, `command?`, `output`, `title?` | Terminal output panel; `output` keeps its ANSI colors. `command`, when set, renders as a prompt row above it. Also usable as an `option.visual`. |
| `filetree` | top or child | `id`, `type`, `title?`, `entries` | Collapsible file tree. `entries[]` is `{path, badge?, note?}`; directories are implicit from path segments. Also usable as an `option.visual`. |
| `record` | top or child | `id`, `type`, `title?`, `chips?`, `facts`, `links?` | One entity's labeled profile. `facts` is `Fact[]` (see Validation; label required here), `chips` as on `card`, `links[]` is `{label, url}`. Also usable as an `option.visual`. |
| `image` | top or child | `id`, `type`, `src`, `alt`, `caption?` | `src` is `https://…`, `asset:<sha256>`, or `data:…`. |
| `table` | top or child | `id`, `type`, `columns`, `rows` | `columns[]` is `{key, label, align?}` where `align` is `left` or `right`; `rows[]` is a `Record<string,string>` of inline-markdown cells. |
| `progress` | top or child | `id`, `type`, `label`, `value`, `max`, `state?` | `state` is one of `active`, `done`, `error` and is agent-owned. |

### Validation

`Doc.Validate` reports every violation at once, joined one per line, each naming the offending block id (or the envelope field, for document-level checks):

- `version` must be 1; `title` must be non-empty.
- `presentation`, when set, is one of `focus` or `board`.
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
- On `diagram`, `kind` is `mermaid`, `source` is non-empty and at most **8 KiB**, and
  `title`, when set, is single-line.
- On `chart`, `kind` is `bar` or `line`; `categories` (at most **100**) are non-empty,
  single-line, and unique; each of at most **6** series carries a non-empty, single-line,
  unique `label` and exactly one value per category, every value finite (`NaN` and
  `±Inf` are rejected). Negative values are legal — the renderer anchors the value
  axis at 0. `title` and `unit`, when set, are single-line.
- On `term`, `output` is non-empty and at most **32 KiB**; `command` and `title`, when
  set, are single-line.
- On `filetree`, `entries` holds 1 to **200** entries; every `path` is relative,
  slash-separated, and unique, with no empty, `.`, or `..` segment and no trailing
  slash — directories are implicit from path segments, never entries of their own.
  `badge`, when set, is one of `added`, `modified`, `removed`; `note`, when set, is
  single-line.
- On `record`, `facts` holds 1 to **16** entries and every fact carries a `label` — a
  standalone profile has no comparison grid to supply context; `chips` and `links`
  each hold at most **8**. Every `links[].label` is non-empty and single-line, and
  every `links[].url` is `https://…` with a host — `http:`, relative, and
  `javascript:` URLs are all rejected by the one rule. `title`, when set, is
  single-line.
- A chip's `label` — on `card` and `record` alike — is non-empty and single-line.
- A choice option's `visual`, when set, decodes to a `code`, `diagram`, `image`,
  `diff`, `chart`, `term`, `filetree`, or `record` block; any other type is rejected
  at decode time. A visual carries its own doc-unique block id, so it is addressable
  in single-block mode.
- A single-select choice (`multi` unset or false) has at most one `recommended` option.
- The serialized document is at most **1 MiB**.

`Fact` and `Detail` are the option-context shapes — `facts` and `detail` on a choice
option, `detail` on an approval:

```ts
Fact = { label?, value, tone?: 'default' | 'good' | 'warn' | 'bad' }
Detail = { pros?: string[], cons?: string[], md?, mode?: 'inline' | 'modal' }
```

- `fact.value` is required and single-line; `fact.label`, when set, is single-line;
  `fact.tone`, when set, is one of the four tones.
- A non-null `detail` carries at least one of `pros`, `cons`, or `md`; every `pros`
  and `cons` entry is non-empty and single-line.
- `detail.mode`, when set, is `inline` or `modal`; unset renders as `inline`. Inline
  expands in place; modal opens an overlay.

`Validate` takes the installed pack registry and checks each pack block's payload
against its declared schema; an uninstalled dotted type is a violation naming the
block id.

Type dispatch at decode is lenient about packs and strict about everything else:

- An unknown dot-free type is rejected at decode time, before `Validate`, with a
  message naming the offending block id.
- A malformed dotted type — one that does not match `<pack>.<name>` (see Block
  packs) — is also a decode error naming the block id.
- A well-formed dotted type always decodes, into an opaque pack block that
  preserves every field byte for byte, whether or not the pack is installed.
  Whether the type is actually declared by an installed pack is checked only at
  the authoring edges (`start`, `push`, `update-block`, `push --dry-run`), never
  in the reducers — so replay stays total after a pack is uninstalled, and an
  old log renders instead of poisoning every future reduction.

## Block packs

A block pack is a set of plugin-supplied block types: a TOML manifest, a JSON
Schema per block, and one prebuilt ES-module bundle the SPA imports at runtime.
The authoring guide is [packs.md](packs.md); this section is the wire and
discovery contract.

### Namespacing

A pack block's wire type is `<pack>.<name>`, both halves matching
`^[a-z][a-z0-9-]*$`. Built-in types never contain a dot, so the dotted namespace
is reserved for packs permanently. A pack block is a leaf: it may appear at the
top level or as a card child, and never nests children of its own. The block
schema a pack declares validates the entire block object — `id`, `type`, and
every pack-defined field.

### Manifest

`cc-present.toml` at the pack root, decoded strictly: an unknown key is an
error. Every path field is manifest-relative and must resolve inside the pack
root.

| Field | Required | Constraint |
|---|---|---|
| `host_api` | yes | The minimum host API the pack requires — a floor, not an equality. The daemon (host API **2**) loads any pack in `1..2`; a floor above the daemon's version drops the pack at discovery with `host_api <n>, want 1..2`. |
| `name` | yes | Matches `^[a-z][a-z0-9-]*$`, at most 32 characters; the `<pack>` half of every block type. |
| `version` | yes | Non-empty; cache-busts the bundle and styles URLs. |
| `description` | no | Prose shown in `/api/packs` and `pack list`. |
| `entry` | yes | The ES-module bundle; must live under `dist/`. |
| `styles` | no | A stylesheet the SPA injects once per page; must live under `dist/`. |
| `reference` | no | A Markdown fragment describing the blocks for an authoring agent; `pack list` prints its absolute path. |
| `blocks.<name>` | one or more | One table per block type; `<name>` matches the same pattern as `name`. |
| `blocks.<name>.description` | yes | Non-empty prose. |
| `blocks.<name>.schema` | yes | JSON Schema (Draft 2020-12) for the whole block object. |
| `blocks.<name>.interaction` | no | JSON Schema for the human interaction payload; its presence marks the block interactive. |
| `blocks.<name>.examples` | one or more | Example block objects; `pack lint` validates each against the schema. |

Schemas compile with a loader that rejects every external `$ref`, so a schema
can reach neither the network nor the filesystem.

### Discovery

The daemon scans two tiers of pack roots and re-scans on access after a
2-second TTL, so installing a pack needs no restart:

- **Dev** — each directory in the host config's `packDirs`
  (`~/.cc-present/config.json`), in order.
- **Plugin** — each installed Claude plugin (read from
  `$CLAUDE_CONFIG_DIR/plugins/installed_plugins.json`, default `~/.claude`)
  whose `.claude/components/` directory holds a `cc-present.toml`. The
  components directory is the pack root, so a plugin ships exactly one pack.

Discovery is fail-soft per pack. Any violation drops that pack and records the
directory and reason in a `dropped` list, visible in `/api/packs` and
`cc-present pack list`, while every other pack still loads. A manifest error, a
`host_api` outside `1..2`, a missing declared file, and a schema that fails to
compile are each such a violation. The HTTP
response carries only the dropped directory's base name, never its absolute
path.

Same-name conflicts resolve by tier: a dev pack shadows an installed plugin
pack (the plugin copy is dropped with a `shadowed by dev dir` reason), and two
same-name packs in the same tier drop each other — a deliberate mutual drop, so
neither silently wins. A name listed in the config's `disabledPacks` is dropped
unconditionally, beating every other rule.

The manifest and every manifest-declared file are capped at **512 KiB**.

### Serving

`GET /packs/{pack}/{file}` serves only files under the pack's `dist/` subtree,
opened through `os.Root` so no symlink or `..` component can escape the pack
root. An unknown pack, a path outside `dist/`, or a missing file is a 404,
never an SPA fallthrough. Responses are `nosniff`, cached immutably, and
cache-busted by the manifest version (`?v=<version>`); `.js`/`.mjs`, `.css`,
and `.json`/`.map` files get pinned Content-Types so a bundle always loads as
an ES module.

`GET /api/packs` is the SPA's boot manifest:

```ts
PacksResponse = {
  hostApi: 2,
  packs: {
    name, version, description,
    bundle,                 // "/packs/<name>/dist/…?v=<version>"
    styles?,                // same URL shape
    blocks: { type, interactive, schema, interaction? }[]
  }[],
  dropped: { dir, reason }[]
}
```

`schema` and `interaction` are the raw schema documents, inlined. `hostApi`
echoes the daemon's host API version — the ceiling every manifest's `host_api`
floor is checked against.

### Single-block mode

`/p/<ref>?block=<id>` renders one block full-bleed: the same SSE replay and
interaction REST as the board, with no board chrome. It is what the iOS client
loads in a webview per pack block. A block whose enclosing top-level block
belongs to a closed round renders read-only, folded into the same `closed` flag
every interactive block honors. When a `ccPresentHeight` WebKit message handler
is present, the page posts `{type: "height", px}` on every content resize so
the native host can size the webview.

Toasts raised in this mode (`ui.toast`, connection notices) render in-flow
inside `.single-block`; the webview frame is block-height and unscrollable, so
a fixed overlay would clip. A toast's appearance and dismissal each move the
reported height; that resize is the report working.

### Theme tokens

The SPA's palette lives in `web/src/styles/tokens.css` as CSS custom
properties, layered raw-then-alias. The **alias names are the public
contract**: packs consume them through `ui.tokens` on `window.CcPresent` as
`var()` reference strings, never resolved colors, so pack styles re-ink under
theme flips; the iOS client's `BlockPalette` resyncs mechanically from the
tables below. The raw palette is free to churn between releases. The alias
names are frozen.

The raw palette ("Blue Pencil"):

| Name | Light | Dark |
|---|---|---|
| `--paper` | `#FBFBF9` | `#171A21` |
| `--ink` | `#1F2430` | `#E7EAF1` |
| `--graphite` | `#5C6472` | `#98A0AF` |
| `--pencil` | `#3D56C5` | `#91A3F2` |
| `--approve` | `#1E7B4F` | `#5BC489` |
| `--reject` | `#BF3B2F` | `#EE8273` |
| `--hold` | `#8F6400` | `#DCA847` |

The alias contract, with `color-mix()` expressions resolved to sRGB hex:

| Alias | Source | Light | Dark |
|---|---|---|---|
| `--bg` | paper | `#FBFBF9` | `#171A21` |
| `--bg-soft` | mono surface | `#F1F1EF` | `#12141A` |
| `--surface` | card | `#FFFFFF` | `#1E222B` |
| `--surface-raised` | card | `#FFFFFF` | `#1E222B` |
| `--text` | ink | `#1F2430` | `#E7EAF1` |
| `--dim` | graphite | `#5C6472` | `#98A0AF` |
| `--border` | ink 14% over paper | `#DCDDDD` | `#34373E` |
| `--border-strong` | ink 28% over paper | `#BDBFC1` | `#51545B` |
| `--accent` | pencil | `#3D56C5` | `#91A3F2` |
| `--accent-fg` | fixed | `#FFFFFF` | `#14161C` |
| `--ok` | approve | `#1E7B4F` | `#5BC489` |
| `--warn` | hold | `#8F6400` | `#DCA847` |
| `--danger` | reject | `#BF3B2F` | `#EE8273` |
| `--focus-ring` | pencil at 50% alpha | `#3D56C580` | `#91A3F280` |

The non-color aliases: `--radius-sm`/`--radius-md`/`--radius-lg` are 2px / 4px
/ 6px; `--font-prose` is the system-ui stack, `--font-mono` the ui-monospace
stack; `--track-caps` is 0.1em.

### Assets

> **Warning:** a pack block field must not carry an `asset:` URI. The garbage
> collector's reference walk does not see pack-defined fields, so an `asset:`
> reference inside one is unreferenced to the sweep and its bytes are deleted on
> the next close. Use `/packs/<name>/dist/…`, `https:`, or `data:` URLs.

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
| agent | `reply.created` | `{id, blockId, md}` | Append to the block's reply thread. The thread renders under every block type, and the daemon rejects a reply whose `blockId` names no block in the current document — top level or a card child — with an error. |
| agent | `round.started` | `{title?}` | When the round is dirty (a live top-level block is stamped with the current round), snapshot it into `rounds.history` without a `submittedRevision` and advance `rounds.current`; then set the current round's title. When clean, only the title changes — so a `round.started` right after a submit names the round the submit already opened. Never bumps the revision, which counts `doc.replaced` events only. |
| system | `present.closed` | `{summary?}` | Set closed. Terminal for the reduction: any event ordered after it is a no-op (see below). Recorded with a `system` origin, not `agent`, so it survives the agent-side `watch`/channel `exclude_origin=agent` filter — `watch` terminates on it. |
| human | `decision.created` | `{blockId, verdict, note?}` | Last-write-wins per block. `verdict` is one of `approved`, `rejected`, `cleared`; `cleared` removes the decision, returning the block to undecided. |
| human | `choice.selected` | `{blockId, optionIds, other?}` | Last-write-wins per block. `other` is a free-text write-in outside the authored option set; it may stand alone (single-select write-in, empty `optionIds`) or coexist with `optionIds` (multi-select). A re-pick replaces the whole selection, so it drops a prior `other`. |
| human | `feedback.created` | `{id, blockId, text}` | Append to the block's feedback list. Targets any block — approval or choice alike — so a choice carries an append-only note thread beside its selection. |
| human | `input.submitted` | `{blockId, text}` | Last-write-wins per block. |
| human | `pack.interaction` | `{blockId, payload}` | Last-write-wins per block. `payload` is the pack-defined interaction object, stored opaquely — the reducer never inspects its shape. The REST edge validates it against the block's declared interaction schema before appending. |
| human | `submit` | `{revision}` | Set submitted with the revision. When the round is dirty, additionally snapshot the current round into `rounds.history` with `submittedRevision` set, advance `rounds.current`, and clear the title; a clean submit records only the revision. Does not close the document, so rounds continue. The REST plane rejects a revision the log never produced (below 0 or past the current revision). |
| agent | `revising.changed` | `{blockIds, note?}` | Replace the revising working set wholesale (last-write-wins). Each id names a current top-level block; a `block.upserted` or `block.removed` drops its id, and draining the last id clears the shared `note` too. `doc.replaced` clears everything. An empty set with a `note` is the doc-level drafting state, while an empty set with no `note` abandons the announcement. Announcing never stamps rounds (see Live revision). |

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

`internal/state.State` holds the document, the keyed human interactions, the
round partition, and the agent's revising working set:

```
State = { doc, interactions, rounds, revising }
interactions = {
  decisions: { [blockId]: {verdict, note?} },     // last-write-wins
  choices:   { [blockId]: {optionIds, other?} },  // last-write-wins
  inputs:    { [blockId]: {text, round} },        // last-write-wins; round-stamped
  packs:     { [blockId]: {payload} },            // last-write-wins; opaque payload
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
RoundRecord = { number, title?, blocks, decisions, choices, inputs, packs, feedback, submittedRevision? }
revising = { blockIds: string[], note?: string }  // agent's declared working set
```

`Reduce` starts from an empty document with `version 1`, no title, and no blocks, so
a `block.upserted` before any `doc.replaced` appends to it. All six interaction maps
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
inputs, pack interactions, and feedback filtered to those blocks' ids, including
one level of card children. `submittedRevision` is set only when a submit closed the round.

**Revision is not round.** The revision counts `doc.replaced` events only;
`round.started` never bumps it, and a round can span many revisions or none.

## Live revision

`revising` is the agent's declared working set: the top-level block ids it is
rewriting plus an optional shared `note`, a sibling of `interactions` and `rounds`
in the reduced state. It is warn-only — no lock semantics live in the wire or the
reducers; staleness and decay are client presentation.

`revising.changed` replaces the whole set (last-write-wins). Completion is implicit:
a `block.upserted` or `block.removed` drops its id, and draining the last id clears
the shared `note` too, so the common path is announce-once then upsert-N-times with
no terminal call. `doc.replaced` clears everything; `present.closed` no-ops it like
every post-close event. An empty set with a `note` (`blockIds: []` + `note`) is the
doc-level drafting state — work with no existing block to mark yet — cleared by the
next `block.upserted`. An empty set with no `note` abandons the announcement.

`revising.changed` never stamps `blockRounds`: announcing is not touching. The
reducers stay pure — no wall clock — so a mark persists until an upsert, removal, or
replacement clears it; client-side decay of a stale mark is presentation only.

The daemon edge validates each announced id names a current top-level block (child
ids and unknown ids are rejected); an empty set skips id validation. The agent sends
it with `cc-present revising [blockId...] [--note "…"]` — a bare call abandons.

| Method | Path | Body | Purpose |
|---|---|---|---|
| `POST` | `/api/interactions` | `{subject, nonce, interaction}` | Submit one human interaction. `interaction` is a discriminated union over the human event payloads. The handler validates `blockId` and type against the reduced document, then appends. The body is capped at **256 KiB**. Requires `Content-Type: application/json`; anything else is rejected with 415 (CSRF hardening — a hostile localhost page cannot send it preflight-free). |
| `POST` | `/api/assets` | image bytes | Store an image content-addressed; returns its `asset:<sha256>`. Requires `Content-Type: application/octet-stream` or `image/*` (neither is CORS-simple — same CSRF hardening); a body that does not sniff as an image is rejected with 415. |
| `GET` | `/assets/{sha}` | none | Fetch a stored asset by its sha256. |
| `GET` | `/api/sessions` | none | List the open artifacts, most-recently-updated first (see Session listing). |
| `GET` | `/api/packs` | none | List the installed block packs with inlined schemas, plus the dropped candidates (see Block packs). |
| `GET` | `/packs/{pack}/{file}` | none | Fetch a pack's prebuilt bundle asset from its `dist/` subtree (see Block packs). |

A block-scoped interaction (`decision.created`, `choice.selected`,
`feedback.created`, `input.submitted`, `pack.interaction`) whose enclosing
top-level block belongs
to a closed round is rejected with `400 block "<id>" belongs to closed round
<n>`. `submit` is exempt from the round guard — it is what closes a round.

### The event stream

The event stream is `GET /events` over SSE. A new connection replays the log
as unnamed frames, `id:` carrying the seq and `data:` the payload,
self-describing through the injected `"type"` (see Event taxonomy). Replay
starts at seq 0, or after the client's `Last-Event-ID` on a resume. Once the
replay flush completes, the stream emits one named frame, `event: caught-up`
with `data: {"seq": N}`, where `N` is the connection's replay high-water seq.
The marker fires exactly once per connection, after a zero-length replay too
when a resume is already at the head. It is stream-plane only: never appended
to the log, never entering either reducer.

`caught-up` marks the replay/live boundary. A client that reacts to activity
gates live-only behavior on it: the SPA toasts only on frames past the marker,
so a tab remount replays history silently. Clients ignore named events they do
not recognize, which keeps future stream-plane markers backward-compatible.

## Authentication

The HTTP plane binds per `~/.cc-present/config.json`: an absent file or empty
`bind` is loopback-only `127.0.0.1`, and `"bind": "0.0.0.0"` exposes the plane to
the LAN — those are the two valid values. The bearer token lives at
`~/.cc-present/token` — 32 crypto-random bytes as lowercase hex, written 0600.
`cc-present pair` writes both; an absent or empty token disables the token path.
The daemon refuses to start the HTTP plane on a non-loopback bind with no token
and no synckit trust; it never serves an off-host request unauthenticated.

The auth middleware wraps the whole plane: `GET /events`, the REST routes,
`/assets/{sha}`, the pack routes (`/api/packs` and `/packs/{pack}/{file}`), and
the SPA. A request is accepted on exactly one of three paths:

- **Loopback peer.** An unzoned loopback TCP peer passes without a token, under
  the `Origin`-header gate below. A v4-in-v6 `::ffff:127.0.0.1` counts as
  loopback; a zoned `[::1%zone]` does not.
- **Trusted peer.** A non-loopback TCP peer whose IP belongs to a machine in the
  user's synckit mesh (below) passes without a token, under the same
  `Origin`-header gate.
- **Bearer token.** Any other request must carry the token, in an
  `Authorization: Bearer <token>` header or the `?token=` query fallback that
  browser `EventSource` needs, since it cannot set headers. The header wins when
  both are present. The comparison is constant-time; anything else is 401. A
  `?token=` parameter is stripped from the request before any handler runs, so
  it never reaches a redirect `Location` or access log.

### The Origin-header gate

Both no-token paths check the request's `Origin` header, so a foreign web page
cannot drive the daemon through a browser running on an admitted machine. The
`Origin` must be absent (a native client), name `localhost` or a loopback IP, or
name a host the daemon is itself served under — its own MagicDNS name or one of
its own tailnet IPs, IP literals compared canonically. Ports in `Origin` values
are ignored. An absent `Origin` is additionally rejected when the request
carries `Sec-Fetch-Site: cross-site`, which closes originless cross-site GET
navigations; native clients never send that header and are unaffected. A foreign
page in a browser on any machine, trusted or not, must present the token.

### Synckit mesh trust

Trust is automatic: when synckit's state file (`~/.config/synckit/state.json`)
exists at daemon start, every host registered in the mesh is trusted. Registered
targets (`user@host` strings) resolve to tailnet IPs via `tailscale status
--json`, accepted only while `BackendState` is `Running`; a parse anomaly
anywhere rejects the whole snapshot. The trust set refreshes on a 30-second TTL,
so mid-session mesh changes apply without a daemon restart, and fails closed to
empty on any read, exec, or parse error. Bare-LAN registry entries resolve to no
tailnet IPs and are not network-trusted. Offline peers stay trusted — the set is
identity, not liveness. Trust is machine-level: any process on a trusted machine
can reach the plane.

With trust on and a loopback primary bind, the daemon additionally binds each of
its own tailnet IPs, best-effort: an unbindable address is skipped with a
warning, and loopback always serves. Binding is dynamic — a reconcile pass runs
every 30 seconds, so a daemon started while tailscale was down grows its tailnet
legs within one pass of `tailscale up`, no restart needed. Legs are never
pruned: a leg whose address vanished is inert (auth is per-request), and the
same socket resumes if the address returns. The bound legs are recorded in the
handshake's `extra_addrs` — rewritten atomically as each leg binds, the source
of truth for where the plane listens. An extra leg can land on a different port
than the primary when the port-reuse hint is taken on one interface only, and
ports are sticky across restarts: each bind retries its previous port first,
falling to ephemeral only on conflict. One restart still required: synckit
state created after daemon start is not detected — trust hooks fix at daemon
construction. With `pair`'s `0.0.0.0` bind the primary already covers the
tailnet and no extra legs are bound.

`start` and `push` results carry the composed display URLs for the live legs
(`tailnetUrls`): `https://` on the daemon's MagicDNS name when tailscale
publishes one and the daemon holds a certificate for it, else `http://` on the
bare machine label (the MagicDNS name's first label, e.g. `yasyf-home`) — both
deduped by port, so v4 and v6 legs on one port yield one URL. Raw tailnet IPs
over `http` appear only when no usable name exists: tailscale down, or the
name quarantined by a DNS collision. An `http` URL on the full MagicDNS name
is never composed: `ts.net` is on the browser HSTS-preload list, so a browser
rewrites such a URL to `https://` before connecting and the navigation can
only fail. The bare label sits outside the preload list, resolves through
MagicDNS search domains on every tailnet device, and is a trusted origin like
the full name; WireGuard encrypts the path either way.

Each tailnet leg serves both protocols on its one port by sniffing the first
byte of every connection: a TLS ClientHello is terminated with the daemon's
certificate, anything else is served as plain HTTP. Mesh clients keep dialing
the `extra_addrs` IPs over `http` unchanged; browsers land on the `https` name
URL. The certificate comes from `tailscale cert`, which needs the tailnet's
HTTPS-certificates feature enabled, and is minted for the MagicDNS name
asynchronously at daemon start, refreshed by the reconcile pass as expiry
nears, and swapped in without rebinding. While no certificate is held — feature disabled, tailscale
down, first mint still in flight — TLS handshakes on the legs fail and the
composed URLs fall back to the label form (or IPs when no name is usable).

`cc-present trust` is a read-only inspector: it reports whether synckit state
was detected, each registered host with its resolved tailnet IPs (or that it is
not network-trusted), and the live listener addresses when the daemon is
reachable.

Dial the plane directly, never through a reverse proxy. `tailscale serve` and
Funnel deliver every proxied request from a loopback TCP peer, which rides the
loopback path and defeats both the token and the trust check.

## Health

`GET /api/health` returns `{"version":"<daemon version>"}`. This is the one
non-vacuous liveness probe: the SPA fallback answers unmatched paths with a 200
HTML shell, so a bare status-code check proves nothing — expect the JSON body.
Unknown `/api/*` paths return 404, never the shell.

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
