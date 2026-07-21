# Troubleshooting

**`pack init` refused to run.** `destination "my-pack" is not empty` — init scaffolds only into an empty or nonexistent directory; pick a fresh one. `derived name "My Pack" is invalid (…); pass --name` — the target directory's basename fails the name rules (`^[a-z][a-z0-9-]*$`, at most 32 characters); pass a valid `--name` explicitly.

**A fresh scaffold fails `pack lint`.** `entry "dist/pack.js" not found` is expected before the first build — lint checks the file the manifest's `entry` names, and the scaffold ships source, not a bundle. Run `bun install && bun run build` first, then lint.

**`pack list` shows the pack under `dropped:`.** The reason is the fix. A manifest or schema string (see `manifest.md` for every exact string): correct the named field or file and rerun `pack lint`. `host_api 2, want 1`: set both the manifest's `host_api` and the bundle export's `hostApi` to `1`, then rebuild. `entry "dist/pack.js" not found` on an installed plugin pack: the plugin shipped without the built bundle — build and commit `dist/`, the daemon never builds for you. `pack "my-pack" disabled by config`: remove the name from `disabledPacks` in `~/.cc-present/config.json`.

**Two same-named packs both vanished.** `duplicate pack name "my-pack" in same tier` is a deliberate mutual drop — neither silently wins. Rename one, or remove one of the two roots. Across tiers there is no drop pair: the dev copy wins and the plugin copy shows `pack "my-pack" shadowed by dev dir`, which is the intended dev loop. Remove the dir from `packDirs` when you want the installed copy back.

**Every hook throws "Invalid hook call".** The bundle carries its own React, so two React instances share one page. Keep the scaffold's vite aliases pointing `react` and `react/jsx-runtime` at the `src/host/` shims — and don't replace them with `external` or `output.globals`, which don't work for browser ESM. Check `dist/pack.js`: a correct bundle contains no React internals.

**A rebuilt bundle doesn't show in the browser.** Two caches stack: the SPA imports a bundle once per page (reload the tab), and bundle URLs are cached immutably keyed on the manifest `version` (bump `version`, or hard-reload). Bumping the version is the reliable path — it changes the URL.

**`pack block type "triage.severity" is not installed` at push.** No installed pack supplies that type. Run `pack list`: pack absent entirely — discovery never saw it (add the root to `packDirs`, or install the plugin that ships it); pack under `dropped:` — fix what the reason names; pack listed but the type isn't — the block key in the manifest or `pack.tsx` doesn't match the type your document uses.

**The block renders as a labeled placeholder.** The label names the state. `loading pack…` resolves itself. `unknown pack` — no installed pack matches the type's `<pack>` half. `pack failed to load` — the bundle import threw; check the browser console. `component not exported` — the pack loaded but its default export's `blocks` map has no entry for this block's bare name; align `pack.tsx` with the manifest's `[blocks.<name>]`. `crashed while rendering` — your component threw; it retries automatically when the agent redrafts the block.

**A click submits nothing, or the interaction is rejected.** A block with no `interaction` schema in its manifest table renders with `disabled` true — declare one to make the block interactive. A rejected submit names its check: `pack interaction "triage.severity": <violation>` (the payload fails your interaction schema), `interaction exceeds 262144 bytes` (HTTP 413 — the payload is over the 256 KiB cap), `block "sev" belongs to closed round 1` (the round is over; only the current round accepts interactions), or HTTP 409 `presentation is closed` (the artifact is closed for good).

**`bun run smoke` fails after renaming or adding blocks.** The scaffolded `scripts/smoke.ts` asserts the original block names against the built bundle's default export. Update its asserted name list to match your `pack.tsx`, rebuild, and rerun.

**Lint passes but an example seems wrong.** `pack lint` validates every declared example against its block schema — so if lint is green and the example still misleads, the schema is too loose. Tighten it: pin `type` to a `const`, set `additionalProperties: false`, and mark required fields, then lint again.
