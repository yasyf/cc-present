# display pack blocks

The `display` pack holds read-only blocks for explanations: a Markdown page
with Mermaid diagrams, an animated sequence of messages, an animated timeline,
a comparison grid, and a sandboxed HTML or SVG artifact. None is interactive,
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

## display.timeline

Dated events on a vertical rail. With `autoplay`, events appear one at a time
when the block scrolls into view; otherwise, all show and the play button
replays them.

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

Options as columns, criteria as rows. A toned cell leads with a mark: `✓` for
`good`, `✗` for `bad`, `!` for `warn`, and `–` for `neutral`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `columns` | array | yes | 1 to 8 `{label, sub?, highlight?}`. `highlight` tints the column. |
| `rows` | array | yes | 1 to 40 `{label, hint?, cells}`. |
| `rows[].cells` | array | yes | One cell per column, in column order: an inline Markdown string, or `{md, tone?}`. |
| `caption` | string | no | Markdown above the grid. |
| `title` | string | no | One line. |

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
