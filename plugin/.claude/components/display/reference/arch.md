# Architecture docs (arch/v1)

An architecture doc describes a system at every zoom level as one JSON file.
The renderer, `src/arch-map.js`, draws it as a map a reader can zoom into: the
top level shows the main systems, and each box opens into the components it is
made of, as deep as the doc goes. Lines carry labels, sticky notes carry the key
numbers, and the main path is numbered at every level it crosses.

The file holds no positions. You write what exists and what connects to what,
and the renderer lays out boxes, routes lines and places labels. The same file
reads as data: stable ids, nesting for detail, and `toMarkdown` prints it
as an outline. The schema is `src/arch-map.schema.json`.

## Write one

1. `name`, the system's short name, and `title`, the one claim the top level
   makes.
2. `nodes`, the top level, four to eight boxes in the order a request meets them.
   Give each one `nodes` for what it expands into. Four to eight per level
   read best; nest the rest.
3. `edges` between the deepest nodes you know. An edge between two components
   inside different systems shows as a line between those systems at the top
   level, and between the components once a reader zooms in.
4. `path`, the one journey the doc explains, three to eight steps.
5. `stories`, other journeys worth walking: a retry, a crash, an option under
   review.
6. `notes`, each one fact or number, on a node or on an edge.
7. `groups` for the boundaries boxes sit in: an account, a machine, a process.

## How zoom works

A reader always sees one level: the children of the node in focus, drawn as
boxes, with their own children drawn as parts inside each box. Selecting a
part that has children offers "Zoom into" it, and the trail at the top zooms
back out.

Everything that names a node deeper than the level shows at the nearest
visible ancestor. A step inside one visible box becomes a step on that box,
and consecutive ones merge. A node outside the focus that an edge or step
reaches is drawn as an outside box, so a reader zoomed into one system still
sees what calls it. Notes show only where their node or edge is drawn.

## Fields

### Doc

| Field | Type | Required | Notes |
|---|---|---|---|
| `arch` | `1` | yes | The format version. |
| `name` | string, 40 | yes | First crumb in the zoom trail. |
| `title` | string, 100 | yes | The top level's one claim, drawn across the top. |
| `summary` | string, 1200 | no | A paragraph for whoever reads only this. |
| `nodes` | Node[] | yes | The top level. |
| `groups` | Group[] | no | Dashed boundaries around boxes. |
| `edges` | Edge[] | no | Lines between nodes at any depth. |
| `path` | `{title, steps}` | no | The main journey, numbered in blue. |
| `stories` | Story[] | no | Chips that walk other journeys. |
| `notes` | Note[] | no | Sticky notes. |
| `lanes` | `{main, background, control}` | no | Row labels when boxes set `lane`. |
| `start` | `{node}` or `{story}` | no | Open zoomed into a node, or on a story. |

### Node

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | id | yes | A lowercase letter, then letters, digits and dashes, 40 at most. Unique across the doc. |
| `kind` | `actor` \| `app` \| `service` \| `worker` \| `store` \| `log` \| `external` \| `artifact` \| `doc` | yes | Picks the box's outline. |
| `name` | string, 28 | yes | About three words. |
| `role` | string, 48 | yes | What it does, a short verb phrase. |
| `about` | string, 280 | no | Shown when a reader selects it. |
| `claim` | string, 100 | no | The claim its zoomed-in view makes. Defaults to name and role. |
| `nodes` | Node[] | no | What it expands into. |
| `shape` | part shape | no | The glyph when it shows as a part: `api`, `relay`, `gate`, `router`, `guard`, `queue`, `pool`, `log`, `recovery`, `handoff`, `watcher`, `keys`, `budget`, `timer`, `uploader`, `codec`, `compactor`, `sweeper`, `cache`, `ui`, `tool`, `docs`, `sdk`, `module`, `store`. Defaults from `kind`. |
| `medium` | `memory` \| `disk` \| `bucket` \| `keys` \| `pages` | store and log only, required there | Picks the store's silhouette. |
| `tag`, `holds` | string 8, rows | store and log only | A short label such as `S3`; rows of what it keeps. |
| `status` | `live` \| `planned` \| `retired` | no | `planned` marks what is not built yet, `retired` what is going away. |
| `group` | id | no | The group drawn around it when it shows as a box. |
| `lane`, `critical`, `badges` | | no | As in `display.system`. |

### Group, edge, step and note

**Group**: `id`, `label` (40), `kind` (`zone`, `machine` or `process`; a
process names its machine in `in`).

**Edge**: `from`, `to`, `label` (40, about five words), and `kind`: `sync`
calls and waits (the default), `async` hands off without waiting, `copy`
copies data, `control` checks or signals.

**Step**: `from`, `to`, `say` (40). On the path, `waitsFor` names the earlier
step a reply waits for. On a story, `mark` is `risk` or `stop`.

**Note**: `text` (56), and either `on`, a node id, or `edge`, the `{from, to}`
of an edge the doc draws.

## Rendering

```js
import { mount, toMarkdown, validate } from "./arch-map.js";

const handle = mount(host, doc, { onState: (view) => save(view) });
handle.show({ focus: "api", block: "orders" });
```

`mount` draws the clean skin of the system map: crisp lines, sans type and a
key under the map. The map's full screen button opens the same doc in a
dialog that keeps its zoom level. `validate(doc)` returns the problems as
sentences, and `mount` shows them in place of the map.

`scripts/fixtures/arch-shop.json` is a complete example.
