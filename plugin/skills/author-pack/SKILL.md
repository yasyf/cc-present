---
name: author-pack
description: Author a block pack — custom block types for cc-present boards, shipped as React components declared by JSON Schemas and one prebuilt bundle. Use when the user wants a new block type for cc-present, a custom component or custom block on a board, asks to author, build, or write a block pack, or wants to extend cc-present's blocks beyond the built-ins.
---

# /author-pack

You are authoring a block pack: custom block types the cc-present SPA loads at runtime. A pack is one directory — a TOML manifest (`cc-present.toml`), a JSON Schema per block, and one prebuilt ES-module bundle of React components. Each block gets a dotted wire type, `<pack>.<name>`, that composes on a board like any built-in. Author a pack when the built-in blocks can't render what a board needs. *Using* installed packs on a board is the `present` skill's job (its `reference/blocks.md` § Pack blocks) — don't re-teach composition here.

Invoke it as bare `cc-present` — Claude Code (≥ 2.1.91) puts the plugin's `bin/` on the Bash tool PATH. If the command isn't found or resolves to a stale version, use the absolute path `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present"` (see the present skill's `reference/troubleshooting.md`), which resolves and caches the version-exact binary via binrun on its first call. Building the bundle also needs `bun` on PATH.

## 1. Scaffold

```bash
cc-present pack init --name <pack> <dir>
```

Offline, no daemon. `--name` defaults to the target directory's basename, and the command refuses a non-empty directory. It writes 23 files: a working pack with one content block (`<pack>.callout`) and two interactive blocks (`<pack>.rating`, plus the two-step `<pack>.survey` wizard that exercises the hostApi 1 helpers), renamed to your pack throughout, plus a `.gitignore` that ignores only `node_modules/` — never `dist/`, which a shipped pack commits.

The name must match `^[a-z][a-z0-9-]*$` and run at most 32 characters. It becomes the `<pack>.` half of every block type; built-in types never contain a dot, so the dotted namespace belongs to packs permanently.

```
my-pack/
├── cc-present.toml      # the manifest
├── schema/              # one JSON Schema per block, plus interaction schemas
├── examples/            # one example block object per block
├── src/
│   ├── pack.tsx         # bundle entry: default export { hostApi: 1, blocks }
│   ├── host/            # react shims + window.CcPresent typings — keep verbatim
│   └── *.tsx            # your components
├── reference/blocks.md  # what a consuming agent reads to compose your blocks
├── scripts/smoke.ts     # bundle self-test
├── vite.config.ts       # lib-mode build with the react aliases
├── package.json / tsconfig.json / .gitignore
```

## 2. Manifest and schemas

In `cc-present.toml`: `name`, `version`, `entry` (the bundle path, under `dist/`), and one `[blocks.<name>]` table per block, each pointing at its schema, an optional `interaction` schema, and at least one `examples` entry. The `interaction` schema's presence is what marks a block interactive. Field-by-field rules, every exact validation error, and the discovery and conflict rules: `reference/manifest.md`.

Each block schema validates the whole block object — `id` and `type` included — as Draft 2020-12, with every external `$ref` rejected. Pin `type` to a `const` and set `additionalProperties: false` so `pack lint` catches a typoed field instead of passing it through:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["id", "type", "label"],
  "additionalProperties": false,
  "properties": {
    "id": { "type": "string", "minLength": 1 },
    "type": { "const": "my-pack.rating" },
    "label": { "type": "string", "minLength": 1 }
  }
}
```

An interactive block's `interaction` schema describes the payload a human submits. It is enforced at the REST edge on every click, so keep it just as strict.

## 3. Components

Components import `react` normally; the vite config aliases `react` and `react/jsx-runtime` to the `src/host/` shims, which re-export the host page's single React instance from `window.CcPresent`. **Never bundle React.** The aliases are the mechanism, and `external`/`output.globals` don't work for browser ESM. The entry's default export registers the blocks:

```tsx
export default {
  hostApi: 1,
  blocks: { callout: Callout, rating: Rating, survey: Survey },
};
```

The host qualifies those bare names with your manifest's pack name and calls each component with `{block, value, submit, disabled, context}`. The full host surface, the prop semantics, and the interaction wire: `reference/host-api.md`. The rules that matter most:

- Render read-only when `disabled` — the artifact is closed or the block's round is over; `context` (`{closed, roundOver, round}`) says which.
- Theme with `ui.tokens`, the host's frozen map of theme-variable references, via the scaffold's `src/host/present.ts` wrappers (`tokens().text`, `tokens().surface`, `tokens().accent`, …), so the block matches light and dark boards. The token names are the contract; never hand-write a raw palette variable.
- Never put an `asset:` URI in a pack field: the garbage collector's reference walk can't see pack-defined fields, so the bytes get deleted. Use `/packs/<pack>/dist/…`, `https:`, or `data:` URLs.
- Blocks also render full-bleed in single-block mode (the iOS webview) — don't assume board chrome around you.
- In focus mode an interactive block is its own step, your component the card's body — the host reserves no gestures over it: `reference/host-api.md` § Focus mode.

Host API 1 includes three interactivity helpers, all wrapped by `src/host/present.ts`: `toast({kind, text})` raises a shell toast at commit moments; `usePackState(key, initial)` holds per-tab draft state that survives focus-deck navigation and agent re-upserts (it dies on reload and never enters the event log); and the `context` prop above decomposes `disabled`. A multi-control block declares one object interaction schema and each control submits the merged payload — `submit({...(value ?? {}), field})`. The scaffolded survey block exercises all of it. Both the manifest's `host_api` and the bundle export's `hostApi` must be exactly `1`: `reference/host-api.md` § Versioning.

## 4. Build and check

```bash
bun install
bun run typecheck
bun run build     # emits dist/pack.js — the manifest's entry
bun run smoke     # imports the built bundle under a stubbed host
cc-present pack lint .
```

`pack lint` runs discovery's own fail-loud checks (strict manifest, `host_api`, declared files present, schemas compiling) plus one discovery skips: every declared example must validate against its block schema. A clean lint prints exactly one line, `ok: <pack> <version> (N blocks)`. A fresh scaffold fails lint until you build (`entry "dist/pack.js" not found`), and `smoke` asserts the scaffolded block names — update `scripts/smoke.ts` when you rename or add blocks. Anything else red: `reference/troubleshooting.md`.

## 5. Register and verify

**Dev loop.** Add the pack root's absolute path to `packDirs` in `~/.cc-present/config.json` (`pack init` prints the exact path to add). The daemon re-scans within 2 seconds — no restart — and a dev pack shadows an installed plugin pack of the same name, so you can iterate on a pack you've already shipped. Two rebuild caveats: the SPA imports a bundle once per page, so reload the tab; and bundle URLs are cached immutably keyed on the manifest `version`, so bump `version` (or hard-reload) to see a rebuild.

**Ship.** Put the pack — `dist/` built and committed — at `.claude/components/` in your Claude plugin. That directory is the pack root (`.claude/components/cc-present.toml`), and a plugin ships exactly one pack. The daemon never builds a pack; it serves the committed `dist/` unchanged, so a plugin missing the built bundle is dropped at discovery with a visible reason.

**Verify** either way:

```bash
cc-present pack list
```

Per pack it prints the name and version, the directory, the reference fragment's absolute path, and each block's dotted type with an `(interactive)` marker; `dropped:` lists every skipped candidate with its reason. Then `push --dry-run` a document that uses the dotted type — an uninstalled type fails with `pack block type "<pack>.<name>" is not installed`; a validating one prints `ok`.

## 6. Document for agents

Fill in the pack's own `reference/blocks.md` — the file the manifest's `reference` field names. `pack list` prints its absolute path next to your pack, and it is what a consuming agent reads before composing your blocks. Keep the scaffold's format: one section per block with a field table (name, type, required, notes) and one JSON example, plus a line saying what the interaction payload means.

## Worked example, end to end

The user asks: "add a severity picker for our triage boards."

```bash
cc-present pack init --name triage triage
```

```
scaffolded pack "triage" into triage (23 files)

next steps:
  cd triage
  bun install
  bun run build          # builds dist/pack.js, which pack lint needs
  bun run smoke
  cc-present pack lint .

register it for local dev by adding this absolute path to packDirs in ~/.cc-present/config.json:
  /work/triage
```

Rework the scaffolded `rating` block into `severity`:

- `mv schema/rating.json schema/severity.json`, same for `schema/rating.interaction.json` and `examples/rating.json`.
- `cc-present.toml` — retitle `[blocks.rating]` to `[blocks.severity]`, point `schema`, `interaction`, and `examples` at the moved files, reword `description`.
- `schema/severity.json` — `"const": "triage.severity"` (and the `title`); the same `type` in `examples/severity.json`.
- `src/pack.tsx` — `blocks: { callout: Callout, severity: Rating }`; the bare key is what the host qualifies to `triage.severity`.
- `scripts/smoke.ts` — the asserted block names become `['callout', 'severity', 'survey']`.
- `reference/blocks.md` — retitle the `triage.rating` section to `triage.severity` and describe the payload.

Build, check, register:

```bash
cd triage && bun install && bun run typecheck && bun run build && bun run smoke
cc-present pack lint .        # → ok: triage 0.2.0 (3 blocks)
```

Add `"/work/triage"` to `packDirs` in `~/.cc-present/config.json`; within 2 seconds:

```bash
cc-present pack list
```

```
triage 0.2.0
  dir: /work/triage
  reference: /work/triage/reference/blocks.md
  blocks:
    triage.callout
    triage.severity (interactive)
    triage.survey (interactive)
```

Prove it composes — write a document using the new type and dry-run it:

```json
{
  "version": 1,
  "title": "Pack check",
  "blocks": [
    { "id": "sev", "type": "triage.severity", "label": "How bad is the outage?", "scale": 4 }
  ]
}
```

```bash
cc-present push --dry-run "$DOC"   # → ok
```

When the human later clicks a point, the agent receives `{"blockId":"sev","payload":{"value":3},"type":"pack.interaction"}`. To ship, commit the pack — `dist/` included — at the plugin's `.claude/components/`.

## Reference

- `reference/manifest.md` — every manifest field, exact validation errors and drop reasons, discovery tiers, conflict rules, size caps, path containment.
- `reference/host-api.md` — the `window.CcPresent` surface, the react shims, component props, the `pack.interaction` wire, serving, single-block mode.
- `reference/troubleshooting.md` — symptom-first fixes: lint failures, dropped packs, stale bundles, rejected interactions.
- https://github.com/yasyf/cc-present — the full wire contract and the reference pack's source.
