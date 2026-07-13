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

The UI clamps long prose behind "Show more" — markdown blocks at ~10 lines, approval replies at 4, option bodies at 3. Write for the fold. Labels carry the pick, `hint` the one-line why, `md` only detail the human must read.

```json
{ "id": "sqlite", "label": "Use SQLite because it gives us a single-file deployment with zero operational overhead, WAL supports our single-writer daemon, and migrating to Postgres later stays easy since all SQL lives in the store package" }
```

```json
{ "id": "sqlite", "label": "SQLite, single writer", "hint": "zero ops; rules out horizontal scaling", "md": "WAL fits the per-repo daemon; all SQL stays in `store`, so a later Postgres move is contained." }
```

The first buries the tradeoff in a label nobody scans; the second reads at a glance and keeps the detail one clamp away.

## Composing for focus mode

A board with any decision unit opens in focus mode by default: one step at a time, in document order. The same document serves both views — these rules make it read well as a deck.

- **Each card is one focus step.** "Cards are decision units" is literal: the card renders as the step body and its controls decide it.
- **Doc order is deck order.** Order cards by decision priority — the human meets them one at a time.
- **A card fits one screen.** The content-density clamps above are the budget; an overlong card splits into multiple cards, same as on the board.
- **Context attaches forward.** A top-level run of content blocks (markdown, code, diff, image, table, progress) becomes the lead-in of the next card or decidable; a trailing run is its own read-only step. Put context immediately before the decision it informs — a run cut off by a section header turns into a standalone read-only step instead of attaching.
- **Sections are never steps.** A section surfaces as the tier label in the deck's progress header.
- **A lone approval swipes.** A step whose only decidable is an approval takes swipe-to-decide — right approves, left rejects. A card with several decidables decides by its controls only.
- **Content-only boards stay boards.** A push with no decision unit opens as the classic board; set `presentation` to override either default.

## Size caps

| Cap | Limit |
|---|---|
| Serialized document | 1 MiB |
| `data:` URI in `image.src` | 32 KiB |
| Local image file (inlined to an `asset:` ref) | 5 MiB |

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
{ "id": "opener-approval", "type": "approval", "prompt": "Approve this opener?", "allowFeedback": true }
```

`allowFeedback` defaults to true; set `false` to forbid free-text feedback and verdict notes on this block.

### `choice` — pick from options

```json
{
  "id": "opener-alts",
  "type": "choice",
  "prompt": "My pick is selected. Prefer an alternate?",
  "multi": false,
  "options": [
    { "id": "pick", "label": "Never ship 'delve' again.", "hint": "my pick — shortest, boldest" },
    { "id": "alt-a", "label": "AI-written, without the AI accent.", "hint": "safer, less memorable" }
  ]
}
```

An option has three tiers: `label` (the pick itself, ~6 words), optional `hint` (a one-line inline-markdown qualifier rendered dim beside the label — the why or the cost), and optional `md` (a detail body the UI clamps at ~3 lines). A newline in `hint` fails validation, naming the block id.

### `input` — free text

```json
{ "id": "extra-notes", "type": "input", "label": "Anything I missed?", "placeholder": "Optional", "multiline": true }
```

An input carried into a new round renders empty with a dim "last round: …" hint showing the previous entry; the old text stays read-only inside the collapsed round. Fields are fresh each round automatically — never ask the human to clear one.

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

`push --dry-run FILE` runs the full check offline and prints the first violation, naming the offending block id — an unknown type, a duplicate id, a missing required field (`input.label`, `markdown.md`, `code.lang`/`code.code`, `image.src`/`image.alt`, `progress.label`), a `progress` with `max <= 0` or `value` outside `[0, max]`, an over-cap image, or a document past 1 MiB.

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
            { "id": "pick", "label": "Your spec clocks in. A reviewed diff clocks out.", "hint": "my pick — names both ends of the loop" },
            { "id": "alt-a", "label": "Stop reviewing your agent's first draft.", "hint": "punchier, but leads with a negative" }
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

## Pack blocks

Installed block packs extend the block set beyond the built-ins above. A pack block's `type` is dotted — `<pack>.<block>`, both segments lowercase kebab-case — and the dot is the namespace boundary: built-in types never contain one. Discover what is installed:

```bash
"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" pack list
```

Per pack it prints the name and version, the pack directory, the absolute path of the pack's reference fragment, and each block's dotted type with an `(interactive)` marker; `dropped:` lists every skipped candidate with its reason. Read the reference fragment before first use — it documents the pack's fields the way this page documents the built-ins.

A pack block composes like any other leaf, at the top level or inside a card:

```json
{ "id": "ex-rating", "type": "example.rating", "label": "How useful is this reference pack?", "scale": 5 }
```

`push --dry-run` validates each pack block against the pack's declared JSON Schema, offline like the rest of the check. An uninstalled dotted type fails with `pack block type "example.rating" is not installed`; a schema violation names the type and the failing property. Every cap and rule above applies to pack blocks unchanged. Authoring a new pack is the `cc-present:author-pack` skill's job.
