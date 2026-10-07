# plan pack blocks

The `plan` pack holds exhibits for a plan: each one proves a single claim on a
card. `plan.calls` shows which calls a change adds, removes, or changes.
`plan.machine` shows a lifecycle and the screen for each state. `plan.mock`
draws UI that does not exist yet. Put one exhibit in a card, next to the
`choice` that changes it.

`plan.calls` and `plan.mock` are interactive but optional: the reader can
strike rows or comment on pins, and the board still reads "All answered"
without them. `plan.machine` is read-only.

## plan.calls

Call trees, one per entrypoint. Each row is one call. The header counts the
marked rows: `+` added, `−` removed, `~` changed, and `?` proposed.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `calls` | array | yes | 1 to 40 root rows, one per entrypoint. Each `calls` array holds at most 40 rows. |
| `calls[].call` | string | yes | One line: the call, component, or route as written in code. |
| `calls[].mark` | `+` \| `-` \| `~` \| `?` \| `" "` | yes | `+` new call, `-` removed, `~` changed, `?` proposed (drawn dashed), `" "` context. |
| `calls[].new` | boolean | no | Bold: the call is to a new symbol. A `+` row without it is a new call to an existing symbol. |
| `calls[].at` | string | no | `path:line`, shown at the end of the row. For a new function in an existing file, cite the line it goes after. |
| `calls[].note` | string | no | Inline Markdown shown when the reader taps the row. |
| `calls[].id` | string | no | A stable key for the struck list. Without one, a row's key is its index path, such as `0.1.2`. |
| `calls[].calls` | array | no | Child rows, same shape. |
| `title` | string | no | One line beside the `CALLS` tag. |
| `caption` | string | no | Inline Markdown under the tree. |

Keep a tree under about 15 rows. Two rows with the same key render an error
in place of the block.

```json
{"id": "sched-calls", "type": "plan.calls", "title": "Scheduling", "calls": [
  {"call": "<Composer/>", "mark": "~", "at": "web/src/composer/Composer.tsx:41", "calls": [
    {"id": "menu", "call": "<SendLaterMenu/>", "mark": "+", "new": true, "at": "web/src/composer/SendLaterMenu.tsx:12"},
    {"id": "metrics", "call": "trackScheduled()", "mark": "?", "at": "web/src/metrics.ts:14", "note": "Strike it if the event is not wanted."}
  ]}
]}
```

**Interaction:** `{"struck": ["metrics"]}` lists the keys of the rows the
reader struck. A struck row takes its whole subtree with it, so the subtree's
keys are not listed. Only marked rows can be struck.

## plan.machine

A state machine drawn as a diagram. The reader taps a state to read its note
and see its screen, and the transitions out of it light up. The block starts
on `initial`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `states` | array | yes | 1 to 12 states. Keep it to 8. |
| `states[].id` | string | yes | Letters, digits, `_`, `.`, `-`. |
| `states[].label` | string | no | Shown in the node; defaults to `id`. |
| `states[].note` | string | no | Inline Markdown: one short sentence about the state. |
| `states[].final` | boolean | no | A final state may have no way out; it draws with a heavy border. |
| `states[].mark` | `+` \| `-` \| `~` | no | New, removed, or changed state. |
| `states[].screen` | object | no | `{"md": "…"}` for Markdown, or `{"mock": {…}}` for a mock with the `plan.mock` fields `html`, `css`, `w`, `h`, `frame`, `url`, and `title`, without pins. |
| `transitions` | array | yes | Up to 40 `{from, event, to, label?, mark?}`. The arrow shows `label`, or `event` without one, so keep it to a key, a code, or two words. |
| `transitions[].mark` | `+` \| `-` \| `~` \| `?` | no | `+` and `?` draw dashed green, `-` faded red, `~` amber. Mark an arrow that hangs on a decision. If the whole machine is new, mark nothing. |
| `grid` | array | no | Up to 6 rows of up to 6 state ids, `null` for an empty cell. Every state needs exactly one cell. Put the main path on the top row and the ways out below it. |
| `initial` | string | no | Defaults to the first state. |
| `title` | string | no | One line. |
| `caption` | string | no | Inline Markdown under the block. |

Without a `grid`, states lay out left to right by distance from `initial`,
and labels can overlap once arrows cross.

The block checks the graph and renders an error banner in place of the
diagram. It names each state that `initial` cannot reach, each dead end that
lacks `final`, and each id that is unknown or misplaced on the grid. A `-`
transition does not count toward either check, and a `-` state is
exempt from both. `push --dry-run` validates only the schema, so open the
board to see these errors.

```json
{"id": "msg", "type": "plan.machine", "initial": "scheduled",
 "states": [{"id": "scheduled", "note": "Waiting for its time."}, {"id": "sending"}, {"id": "sent", "final": true}, {"id": "failed"}],
 "grid": [["scheduled", "sending", "sent"], [null, "failed", null]],
 "transitions": [
   {"from": "scheduled", "event": "due", "to": "sending"},
   {"from": "sending", "event": "ok", "to": "sent"},
   {"from": "sending", "event": "fail", "to": "failed"},
   {"from": "failed", "event": "retry", "to": "sending", "label": "retry ×3", "mark": "+"}
 ]}
```

## plan.mock

Real HTML drawn at the width you design for, scaled down to fit the column.
Draw the smallest region that makes the point: one card, one menu, or one
row, at a `w` of 480 or less with `frame` `none`, so it stays readable on a
phone.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `html` | string | yes | Body markup, at most 256 KiB. Use inline styles or `css`. |
| `css` | string | no | A stylesheet for the mock. |
| `w` | integer | no | The design width in pixels, 200 to 1600, default 480. |
| `h` | integer | no | Clips the height, 60 to 4000 pixels. |
| `frame` | `none` \| `browser` \| `phone` \| `desktop` \| `terminal` | no | Default `none`. |
| `url` | string | no | Shown in the `browser` frame's address bar. |
| `title` | string | no | A heading above the mock; the `desktop` and `terminal` frames show it in their title bar instead. |
| `pins` | array | no | Up to 9 `{ref, title, body?}`. |
| `pins[].ref` | string | yes | Matches a `data-ref="…"` attribute in `html`. |
| `pins[].title` | string | yes | The point, in a few words. |
| `pins[].body` | string | no | Inline Markdown: one sentence. |
| `caption` | string | no | Inline Markdown under the block. |

Each pin draws a numbered dot on the top-right corner of its `data-ref`
element, and a numbered list under the mock repeats every pin. A pin whose
element is missing stays in the list without a dot.

The `terminal` frame is a dark monospace surface that keeps whitespace. Its
helpers are `<b>` and the classes `dim`, `g`, `r`, `y`, `b`, `m`, `o`, `inv`,
and `box`. Keep it to a `w` of 480 or less, about 55 columns.

The mock runs in a sandboxed frame under a content security policy. Your
scripts, inline event handlers, and network requests do not run; images
must be `data:` URLs. Links do not navigate: the frame drops `href`,
`target`, and form `action` attributes, and removes `meta`, `base`, `link`,
and nested frames.

```json
{"id": "footer", "type": "plan.mock", "w": 440,
 "html": "<div class=\"ft\"><span class=\"btn pri\">Send</span><span class=\"btn\" data-ref=\"later\">Send later ▾</span></div>",
 "css": ".ft{display:flex;gap:8px;padding:12px}.btn{border:1px solid #ccc;border-radius:7px;padding:6px 12px}.pri{background:#111;color:#fff}",
 "pins": [{"ref": "later", "title": "New button", "body": "Send is unchanged."}]}
```

**Interaction:** `{"comments": {"later": "Make it a split button."}}` maps a
pin's `ref` to the reader's comment. Saving a blank comment removes it.
