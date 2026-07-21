# Write a block pack

A block pack adds your own block types to cc-present boards: React components
the SPA loads at runtime, each declared by a JSON Schema and addressed as
`<pack>.<name>`. Write one when the built-in blocks can't render what your
Claude plugin wants to present. The wire and discovery facts — manifest fields,
namespacing, conflict rules, serving — live in
[the contract](contract.md#block-packs); the steps below are the authoring path.

## Scaffold from the reference pack

`pack init` copies the embedded reference pack — a working pack with one
content block and two interactive blocks — into a new directory and renames
it, so the scaffold below ships `my-pack.callout`, `my-pack.rating`, and
`my-pack.survey`:

```sh
cc-present pack init --name my-pack my-pack
cd my-pack && bun install
```

`--name` defaults to the target directory's basename, and the command refuses
a non-empty directory. Working inside the cc-present repo, `cp -r
examples/packs/example my-pack` yields the same source files, unrenamed —
along with any local `node_modules/`, `dist/`, and `bun.lock` the copy drags in,
which is why `pack init` is the better start even in-repo.

The layout — 23 files, including a generated `.gitignore` covering
`node_modules/`:

```
my-pack/
├── cc-present.toml      # the manifest
├── .gitignore           # node_modules/ only — dist/ ships committed
├── schema/              # one JSON Schema per block, plus interaction schemas
├── examples/            # one example block object per block
├── src/
│   ├── pack.tsx         # bundle entry: default export { hostApi: 1, blocks }
│   ├── host/            # react shims + window.CcPresent typings — copy verbatim
│   └── *.tsx            # your components
├── reference/blocks.md  # what an authoring agent reads to compose your blocks
├── scripts/smoke.ts     # bundle self-test
└── vite.config.ts       # lib-mode build with the react aliases
```

## Edit the manifest

In `cc-present.toml`, set `name` (the `<pack>` half of every block type),
`version`, and one `[blocks.<name>]` table per block, each pointing at its
schema, an optional interaction schema, and at least one example. The
field-by-field rules are in [the manifest table](contract.md#manifest).

`host_api` is an exact protocol identity, not a compatibility floor. Set it to
`1`. Every other value drops the pack at discovery with `host_api <n>, want 1`.

## Write the schemas and components

Each block schema validates the whole block object. Pin `type` to a const and
set `additionalProperties: false` so `pack lint` catches a typoed field instead
of passing it through:

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

An interactive block also declares an `interaction` schema — the shape of the
payload a human submits, validated at the REST edge on every click.

Components import `react` normally and register through the entry's default
export. This is `src/pack.tsx` from the reference pack:

```tsx
import { Callout } from './Callout';
import { Rating } from './Rating';
import { Survey } from './Survey';

export default {
  hostApi: 1,
  blocks: { callout: Callout, rating: Rating, survey: Survey },
};
```

The host qualifies those bare names with your manifest's pack name and calls
each component with `{block, value, submit, disabled, context}`: `block` is
the block object, `value` is the human's last-committed interaction payload
(or undefined), `submit` posts a new payload, `disabled` means render
read-only, and `context` decomposes why — `{closed, roundOver, round}`.

The host also hands packs three UI helpers on `window.CcPresent.ui`, wrapped
by the scaffold's `src/host/present.ts`: `tokens()` returns the frozen map of
theme-variable references (`t.text`, `t.surface`, `t.accent`, …) so inline
styles match light and dark boards; `toast({kind, text})` raises a shell toast
at commit moments; and `usePackState(key, initial)` holds per-tab draft state
scoped to the block — it survives focus-mode navigation and agent re-upserts,
dies on reload, and never touches the event log.

A block with several controls submits one merged payload against one object
interaction schema — each control spreads the prior value and overwrites the
fields it owns:

```tsx
submit({ ...((value as Record<string, unknown>) ?? {}), summary });
```

The scaffolded survey block is the worked reference for all of this: drafts in
`usePackState`, a toast on submit, a round footer from `context`, styling from
`tokens()`, and the merge idiom across its two steps.

> **Warning:** never bundle React. The vite config aliases `react` and
> `react/jsx-runtime` to the `src/host/` shims, which re-export the host's
> single React instance from `window.CcPresent`. A pack that bundles its own
> copy runs two Reacts in one page and every hook throws "Invalid hook call".
> Keep the aliases; don't switch to `external` or `output.globals`, which don't
> work for browser ESM.

## Build and check

```sh
bun run typecheck
bun run build
bun run smoke
cc-present pack lint .
```

`build` emits `dist/pack.js`, the file the manifest's `entry` names. `smoke`
imports the built bundle under a stubbed host and asserts the default export's
shape. `pack lint` runs the daemon's own discovery checks plus one it skips:
every declared example must validate against its block schema. A clean lint
prints one line:

```
ok: my-pack 0.2.0 (3 blocks)
```

## Iterate against a live daemon

Add the pack root to `packDirs` in `~/.cc-present/config.json`:

```json
{ "packDirs": ["/path/to/my-pack"] }
```

The daemon re-scans within a couple of seconds, so the next push sees your
pack; a dev pack also shadows an installed plugin pack of the same name, so you
can iterate on a pack you've already shipped. Two caveats when you rebuild:

- The SPA imports bundles once per page — reload the tab.
- Bundle URLs are cached immutably, keyed on the manifest `version`. A rebuild
  without a version bump can serve the browser's stale copy; bump `version` or
  hard-reload.

## Ship it in your plugin

Put the pack — with `dist/` built and committed — at `.claude/components/` in
your Claude plugin. The components directory is the pack root
(`.claude/components/cc-present.toml`), and a plugin ships exactly one pack.
The daemon never builds your pack; it serves `dist/` as-is, so a plugin without
the built bundle is dropped at discovery with a visible reason.

Fill in `reference/blocks.md` while you're at it: it's what an authoring agent
reads to learn your block types, and `cc-present pack list` prints its path
next to each installed pack.

## See also

- [The contract § Block packs](contract.md#block-packs) — manifest fields,
  discovery and conflict rules, the `/api/packs` shape, single-block mode
- `examples/packs/example/` — the reference pack `pack init` scaffolds from
- `cc-present pack list` — the installed packs, their block types, and the
  dropped candidates with reasons
- The cc-present plugin ships a `cc-present:author-pack` skill that walks an
  agent through this page's workflow
