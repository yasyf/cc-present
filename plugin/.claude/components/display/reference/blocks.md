# display pack blocks

The `display` pack holds read-only blocks for explanations: a Markdown page
with Mermaid diagrams, an animated sequence of messages, an animated timeline,
a comparison grid, a system map, and a sandboxed HTML or SVG artifact. None is interactive,
so none adds work to a round. Put them in any `Doc.blocks` array or a card's
`children`, next to built-in blocks such as `table` and `diagram`.

To show one file without writing a document, run `cc-present show`:

```bash
cc-present show explainer.html             # display.artifact, kind html
cc-present show flow.svg --height 480      # display.artifact, kind svg
cc-present show notes.md --title "Why"     # display.page
```

`show` resumes this window's artifact and replaces its document, like
`cc-present start --doc`. Pass `--new` to open a fresh artifact instead.

## display.page

Long-form Markdown. Each ```` ```mermaid ```` fence renders as a themed
diagram in place; every other fence stays a code block.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `md` | string | yes | Markdown, at most 256 KiB. |
| `title` | string | no | One line. |

## display.sequence

One to three panels of actors exchanging messages, drawn as lifelines with
arrows. All panels step in lockstep, so two panels side by side compare an old
flow with a new one step for step. The block plays when it scrolls into view;
play, pause, previous, next, and restart buttons sit under the panels. The
current step is drawn solid and its `note`, or its `label`, shows as a caption
under each panel.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `panels` | array | yes | 1 to 3 panels. |
| `panels[].title` | string | no | One line above the panel. |
| `panels[].actors` | array | yes | 1 to 8 `{id, label}`, drawn left to right. |
| `panels[].steps` | array | yes | 1 to 60 steps, in order. |
| `steps[].from` | string | yes | An actor id. |
| `steps[].to` | string | no | An actor id. Omit it, or repeat `from`, for a note on the `from` lifeline. |
| `steps[].label` | string | yes | One line on the arrow or note. |
| `steps[].note` | string | no | Inline Markdown caption shown while the step is current. |
| `steps[].tone` | `default` \| `ok` \| `warn` \| `danger` | no | Arrow and note color. |
| `steps[].dashed` | boolean | no | A dashed arrow, for replies and async messages. |
| `title` | string | no | One line. |
| `autoplay` | boolean | no | Default `true`. `false` shows every step at once. |
| `intervalMs` | integer | no | 400 to 10000, default 1600. |

A step naming an actor its panel does not declare renders an error in place
of the block, listing each bad step.

Panels lay out at the width they render, with 12 px labels. A label that
does not fit wraps onto up to three lines, hyphenating a long word, and past
that ends in an ellipsis whose tooltip shows the full text.

## display.timeline

Dated events on a vertical rail. When every `when` reads as a date or a clock
time (`Sep 12`, `2026-09-12`, `9/12`, `14:03`), the timeline gets player
controls: with `autoplay`, events appear one at a time when the block scrolls
into view; otherwise, all show and the play button replays them. A timeline
whose `when` values are labels, such as `Release 6`, renders as a plain ordered
list with no player and ignores `autoplay`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `events` | array | yes | 1 to 100 events, in display order. |
| `events[].when` | string | yes | One line, shown as written: `Sep 12`, `2026-09-12 14:03 PT`. |
| `events[].title` | string | yes | One line. |
| `events[].md` | string | no | Markdown detail. |
| `events[].tag` | string | no | One-line pill, such as a PR number or component. |
| `events[].tone` | `default` \| `ok` \| `warn` \| `danger` | no | Dot color. |
| `events[].url` | string | no | `https://` link on the title. |
| `title` | string | no | One line. |
| `autoplay` | boolean | no | Default `false`. |
| `intervalMs` | integer | no | 400 to 10000, default 1200. |

## display.compare

Options as columns, criteria as rows. Below 600 px wide the grid stacks:
each row becomes a card, with the column label above each cell. A toned cell leads with a mark: `✓` for
`good`, `✗` for `bad`, `!` for `warn`, and `–` for `neutral`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `columns` | array | yes | 1 to 8 `{label, sub?, highlight?}`. `highlight` tints the column. |
| `rows` | array | yes | 1 to 40 `{label, hint?, cells}`. |
| `rows[].cells` | array | yes | One cell per column, in column order: an inline Markdown string, or `{md, tone?}`. |
| `caption` | string | no | Markdown above the grid. |
| `title` | string | no | One line. |

## display.system

A hand-drawn whiteboard map of a running system. Blocks are drawn by what
they do: a cylinder for a disk, a chip for memory, a bucket for object
storage. The parts inside a block appear as small glyphs. The main path is
drawn as blue numbered steps on the arrows. Stories are chips that walk other
journeys, and sticky notes carry the key numbers.

A reader clicks or zooms into a block to see its parts and how they connect.
You give no positions. List blocks in the order a request meets them, and the
map lays itself out, routes the arrows, and places every caption clear of
every box.

Write it in this order:

1. `title`: the one claim.
2. `blocks`: 4 to 8 is the sweet spot, 10 at most.
3. `parts` for the blocks a reader opens.
4. `links`: part to part where you know it. Links between parts of different
   blocks become the block arrows.
5. `path`: the one journey the map explains, 3 to 5 steps.
6. `stories`: 2 to 4 other journeys worth walking (a retry, a crash, a
   recovery).
7. `notes`: at most 3, each one number.

Mark the store the path must reach before replying with `critical`, and put
`waitsFor` on the reply step.

When the map is too dense to read at the block's width, it shows a glimpse of
names only with an "Open the map" button, which opens the full map in a
dialog. The reader's place (story, open block, part) survives an update of
the block.

### Fields

The limits below are JSON Schema `maxLength` or `maxItems` values. Word counts in parentheses are guidance and are not enforced.

#### Map (top level)

| Field | Type | Required | Notes |
|---|---|---|---|
| `title` | string, 100 | yes | The one claim the map makes (about 18 words). Drawn bold across the top and used as the SVG's accessible name. |
| `blocks` | Block[], 2 to 10 | yes | One per running thing or store. Listed in the order a request meets them. |
| `groups` | Group[], at most 6 | no | Dashed outlines around blocks. |
| `links` | Link[], at most 40 | no | Arrows. Part-level links roll up to block arrows. |
| `path` | Path | no | The one journey the map exists to explain, drawn as blue numbered steps. |
| `stories` | Story[], at most 7 | no | Other journeys. Each is a chip that overlays its steps and frames the view. |
| `notes` | Note[], at most 3 | no | Yellow sticky notes, each carrying one key number. |
| `lanes` | `{main, background, control}`, strings of 24 | no | Row labels. Defaults: "Main path", "Background", "Control". |
| `start` | `{story}` or `{block}` | no | What the map opens on. The default is the whole map, fitted. |

#### Group

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | id | yes | |
| `label` | string, 40 | yes | (5 words) |
| `kind` | `zone` \| `machine` \| `process` | no, default `zone` | A zone is an organisational or network boundary, a machine is a host or VM, and a process is an OS process. Each draws with its own dash. |
| `in` | id | only for `process` | The machine group this process runs on. A machine is drawn around its own blocks and its processes. |

#### Block

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | id | yes | |
| `kind` | block kind, see Vocabulary | yes | Picks the silhouette. |
| `name` | string, 28 | yes | Bold (3 words). Never broken mid-word. |
| `role` | string, 48 | yes | One line under the name (6 words). |
| `group` | id | no | |
| `medium` | medium, see Vocabulary | `store` and `log` only | Picks the store shape. |
| `tag` | string, 8 | no, `store` and `log` only | Chip text, such as `EBS`, `S3`, `TMPFS` or `Postgres`. |
| `parts` | Part[], at most 8 | no | What is inside. A block with parts, or a store with `contents`, can be opened. |
| `contents` | `{text: 40, note?: 60, sealed?: bool}`[], at most 6 | no, `store` and `log` only | Rows on the store's inside view. `sealed` draws a closed green or open red padlock. |
| `about` | string, 280 | no | Shown in the readout when the block is selected. |
| `critical` | bool | no | A thick yellow marker ring. Use it for the store the main path must reach before it replies. |
| `status` | `live` \| `planned` \| `retired` | no, default `live` | Anything other than `live` draws a dashed outline with a pill. |
| `badges` | Badge[], at most 2 | no | Pills at the top right. |
| `lane` | `main` \| `background` \| `control` | no | Overrides the row rule under Layout. |

#### Part

**Part**: `id`, `shape` (part shape, see Vocabulary, required), `name` (28, required), `role` (48, required), `medium` (required when `shape` is `store`, forbidden otherwise), `tag` (8), `about` (280), `status`, `badges` (at most 2).

#### Link

| Field | Type | Required | Notes |
|---|---|---|---|
| `from`, `to` | id of a block or part | yes | |
| `label` | string, 28 | no | Caption (4 words). |
| `style` | `sync` \| `async` \| `copy` \| `control` | no, default `sync` | |
| `hidden` | bool | no | Drawn only while a story walks it, for example startup recovery. |

#### Path, story, step, note, badge and id

**Path**: `title` (60, required), `steps` (Step[], 2 to 6, required).

**Story**: `id`, `label` (18, the chip text, 2 words), `title` (60), `tone` (`ok` \| `fail` \| `pending`, the chip dot), `steps` (Step[], 2 to 6).

**Step**: `from`, `to` (a block or part; the same id twice is a step inside one block), `say` (40, 6 words), `waitsFor` (main path only: the number of an earlier step; draws a red barrier on this step's arrow captioned "waits for step N"), `mark` (stories only: `risk` draws it red and dashed, `stop` draws a stop sign where the design refuses on purpose).

**Note**: `on` (a block or part id), `text` (56, 8 words). It sits beside its block on the overview and beside its part in the inside view.

**Badge**: `tone` (`ok` green check, `fail` red cross, `warn` amber, `info` gray, `ask` speech bubble), `text` (8).

**id**: `^[a-z][a-z0-9-]{0,39}$`.

### Vocabulary

Block kinds:

| `kind` | Use for | Drawn as |
|---|---|---|
| `actor` | a person or outside caller | box with a stick figure |
| `app` | a UI someone runs | window with a title bar |
| `service` | a long-running server | box with a gear and a left bar |
| `worker` | a background loop | box with a cycle arrow |
| `store` | data at rest | by `medium` |
| `log` | append-only data | by `medium`, with segment marks |
| `external` | a third-party API | rounded box with a plug |
| `artifact` | a package or build output | box with a lid |
| `doc` | a document or spec | folded page |

Media (`store` and `log` blocks, and store parts):

| `medium` | Use for | Drawn as |
|---|---|---|
| `memory` | RAM, tmpfs | chip with a ticked base. A memory part of a service, worker or app is drawn nested inside that block. |
| `disk` | local disk, EBS, a database volume | cylinder |
| `bucket` | object storage | bucket with a rim |
| `keys` | KMS, a vault | rounded box with a key |
| `pages` | anything else | stacked pages |

Part shapes, one glyph each:

| Shape | Meaning | Shape | Meaning |
|---|---|---|---|
| `api` | an endpoint | `relay` | passes requests through |
| `gate` | lets a request pass only when a condition holds | `router` | sends each request one of several ways |
| `guard` | rejects bad input, checks auth | `queue` | holds work in order |
| `pool` | a set of workers or connections | `log` | appends records |
| `recovery` | rebuilds state after a crash | `handoff` | passes ownership to another part |
| `watcher` | notices a change and acts | `keys` | holds or unwraps keys |
| `budget` | caps a resource, evicts | `timer` | fires on a schedule |
| `uploader` | sends data off the box | `codec` | encodes or decodes |
| `compactor` | merges or shrinks data | `sweeper` | deletes old data |
| `cache` | fast copy of slower data | `ui` | a screen |
| `tool` | a CLI or script | `docs` | a document |
| `sdk` | a client library | `module` | any other code |
| `store` | data (set `medium`) | | |

Link styles: `sync` means the caller waits (solid line, filled head). `async` is picked up later (dashed line, open head). `copy` is bulk data copied elsewhere (long dash, double head). `control` covers keys, config and health checks (dotted gray line).

### Layout

- **Columns** run left to right in the order blocks first appear in `blocks`, and a top-level group's blocks stay together. Inside a machine group, its process groups come first, ordered by where the main path enters them, and the machine's own blocks follow.
- **Rows:** a block on the main path, or sync-linked to one, goes on top. A block with only control links, or with the `keys` medium, goes at the bottom. Everything else goes in the middle. `lane` overrides this. Lane labels show only when more than one row is used.
- **Overview arrows:** links roll up to one arrow per block pair and style, captioned with the label of the first such link. A link between two parts of one block appears only in that block's inside view.
- **Main path and story steps** ride the arrow between their blocks. A step against an arrow's direction rides a parallel rail beside it. A step with no arrow either way draws its own.
- **Inside view:** parts are cards shaped by `shape`, links between them are edges numbered by the main path or the active story, and neighbouring blocks are ghost cards at the sides.
- **Narrow canvas:** below 900 px of canvas width (not viewport width), the grid turns 90 degrees, rows become columns, and the main path reads top to bottom.

### Checks the map runs

1. Ids are unique across groups, blocks, parts and stories.
2. A `process` group has `in` naming a `machine` group, and no other group has `in`.
3. Every `group`, link end, step end, note `on`, and `start` names a known id. `start.block` must have parts or `contents`.
4. `medium` appears exactly on `store` and `log` blocks and on `store` parts. `tag` and `contents` appear only where `medium` does (`contents` on blocks only).
5. Every part appears in at least one link or step.
6. No link joins a block to itself or to one of its own parts.
7. At most 12 overview arrows remain after roll-up.
8. `waitsFor` names an earlier main-path step and sits on a step between two blocks, `mark` appears only in stories, and story ids and labels are unique.

Semantic mistakes, such as an unknown id, an unlinked part or a process
outside a machine, do not fail `push --dry-run`. They show as a red list in
place of the map. Fix them and update the block.

### Example

```json
{
  "id": "orders-map",
  "type": "display.system",
  "title": "An order is on disk before the shopper hears yes",
  "groups": [
    {
      "id": "phone",
      "kind": "zone",
      "label": "Shopper's phone"
    },
    {
      "id": "vm",
      "kind": "machine",
      "label": "Orders VM"
    },
    {
      "id": "api-proc",
      "kind": "process",
      "label": "orders-api process",
      "in": "vm"
    },
    {
      "id": "cloud",
      "kind": "zone",
      "label": "Our AWS account"
    }
  ],
  "blocks": [
    {
      "id": "app",
      "kind": "app",
      "name": "Shop app",
      "role": "Places and retries orders",
      "group": "phone"
    },
    {
      "id": "api",
      "kind": "service",
      "name": "Orders API",
      "role": "Checks, logs, then replies",
      "group": "api-proc",
      "badges": [
        {
          "tone": "ok",
          "text": "4/4"
        }
      ],
      "parts": [
        {
          "id": "http",
          "shape": "api",
          "name": "HTTP handler",
          "role": "Parses POST /orders"
        },
        {
          "id": "dedupe",
          "shape": "gate",
          "name": "Retry gate",
          "role": "Drops repeated order ids"
        },
        {
          "id": "seen",
          "shape": "store",
          "medium": "memory",
          "tag": "RAM",
          "name": "Seen ids",
          "role": "Last hour of order ids"
        }
      ]
    },
    {
      "id": "wal",
      "kind": "log",
      "medium": "disk",
      "tag": "EBS",
      "name": "Order log",
      "role": "Append, fsync per order",
      "group": "vm",
      "critical": true,
      "contents": [
        {
          "text": "Order rows",
          "note": "/var/lib/orders/wal",
          "sealed": true
        }
      ]
    },
    {
      "id": "shipper",
      "kind": "worker",
      "name": "Log shipper",
      "role": "Copies closed segments",
      "group": "vm"
    },
    {
      "id": "bucket",
      "kind": "store",
      "medium": "bucket",
      "tag": "S3",
      "name": "Backup bucket",
      "role": "Keeps every segment",
      "group": "cloud"
    },
    {
      "id": "key",
      "kind": "store",
      "medium": "keys",
      "tag": "KMS",
      "name": "Log key",
      "role": "Wraps the disk key",
      "group": "cloud"
    }
  ],
  "links": [
    {
      "from": "app",
      "to": "http",
      "label": "POST /orders"
    },
    {
      "from": "http",
      "to": "dedupe",
      "label": "order id"
    },
    {
      "from": "dedupe",
      "to": "seen",
      "label": "seen before?"
    },
    {
      "from": "dedupe",
      "to": "wal",
      "label": "append + fsync"
    },
    {
      "from": "wal",
      "to": "shipper",
      "style": "async",
      "label": "closed segment"
    },
    {
      "from": "shipper",
      "to": "bucket",
      "style": "copy",
      "label": "upload"
    },
    {
      "from": "api",
      "to": "key",
      "style": "control",
      "label": "unwrap at boot"
    }
  ],
  "path": {
    "title": "A placed order",
    "steps": [
      {
        "from": "app",
        "to": "api",
        "say": "send the order"
      },
      {
        "from": "api",
        "to": "wal",
        "say": "append and fsync"
      },
      {
        "from": "api",
        "to": "app",
        "say": "reply 201",
        "waitsFor": 2
      }
    ]
  },
  "stories": [
    {
      "id": "retry",
      "label": "A retry",
      "title": "The app retries after a timeout",
      "steps": [
        {
          "from": "app",
          "to": "api",
          "say": "same order id again"
        },
        {
          "from": "dedupe",
          "to": "seen",
          "say": "found in memory"
        },
        {
          "from": "api",
          "to": "app",
          "say": "reply 201, no second row"
        }
      ]
    },
    {
      "id": "restart",
      "label": "A restart",
      "title": "The API restarts and forgets recent ids",
      "steps": [
        {
          "from": "api",
          "to": "key",
          "say": "unwrap the disk key"
        },
        {
          "from": "dedupe",
          "to": "seen",
          "say": "memory starts empty",
          "mark": "risk"
        }
      ]
    }
  ],
  "notes": [
    {
      "on": "wal",
      "text": "fsync adds about 2 ms per order"
    }
  ]
}
```

## display.artifact

A caller-supplied HTML document or SVG in a sandboxed frame. Scripts run, but
the frame has an opaque origin: it cannot read the board, its cookies, or its
storage, and the daemon rejects its requests. Inline everything the artifact
needs, or load it from `https://` URLs; relative paths do not resolve. The
frame grows to fit its content unless `height` pins it.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `kind` | `html` \| `svg` | yes | `svg` centers the image and scales it to the frame width. |
| `source` | string | yes | The document, at most 512 KiB. |
| `title` | string | no | One line above the frame. |
| `height` | integer | no | Fixed height in pixels, 80 to 4000. |
