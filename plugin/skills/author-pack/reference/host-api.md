# Host API

The runtime contract between a pack bundle and the cc-present page: what `window.CcPresent` provides, how the shims deliver it, what your components are called with, and how a submitted payload travels back to the agent.

## The bundle contract

The entry's default export is the pack module:

```tsx
export default {
  hostApi: 1,
  blocks: { callout: Callout, rating: Rating },
};
```

`hostApi` is declared twice — `host_api` in the manifest (checked at discovery) and here in the export (asserted by `bun run smoke`). The block keys are the bare halves of your dotted types; the host qualifies them with the manifest's pack name, so the `rating` key backs `<pack>.rating`.

## `window.CcPresent`

The host page installs this object before any pack bundle loads. The scaffold's `src/host/present.ts` carries the exact typings — keep that file verbatim.

| Field | What it is |
|---|---|
| `hostApi` | `1` — the compat gate. |
| `React` | The page's single React instance (`typeof import('react')`). |
| `jsxRuntime` | The automatic-runtime factories: `jsx`, `jsxs`, `Fragment`. |
| `reactDom.createPortal` | Portal into the host document (tooltips, overlays). |
| `ui.renderMarkdown(md)` | Markdown to the host's sanitized HTML string. |
| `ui.renderInlineMarkdown(md)` | The inline variant — no block elements. |
| `ui.Clamped` | The host's show-more clamp: a component taking `{ lines?, children }`. |

## The shims — why you never bundle React

`src/host/react.ts` and `src/host/jsx-runtime.ts` re-export the host's React from `window.CcPresent`, and the vite config aliases `react` and `react/jsx-runtime` to them (the specific specifier must precede `react`, which also matches `react/jsx-runtime` as a prefix). Components and `tsc` see ordinary React imports; the built bundle contains none.

A pack that bundles its own React runs two Reacts in one page, and every hook throws "Invalid hook call". The aliases are the mechanism — don't switch to `external` or `output.globals`, which don't work for browser ESM: there is no bundler at runtime to resolve the bare specifier. Everything other than React inlines, so `dist/pack.js` is self-contained apart from `window.CcPresent`.

## Component props

The host renders each pack block by calling your component with `{block, value, submit, disabled}`:

- **`block`** — the block object from the document. `id` and the dotted `type` are guaranteed; every other field is pack-defined, with your block schema as the contract.
- **`value`** — the human's last-committed interaction payload for this block, or `undefined` before the first interaction. Interactions are last-write-wins per block and reduce from the event log, so `value` survives reloads.
- **`submit(payload)`** — posts a new interaction payload. It must satisfy your `interaction` schema; the wire below rejects anything else.
- **`disabled`** — render read-only. True when the artifact is closed, when the block's round is over, or when the block declares no interaction schema.

A crashing component is contained: an error boundary swaps in a labeled placeholder (`crashed while rendering`) and retries when the agent redrafts the block. The other placeholder labels — `loading pack…`, `unknown pack`, `pack failed to load`, `component not exported` — mark the not-yet-renderable states.

## The interaction wire

`submit(payload)` posts `{type: "pack.interaction", blockId, payload}` to the REST edge, which checks, in order:

| Check | Rejection |
|---|---|
| Request body over **256 KiB** | HTTP 413, `interaction exceeds 262144 bytes` |
| Artifact closed | HTTP 409, `presentation is closed` |
| Block id unknown | `unknown block "sev"` |
| Block is not a pack block | `block "sev" is a markdown, not a pack block` |
| Block's round already closed | `block "sev" belongs to closed round 1` |
| Type not installed / not interactive | `pack block type "triage.severity" is not installed` / `… is not interactive` |
| Payload fails the interaction schema | `pack interaction "triage.severity": <schema violation>` |

An accepted payload appends to the event log and reaches the agent as a `<channel source="cc-present">` tag (or a `watch` line):

```json
{"blockId":"sev","payload":{"value":3},"type":"pack.interaction"}
```

The payload travels verbatim — the reducer stores it under `interactions.packs[blockId].payload` without inspecting its shape, and `outcomes` prints it there. The consuming agent learns what it means from your pack's `reference/blocks.md`, so document it.

## Serving

`GET /packs/<pack>/<file>?v=<version>` serves only files under the pack's `dist/` subtree; an unknown pack, a path outside `dist/`, or a missing file is a 404. Responses are `nosniff` and cached immutably, keyed on the manifest `version` — a rebuild is invisible until the version bumps. `.js`/`.mjs`, `.css`, and `.json`/`.map` get pinned Content-Types so a bundle always loads as an ES module. A declared `styles` file injects once per page.

## Single-block mode

`/p/<ref>?block=<id>` renders one block full-bleed: the same event stream and interaction REST as the board, with no board chrome. It is what the iOS client loads in a webview per pack block. A block whose round has closed renders read-only through the same `disabled` flag. When a `ccPresentHeight` WebKit message handler is present, the page posts `{type: "height", px}` on every content resize so the native host can size the webview.

Design for it: the component must lay out sanely at full width with nothing around it, and content should size itself — a fixed height fights the resize reporting.

## Focus mode

A board with decisions opens as a focus deck — one step at a time — and an interactive pack block is one step, your component rendering as the body of a focus card. The single-block rule above already covers the layout: content sizes itself, and nothing about board chrome is assumed. `disabled` semantics are unchanged.

Step participation is automatic: a manifest-declared `interaction` schema makes the block its own step and counts it toward the submit tally, on web and iOS alike; a block without one is context. The component owns all pointer interaction inside its card: a pack step is never swipe-to-decide, and the host reserves no gestures over it. No new host surface: `hostApi` stays 1.
