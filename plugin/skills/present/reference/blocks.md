# Block reference

The document is JSON, validated by `push --dry-run` before anything touches the daemon. This page is the full authoring surface: the envelope, every block type, the composition rules, and a worked document.

## Document envelope

```json
{
  "version": 1,
  "title": "26 openers, redrafted",
  "intro": "Optional markdown shown under the title.",
  "stats": [ { "label": "repos drafted", "value": "26" } ],
  "submit": { "label": "Approve openers", "note": "Approving green-lights the retrofit." },
  "blocks": []
}
```

- `version` is always `1`; `title` is required and seeds the URL slug.
- `stats` renders a headline number row — use it for scale ("26 repos", "3 flagged").
- `submit` labels the submit bar; `note` states exactly what submitting commits the human to. Omit `submit` for a purely informational page.
- `presentation` (optional) is `focus` or `board` — a per-push hint for the default view. The viewer's own toggle overrides it.

## Composition rules

- **Cards are decision units.** Each card carries one thing to sign off: the content blocks, then the `choice`/`approval` that decides it.
- **Sections are tiers.** A `section` is a header marker grouping the cards after it ("Flagged — fix first", "Tier A").
- **One nesting level.** `blocks` is a flat list of sections, cards, and leaves; a card nests leaf blocks only — never a section or another card.
- **Ids are globally unique kebab-case**, card children included. Choice option ids are unique within their block.
- **Agent-owned display state** rides the document: `card.status`, `progress.state`, `chips`, `struck` markdown. Human verdicts live outside it — re-upserting a block never clobbers a decision.
- **Rounds partition the board over time.** A submit on a board you've touched closes the round: those blocks collapse into a read-only "Round N" group, and only blocks you upsert afterward render live. Re-upsert a block (even unchanged) to carry it into the new round.

### Content density

The UI clamps long prose behind "Show more" — markdown blocks at ~10 lines, approval replies at 4, option bodies at 3. Write for the fold. Labels carry the pick, `hint` the one-line why, `md` only the short must-read lede — the full drill-down rides `detail` (next section).

```json
{ "id": "sqlite", "label": "Use SQLite because it gives us a single-file deployment with zero operational overhead, WAL supports our single-writer daemon, and migrating to Postgres later stays easy since all SQL lives in the store package" }
```

```json
{ "id": "sqlite", "label": "SQLite, single writer", "hint": "zero ops; rules out horizontal scaling", "md": "WAL fits the per-repo daemon; all SQL stays in `store`, so a later Postgres move is contained." }
```

The first buries the tradeoff in a label nobody scans; the second reads at a glance and keeps the detail one clamp away.

### Give the whole picture

Never present an option blind. Every option carries enough for the human to decide without a follow-up question: the tradeoff, the numbers, the pros and the cons. If someone scanning the board couldn't say why they'd pick an option over its neighbor, you've under-informed — clamping keeps the row scannable; it is not license to omit. Split context by tier, don't drop it:

- `label` is the pick (~6 words); `hint` the one-line why beside it.
- `facts` are the comparable numbers — `{value, label?, tone?}`, an aligned cluster that reads column-to-column across options. Set `tone` (`good`/`warn`/`bad`) to flag the outlier.
- `detail` is the drill-down — `pros`, `cons`, a full `md` — opened on demand, so depth never costs up-front clarity. Fill it: an empty `detail`, or a throwaway "My pick.", is the anti-pattern this replaces.
- `md` holds only a short must-read lede; everything longer belongs in `detail`.

## Composing for focus mode

A board with any decision unit opens in focus mode by default: one step at a time, in document order. The same document serves both views — these rules make it read well as a deck.

- **Each card is one focus step.** "Cards are decision units" is literal: the card renders as the step body and its controls decide it.
- **Doc order is deck order.** Order cards by decision priority — the human meets them one at a time.
- **A card fits one screen.** The content-density clamps above are the budget; an overlong card splits into multiple cards, same as on the board.
- **Context attaches forward.** A top-level run of content blocks (markdown, code, diff, diagram, image, table, progress, chart, term, filetree, record) becomes the lead-in of the next card or decidable; a trailing run is its own read-only step. Put context immediately before the decision it informs — a run cut off by a section header turns into a standalone read-only step instead of attaching.
- **Sections are never steps.** A section surfaces as the tier label in the deck's progress header.
- **A lone approval swipes.** A step whose only decidable is an approval takes swipe-to-decide — right approves, left rejects. A card with several decidables decides by its controls only.
- **Content-only boards stay boards.** A push with no decision unit opens as the classic board; set `presentation` to override either default.

## Size caps

| Cap | Limit |
|---|---|
| Serialized document | 1 MiB |
| `data:` URI in `image.src` | 32 KiB |
| Local image file (inlined to an `asset:` ref) | 5 MiB |
| `diagram.source` | 8 KiB |
| `chart` | 6 series, 100 categories |
| `term.output` | 32 KiB |
| `filetree.entries` | 200 |
| `record` | 16 facts, 8 chips, 8 links |
| `draft.text` | 64 KiB |
| `triage.items` | 50 |

## Block types

### `section` — tier header (top level only)

```json
{ "id": "sec-flagged", "type": "section", "title": "Flagged — corrected before approval", "md": "Optional prose under the header." }
```

### `card` — decision unit (top level only)

```json
{
  "id": "card-slop-cop",
  "type": "card",
  "title": "slop-cop",
  "summary": "Opener redrafted; install command corrected.",
  "chips": [ { "label": "plugin" }, { "label": "corrected", "tone": "flag" } ],
  "flagged": true,
  "status": "open",
  "children": []
}
```

`chips[].tone` is `default`, `flag`, or `demo`. `status` is agent-owned: `open`, `resolved` (verdict landed), or `redrafted` (you replaced the content after feedback). `flagged` marks a card needing extra scrutiny. `summary` is an optional one-line inline-markdown lede rendered dim under the title; a newline in it fails validation, naming the block id.

### `approval` — approve/reject verdict

```json
{
  "id": "opener-approval",
  "type": "approval",
  "prompt": "Approve this opener?",
  "allowFeedback": true,
  "detail": {
    "pros": [ "Shortest opener of the round", "Doubles as the About text unchanged" ],
    "cons": [ "Tone risks reading as a gimmick" ],
    "md": "The full rationale the approver opens before deciding."
  }
}
```

`allowFeedback` defaults to true; set `false` to forbid free-text feedback and verdict notes on this block. Optional `detail` carries the pros, cons, and why for the single approve/reject gate — same shape and drill-down as a choice option's `detail` (below).

### `choice` — pick from options

```json
{
  "id": "opener-alts",
  "type": "choice",
  "prompt": "My pick is selected. Prefer an alternate?",
  "multi": false,
  "options": [
    {
      "id": "pick",
      "label": "Never ship 'delve' again.",
      "hint": "my pick — shortest, boldest",
      "facts": [
        { "label": "words", "value": "4", "tone": "good" },
        { "label": "frame", "value": "command" }
      ],
      "detail": {
        "pros": [ "The one word every reader has caught an LLM using", "Doubles as the About text unchanged" ],
        "cons": [ "Opaque to anyone who hasn't seen LLM prose" ],
        "md": "One concrete tell beats describing the category — 'delve' makes the promise legible on sight."
      }
    },
    {
      "id": "alt-a",
      "label": "AI-written, without the AI accent.",
      "hint": "safer, less memorable",
      "facts": [
        { "label": "words", "value": "5" },
        { "label": "frame", "value": "outcome" }
      ],
      "detail": {
        "pros": [ "Legible with zero context" ],
        "cons": [ "Concedes 'AI-written' in the first two words" ],
        "mode": "modal"
      }
    }
  ]
}
```

An option splits across two tiers. Always visible in the row: `label` (the pick itself, ~6 words), `hint` (a one-line inline-markdown qualifier rendered dim beside the label — the why or the cost), `facts` (a cluster of `{value, label?, tone?}` — comparable numbers aligned so options read column-to-column; `tone` is `default`, `good`, `warn`, or `bad`), and `md` (a short must-read lede, clamped at ~3 lines). Behind the "Details" affordance: `detail` — `pros`, `cons`, and a full `md` body. `detail.mode` picks the drill-down surface: `inline` (the default) expands in place and joins the board's expand-all; `modal` opens an overlay.

The `md`-vs-`detail.md` line: `md` is what the human must read before deciding; `detail.md` is everything they'd want once they drill in. A newline in `hint`, a fact `label` or `value`, or a `pros`/`cons` entry fails validation, naming the block id; a non-null `detail` needs at least one of `pros`, `cons`, or `md`.

`recommended: true` marks your suggested pick: the option carries a stamp badge, and it replaces the free-text "Recommended —" `hint` prefix that used to signal the same thing. At most one option per single-select choice may set it — a second one fails validation. A multi-select choice allows any number.

`visual` attaches one visual block to an option — a `code`, `diagram`, `image`, `diff`, `chart`, `term`, `filetree`, or `record`, each with its own doc-unique id. The deck renders it in the step's visual stage as the option becomes active, so it stands in for a prose tier instead of crowding the row; on the board it rides inside the option's Details. Pick the type that shows the difference between the options: `code` for the shape of what the option produces, `diagram` for a flow or structure that differs across them, `diff` for a before/after edit, `image` for a rendered mockup or screenshot, `chart` for magnitudes the eye should rank, `term` for what running the option prints, `filetree` for the files it touches, `record` for the entity behind it. Any other block type is rejected at decode, naming the option.

Facts earn the comparison grid by lining up. Give every option the same fact labels in the same order — matched labels render as a column-to-column grid the eye reads across options; a single mismatched label drops the whole choice back to per-option chips, silently.

### `input` — free text

```json
{ "id": "extra-notes", "type": "input", "label": "Anything I missed?", "placeholder": "Optional", "multiline": true }
```

An input carried into a new round renders empty with a dim "last round: …" hint showing the previous entry; the old text stays read-only inside the collapsed round. Fields are fresh each round automatically — never ask the human to clear one.

### `draft` — document for line comments

```json
{
  "id": "notes-draft",
  "type": "draft",
  "lang": "markdown",
  "title": "Release notes, first pass",
  "text": "## v0.14.0\n\ncc-present grows two review primitives.\n\nThe draft block renders a document as numbered lines."
}
```

Reach for `draft` when the deliverable is the text itself — a prose draft, a config, a file — and you want margin notes instead of a verdict. It renders as numbered source lines (`lang` picks the syntax highlighting; use `markdown` for prose). The human selects a line or range and attaches a note; each note streams back as an `annotation.created` event carrying a content anchor and a `quote` of the anchored lines. Annotating never gates Submit; pair the draft with an `approval` when you need a sign-off.

Redraft by upserting the **same block id** with new `text`: annotations persist and re-anchor by content, a note whose lines moved shows its shift, and a note whose lines vanished lands in a detached list instead of disappearing. When you read `outcomes`, each annotation is `{id, anchor, text, quote}` — resolve `anchor` against your current text to place it, and fall back to `quote` (the lines as the human saw them) when it no longer resolves.

### `triage` — per-item accept/reject

```json
{
  "id": "changelog-triage",
  "type": "triage",
  "prompt": "Keep or cut each entry?",
  "items": [
    { "id": "entry-anchors", "label": "Line-anchor port", "hint": "new wire contract", "md": "Three parallel implementations with one conformance corpus." },
    { "id": "entry-bun", "label": "bun toolchain note", "facts": [ { "label": "Scope", "value": "web/ only" } ] }
  ]
}
```

Reach for `triage` when one decision decomposes into several independent accept/reject calls — a list of changelog entries, findings, or candidates to keep or cut. Each item carries an independent approve/reject verdict plus an optional note (`allowNotes: false` forbids notes); items take the choice-option body shape — `md`, `facts`, `detail`, `visual` — minus `recommended`. Verdicts stream back as `triage.decided` partial-map merges, and `outcomes` shows `{[itemId]: {verdict, note?}}` per block.

The block counts **once** in the submit tally, and only when every item has a verdict — a 20-item triage is one undecided step until the last call lands. Closed-round headers count item verdicts individually. Accept all / Reject all covers **every** item, including ones already carrying the opposite verdict; individual flips afterward override.

### `markdown` — prose

```json
{ "id": "opener-was", "type": "markdown", "md": "The old opener.", "struck": true }
```

`struck` renders the "was:" treatment — the before in a before/after pair.

### `code`

```json
{ "id": "get-started", "type": "code", "lang": "bash", "code": "brew install yasyf/tap/slop-cop", "title": "Get started" }
```

### `diff` — unified diff text

```json
{ "id": "readme-diff", "type": "diff", "title": "README.md", "diff": "--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old opener\n+new opener" }
```

### `diagram` — text-to-diagram

```json
{ "id": "flow-streaming", "type": "diagram", "kind": "mermaid", "title": "Streaming path", "source": "graph LR\n  agent[Agent] --> daemon[Daemon]\n  daemon --> browser[Browser]" }
```

`kind` is `mermaid`; `source` is the diagram text (8 KiB cap); `title`, when set, is single-line. The client renders it — mermaid loads lazily, and the diagram is inked in the board's theme and re-inked on a light/dark flip. A diagram works at the top level, inside a card, or as an `option.visual`.

Keep the source scannable — a diagram earns its place by reading at a glance, not by encoding the whole design:

- **At most ~12 nodes.** Past that it stops being legible; cut to the decision-relevant path or split it across diagrams.
- **Plain `graph LR` or `graph TD`.** Left-to-right for a pipeline, top-down for a hierarchy. Skip the specialized mermaid diagram kinds.
- **No inline styling.** Leave out `style`, `classDef`, and `class` directives and any HTML in labels. The client owns ink and theming and re-inks on a theme flip, so author colors fight it and read inconsistently.
- **A visual replaces a prose tier; it never repeats one.** When the diagram carries what a neighboring markdown block already says, cut the prose — don't caption the picture with its own transcript.

### `chart` — data to picture

```json
{
  "id": "latency-chart",
  "type": "chart",
  "kind": "bar",
  "title": "p99 latency by backend",
  "unit": "ms",
  "categories": [ "SQLite", "Postgres", "DynamoDB" ],
  "series": [
    { "label": "read", "values": [ 4, 11, 38 ] },
    { "label": "write", "values": [ 9, 14, 41 ] }
  ]
}
```

`kind` is `bar` or `line`. `categories` names the x-axis; each series carries exactly one finite value per category — each value is 0 or of magnitude 1e-15 to 1e15 (6 series and 100 categories max). The client renders a themed SVG — series colors derive from the board accent and re-ink on a light/dark flip — and `unit` rides the formatted values. A chart works at the top level, inside a card, or as an `option.visual` — it is the default picture of a quantitative tradeoff.

- **Chart over table when magnitudes are comparable.** Numbers the eye should rank at a glance go in a chart; exact values to look up, or mixed units, stay a `table`.
- **At most ~12 categories.** Validation allows 100; legibility doesn't. Past that, aggregate or split.
- **No styling knobs.** The client owns ink and theming, same as `diagram` — there is nothing to set, so don't encode color in labels.
- Negative values are fine; the value axis always anchors at 0.

### `term` — command output

```json
{
  "id": "test-run",
  "type": "term",
  "command": "go test ./internal/doc/",
  "title": "The failing case",
  "output": "--- FAIL: TestValidate (0.00s)\n    doc_test.go:88: want 3 violations, got 2\nFAIL\nFAIL\tcc-present/internal/doc\t0.41s"
}
```

`output` is what the command printed, ANSI colors preserved (32 KiB cap); `command`, when set, renders as a prompt row above it. Both `command` and `title` are single-line.

- **`code` is source, `term` is output.** A snippet to read or copy is a `code` block; what running it printed is a `term`.
- **Trim to the decision-relevant tail.** The failing test and its message, the error and its cause — not the full scrollback.

> **Never board secrets.** Command output loves to embed tokens, signed URLs, and env dumps — scrub before pasting into `output`.

### `filetree` — paths to a tree

```json
{
  "id": "migration-tree",
  "type": "filetree",
  "title": "Files this migration touches",
  "entries": [
    { "path": "internal/doc/doc.go", "badge": "modified", "note": "4 new validators" },
    { "path": "internal/doc/registry.go", "badge": "modified" },
    { "path": "web/src/components/ChartView.tsx", "badge": "added" },
    { "path": "web/src/components/LegacyChart.tsx", "badge": "removed" }
  ]
}
```

List files as relative slash paths (200-entry cap, at most 32 segments deep; a leading `/`, a drive letter, or a backslash is rejected) and the client builds the collapsible tree — directories are implicit from path segments, never entries of their own. `badge` is `added`, `modified`, or `removed`, rendered as a tone chip; `note` is a dim single-line annotation. Keep the tree at most ~4 levels deep — scope to the subtree that matters instead of rooting at the repo.

### `record` — one entity's profile

```json
{
  "id": "flight-nh7",
  "type": "record",
  "title": "NH 7 — SFO → HND",
  "chips": [ { "label": "Nonstop" }, { "label": "Overnight", "tone": "flag" } ],
  "facts": [
    { "label": "Cabin", "value": "Business", "tone": "good" },
    { "label": "Miles", "value": "75k Aeroplan" },
    { "label": "Taxes", "value": "$112", "tone": "warn" },
    { "label": "Departs", "value": "17:05" }
  ],
  "links": [ { "label": "Book on Aeroplan", "url": "https://www.aircanada.com/aeroplan" } ]
}
```

A record is one entity's labeled profile: 1–16 `facts` (every fact carries a `label` here — a standalone profile has no comparison grid to supply context), up to 8 `chips` (same shape and tones as on `card`), and up to 8 `links` (`{label, url}`, https URLs only).

Three shapes carry labeled values; pick by what's being compared:

- **Option `facts`** — numbers aligned *across* options; the comparison grid reads column-to-column.
- **`table`** — many entities sharing columns.
- **`record`** — one entity in full; the thing itself, not a comparison.

### `image`

```json
{ "id": "banner-preview", "type": "image", "src": "docs/assets/banner.png", "alt": "Repo banner, dark variant", "caption": "Generated banner" }
```

`src` takes four forms: `https://…`, `asset:<64-hex sha256>`, `data:…` (32 KiB cap), or a **local file path** — `push`, `update-block`, and `start --doc` read the file (5 MiB cap), store it content-addressed on the daemon, and rewrite the src to its `asset:` ref automatically.

### `table`

```json
{
  "id": "rollout",
  "type": "table",
  "columns": [ { "key": "repo", "label": "Repo" }, { "key": "n", "label": "Stars", "align": "right" } ],
  "rows": [ { "repo": "`slop-cop`", "n": "412" }, { "repo": "`cc-review`", "n": "388" } ]
}
```

Cells are inline markdown, keyed by column `key`. `align` is `left` or `right`.

### `progress` — live progress bar

```json
{ "id": "retrofit-progress", "type": "progress", "label": "READMEs rewritten", "value": 3, "max": 26, "state": "active" }
```

`state` is agent-owned: `active`, `done`, or `error`. Upsert the block as work advances — a natural fit for showing background progress while the human reviews.

Every leaf type (`approval` through `progress`) works both at the top level and inside a card.

## Validation

`push --dry-run FILE` runs the full check offline and prints every violation at once, one per line, each naming its offending block id — an unknown type, a duplicate id, a missing required field (`input.label`, `markdown.md`, `code.lang`/`code.code`, `image.src`/`image.alt`, `progress.label`), a `progress` with `max <= 0` or `value` outside `[0, max]`, a fact without a `value`, a newline in a fact or a `pros`/`cons` entry, a `diagram` whose `kind` isn't `mermaid` or whose `source` is empty or past 8 KiB, a `chart` with a ragged or non-finite series, a `term` output past 32 KiB, a `filetree` path that is absolute, dotted, or duplicated, a `record` fact without a label or a link that isn't https, an empty chip label, an option `visual` outside the `code`/`diagram`/`image`/`diff`/`chart`/`term`/`filetree`/`record` set, a single-select choice with more than one `recommended` option, an empty `detail`, an over-cap image, or a document past 1 MiB. Compose the whole document, validate once, fix everything in a single pass. A file that isn't valid JSON fails earlier, with the line and column of the offending byte.

## Worked document: an opener approval board

A condensed version of `examples/opener-board.json` — sections as tiers, one card per repo, the before struck out, alternates as a choice, an approval per card:

```json
{
  "version": 1,
  "title": "Repo openers, redrafted",
  "intro": "Each card shows my pick, the alternates, and the old line struck out. Approve, or pick an alternate.",
  "stats": [
    { "label": "repos drafted", "value": "2" },
    { "label": "flagged", "value": "1" }
  ],
  "submit": {
    "label": "Approve openers",
    "note": "Approving green-lights the README rewrites, pushed direct to main."
  },
  "blocks": [
    {
      "id": "sec-flagged",
      "type": "section",
      "title": "Flagged — corrected before approval",
      "md": "Pre-release repo; the drafted install command does not work today."
    },
    {
      "id": "card-cc-factory",
      "type": "card",
      "title": "cc-factory",
      "summary": "Opener redrafted; install command corrected to the released path.",
      "flagged": true,
      "status": "redrafted",
      "chips": [ { "label": "plugin" }, { "label": "corrected", "tone": "flag" } ],
      "children": [
        {
          "id": "cc-factory-was",
          "type": "markdown",
          "struck": true,
          "md": "A software factory for Claude Code."
        },
        {
          "id": "cc-factory-opener",
          "type": "markdown",
          "md": "**Your spec clocks in. A reviewed diff clocks out.**"
        },
        {
          "id": "cc-factory-alts",
          "type": "choice",
          "prompt": "My pick is selected. Prefer an alternate?",
          "options": [
            {
              "id": "pick",
              "label": "Your spec clocks in. A reviewed diff clocks out.",
              "hint": "my pick — names both ends of the loop",
              "facts": [
                { "label": "frame", "value": "command" },
                { "label": "words", "value": "9" }
              ],
              "detail": {
                "pros": [ "Names the input and the output in one line", "Factory metaphor matches the repo name" ],
                "cons": [ "Two sentences where the alternate uses one" ],
                "md": "The clock-in/clock-out pair carries the whole pipeline without listing a stage."
              }
            },
            {
              "id": "alt-a",
              "label": "Stop reviewing your agent's first draft.",
              "hint": "punchier, but leads with a negative",
              "facts": [
                { "label": "frame", "value": "loss" },
                { "label": "words", "value": "6", "tone": "good" }
              ],
              "detail": {
                "pros": [ "Names the pain directly" ],
                "cons": [ "Says what it stops, not what it ships" ]
              }
            }
          ]
        },
        {
          "id": "cc-factory-approval",
          "type": "approval",
          "prompt": "Approve this opener and the deferred-install correction?"
        }
      ]
    },
    {
      "id": "sec-shipped",
      "type": "section",
      "title": "Tier A — shipped repos"
    },
    {
      "id": "card-slop-cop",
      "type": "card",
      "title": "slop-cop",
      "chips": [ { "label": "both" } ],
      "children": [
        {
          "id": "slop-cop-opener",
          "type": "markdown",
          "md": "**Never ship 'delve' again.**"
        },
        {
          "id": "slop-cop-get-started",
          "type": "code",
          "title": "Get started",
          "lang": "bash",
          "code": "brew install yasyf/tap/slop-cop\nslop-cop check draft.md --pretty"
        },
        {
          "id": "slop-cop-approval",
          "type": "approval",
          "prompt": "Approve this opener?"
        }
      ]
    },
    {
      "id": "board-notes",
      "type": "input",
      "label": "Anything the board missed?",
      "multiline": true
    }
  ]
}
```

The full 26-repo original lives at `examples/opener-board.json` in the repo; both validate with `push --dry-run`.

## Board templates

The dominant board shape: a section groups cards, and each card carries the before struck out, the after, alternates as a choice, an optional code sample, and the approval that decides it. Focus mode turns exactly this into one step per card. Start from one of these skeletons and swap in your content — each validates with `push --dry-run` as-is.

**Single-decision redraft** — the minimal shape, one card:

```json
{
  "version": 1,
  "title": "Opener redraft",
  "submit": { "label": "Send verdicts", "note": "Approved wording ships as-is." },
  "blocks": [
    { "id": "sec-redrafts", "type": "section", "title": "Redrafts" },
    {
      "id": "card-opener",
      "type": "card",
      "title": "README opener",
      "status": "redrafted",
      "children": [
        { "id": "opener-was", "type": "markdown", "struck": true, "md": "A tool for checking prose." },
        { "id": "opener-new", "type": "markdown", "md": "**Never ship 'delve' again.**" },
        { "id": "opener-alts", "type": "choice", "prompt": "My pick is selected. Prefer an alternate?", "options": [
          { "id": "pick", "label": "Never ship 'delve' again.", "hint": "my pick — shortest, boldest",
            "facts": [ { "label": "words", "value": "4", "tone": "good" } ],
            "detail": { "pros": [ "A concrete tell every reader recognizes" ], "cons": [ "Opaque without LLM context" ] } },
          { "id": "alt-a", "label": "Catch the AI accent before it lands.", "hint": "softer, names the mechanism",
            "facts": [ { "label": "words", "value": "7" } ],
            "detail": { "pros": [ "Legible with zero context" ], "cons": [ "Less memorable than the pick" ] } }
        ]},
        { "id": "opener-approval", "type": "approval", "prompt": "Approve this opener?" }
      ]
    }
  ]
}
```

**Change with a code sample** — the same card, the code block carrying the exact edit:

```json
{
  "version": 1,
  "title": "Config change",
  "submit": { "label": "Approve change", "note": "Approving lands the edit on main." },
  "blocks": [
    { "id": "sec-changes", "type": "section", "title": "Proposed changes" },
    {
      "id": "card-timeout",
      "type": "card",
      "title": "Raise the request timeout",
      "summary": "30s trips on cold starts; 60s clears every observed case.",
      "children": [
        { "id": "timeout-was", "type": "markdown", "struck": true, "md": "`timeout: 30s` — trips on cold starts." },
        { "id": "timeout-new", "type": "markdown", "md": "Raise to **60s**; the p99 cold start is 41s." },
        { "id": "timeout-code", "type": "code", "lang": "yaml", "title": "config/server.yaml", "code": "server:\n  timeout: 60s" },
        { "id": "timeout-approval", "type": "approval", "prompt": "Ship this change?" }
      ]
    }
  ]
}
```

**Tiered multi-card review** — the shape scaled up: the urgent tier first (deck order is doc order), a routine tier after, a trailing input as the catch-all:

```json
{
  "version": 1,
  "title": "Release review",
  "stats": [ { "label": "changes", "value": "2" }, { "label": "flagged", "value": "1" } ],
  "submit": { "label": "Approve release", "note": "Approving tags v2.1 and publishes the notes." },
  "blocks": [
    { "id": "sec-flagged", "type": "section", "title": "Flagged — decide first" },
    {
      "id": "card-breaking",
      "type": "card",
      "title": "Breaking: config key rename",
      "flagged": true,
      "chips": [ { "label": "breaking", "tone": "flag" } ],
      "children": [
        { "id": "breaking-was", "type": "markdown", "struck": true, "md": "`packDirs` (camelCase) in `config.json`." },
        { "id": "breaking-new", "type": "markdown", "md": "Rename to `pack_dirs`; the old key errors with a migration hint." },
        { "id": "breaking-approval", "type": "approval", "prompt": "Accept the breaking rename?" }
      ]
    },
    { "id": "sec-routine", "type": "section", "title": "Routine" },
    {
      "id": "card-notes",
      "type": "card",
      "title": "Release notes",
      "children": [
        { "id": "notes-new", "type": "markdown", "md": "**v2.1** — offline validation, `--no-doc` drains, line/col JSON errors." },
        { "id": "notes-approval", "type": "approval", "prompt": "Ship these notes?" }
      ]
    },
    { "id": "board-notes", "type": "input", "label": "Anything the release missed?", "multiline": true }
  ]
}
```

## Pack blocks

Installed block packs extend the block set beyond the built-ins above. A pack block's `type` is dotted — `<pack>.<block>`, both segments lowercase kebab-case — and the dot is the namespace boundary: built-in types never contain one. Discover what is installed:

```bash
cc-present pack list
```

Per pack it prints the name and version, the pack directory, the absolute path of the pack's reference fragment, and each block's dotted type with an `(interactive)` marker; `dropped:` lists every skipped candidate with its reason. Read the reference fragment before first use — it documents the pack's fields the way this page documents the built-ins.

A pack block composes like any other leaf, at the top level or inside a card:

```json
{ "id": "ex-rating", "type": "example.rating", "label": "How useful is this reference pack?", "scale": 5 }
```

`push --dry-run` validates each pack block against the pack's declared JSON Schema, offline like the rest of the check. An uninstalled dotted type fails with `pack block type "example.rating" is not installed`; a schema violation names the type and the failing property. Every cap and rule above applies to pack blocks unchanged. Authoring a new pack is the `cc-present:author-pack` skill's job.
