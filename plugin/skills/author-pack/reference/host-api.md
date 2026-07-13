# Host API

The runtime contract between a pack bundle and the cc-present page: what `window.CcPresent` provides, how the shims deliver it, what your components are called with, and how a submitted payload travels back to the agent.

## The bundle contract

The entry's default export is the pack module:

```tsx
export default {
  hostApi: 2,
  blocks: { callout: Callout, rating: Rating, survey: Survey },
};
```

`hostApi` is declared twice — `host_api` in the manifest (checked at discovery) and here in the export, asserted by `bun run smoke` and range-checked again when the SPA imports the bundle. The block keys are the bare halves of your dotted types; the host qualifies them with the manifest's pack name, so the `rating` key backs `<pack>.rating`.

## Versioning — the floor rule

`host_api` declares the minimum host API the pack requires, and the daemon loads any pack whose floor it meets: `1 <= host_api <= 2`, the daemon's own version. A pack declaring `1` loads unchanged on today's version-2 host; the v1 surface is a strict subset of v2, so there is nothing to migrate. Declare `2` only when you use `ui.tokens`, `ui.toast`, `ui.usePackState`, or the `context` prop, since a floor above the daemon's version drops the pack at discovery with `host_api <n>, want 1..2`. There is no capability detection. The floor is the whole negotiation.

## `window.CcPresent`

The host page installs this object before any pack bundle loads. The scaffold's `src/host/present.ts` carries the exact typings — keep that file verbatim.

| Field | What it is |
|---|---|
| `hostApi` | `2` — the version this surface implements; every manifest's `host_api` floor gates against it. |
| `React` | The page's single React instance (`typeof import('react')`). |
| `jsxRuntime` | The automatic-runtime factories: `jsx`, `jsxs`, `Fragment`. |
| `reactDom.createPortal` | Portal into the host document (tooltips, overlays). |
| `ui.renderMarkdown(md)` | Markdown to the host's sanitized HTML string. |
| `ui.renderInlineMarkdown(md)` | The inline variant — no block elements. |
| `ui.Clamped` | The host's show-more clamp: a component taking `{ lines?, children }`. |
| `ui.tokens` | `ThemeTokens` — a frozen object of CSS-variable reference strings for theming (below). |
| `ui.toast` | `(toast: PackToast) => void` — raise a toast in the host's stack. |
| `ui.usePackState` | `<T>(key: string, initial: T) => [T, (next: T) => void]` — per-tab draft state (below). |

### `ui.tokens` — theming

A frozen object whose 20 values are `var()` reference strings (`'var(--text)'`), never resolved colors. Put them straight into inline styles and the block re-inks under light/dark and theme flips with no listener. The alias names behind them are the host's frozen public contract; the raw palette they point at churns freely between releases. Never hand-write a raw palette variable.

```ts
export interface ThemeTokens {
  readonly bg: string;
  readonly bgSoft: string;
  readonly surface: string;
  readonly surfaceRaised: string;
  readonly text: string;
  readonly dim: string;
  readonly border: string;
  readonly borderStrong: string;
  readonly accent: string;
  readonly accentFg: string;
  readonly ok: string;
  readonly warn: string;
  readonly danger: string;
  readonly focusRing: string;
  readonly radiusSm: string;
  readonly radiusMd: string;
  readonly radiusLg: string;
  readonly fontProse: string;
  readonly fontMono: string;
  readonly trackCaps: string;
}
```

### `ui.toast` — commit moments

```ts
export interface PackToast {
  kind: 'info' | 'error';
  text: string;
}
```

The toast renders in the shell's stack like the host's own notices. Use it for commit moments — a submit landed, an action failed — not for chatter. In single-block mode (the iOS webview) the stack flows in-tree inside the block instead of overlaying, and the height report grows to include it. Raising a toast before the shell has mounted throws.

### `ui.usePackState` — per-tab drafts

`usePackState<T>(key: string, initial: T): [T, (next: T) => void]` is a React hook (call it unconditionally, at the top level of the component) holding ephemeral draft state — a wizard's step index, an uncommitted text field. It is scoped to the enclosing block's id, so two blocks of the same type never share a key, and it throws outside a pack block.

Its lifetime is the tab: state survives board↔focus toggles, focus-deck navigation unmounting your component, and agent re-upserts of the block; it dies on reload and never touches the event log. Anything the agent must see goes through `submit` instead.

## The shims — why you never bundle React

`src/host/react.ts` and `src/host/jsx-runtime.ts` re-export the host's React from `window.CcPresent`, and the vite config aliases `react` and `react/jsx-runtime` to them (the specific specifier must precede `react`, which also matches `react/jsx-runtime` as a prefix). Components and `tsc` see ordinary React imports; the built bundle contains none.

A pack that bundles its own React runs two Reacts in one page, and every hook throws "Invalid hook call". The aliases are the mechanism — don't switch to `external` or `output.globals`, which don't work for browser ESM: there is no bundler at runtime to resolve the bare specifier. Everything other than React inlines, so `dist/pack.js` is self-contained apart from `window.CcPresent`.

## Component props

The host renders each pack block by calling your component with `{block, value, submit, disabled, context}`:

- **`block`** — the block object from the document. `id` and the dotted `type` are guaranteed; every other field is pack-defined, with your block schema as the contract.
- **`value`** — the human's last-committed interaction payload for this block, or `undefined` before the first interaction. Interactions are last-write-wins per block and reduce from the event log, so `value` survives reloads.
- **`submit(payload)`** — posts a new interaction payload. It must satisfy your `interaction` schema; the wire below rejects anything else.
- **`disabled`** — render read-only. True when the artifact is closed, when the block's round is over, or when the block declares no interaction schema.
- **`context`** — `{closed: boolean, roundOver: boolean, round: number}`. It decomposes the *why* behind `disabled`, the artifact terminally closed versus the block's round over, and carries the current 1-based round number, so a component can say "Round over" instead of greying out. It is deliberately block-local, with no sibling blocks and no card parent: a pack block is a leaf, and on iOS it renders alone in its own webview.

The exact prop types, from the scaffold's `src/host/present.ts`:

```ts
export interface PackBlockContext {
  closed: boolean;
  roundOver: boolean;
  round: number;
}

export interface PackComponentProps {
  block: PackBlock;
  value: unknown;
  submit: (payload: unknown) => void;
  disabled: boolean;
  context: PackBlockContext;
}
```

A crashing component is contained: an error boundary swaps in a labeled placeholder (`crashed while rendering`) and retries when the agent redrafts the block. The other placeholder labels — `loading pack…`, `unknown pack`, `pack failed to load`, `component not exported` — mark the not-yet-renderable states.

## Multi-control blocks — the merge idiom

A block has exactly one interaction slot: interactions are last-write-wins per block, and the newest payload replaces the old one whole. A block with several controls therefore declares **one object interaction schema** covering every field, and each control submits the merged object — spread the prior `value`, overwrite the fields you own:

```tsx
const prior = (value as Record<string, unknown> | null | undefined) ?? {};
submit({ ...prior, summary });
```

This is race-free by construction: posts serialize per artifact, and the optimistic reduction lands before the next render, so the next control's spread always sees the previous one's write. The reference pack's survey block is the worked example — two steps, one merged `{summary, detail}` payload, drafts held in `usePackState` until the final submit.

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

Step participation is automatic: a manifest-declared `interaction` schema makes the block its own step and counts it toward the submit tally, on web and iOS alike; a block without one is context. The component owns all pointer interaction inside its card: a pack step is never swipe-to-decide, and the host reserves no gestures over it. hostApi 2 changes none of this — step participation still comes from the manifest, never from a runtime declaration. What v2 does buy a focus step is `usePackState`: deck navigation unmounts non-current steps, and per-tab draft state is what keeps a half-filled wizard alive across that unmount.
