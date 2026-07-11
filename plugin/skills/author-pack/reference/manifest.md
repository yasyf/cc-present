# Manifest, validation, and discovery

The manifest is `cc-present.toml` at the pack root, decoded strictly — an unknown key fails the parse with a `decode manifest:` error. Every path field is manifest-relative and must resolve inside the pack root.

## Fields

| Field | Required | Constraint |
|---|---|---|
| `host_api` | yes | Must equal the daemon's host API version, currently **1**. |
| `name` | yes | Matches `^[a-z][a-z0-9-]*$`, at most 32 characters; the `<pack>` half of every block type. |
| `version` | yes | Non-empty; cache-busts the bundle and styles URLs. |
| `description` | no | Prose shown in `/api/packs` and `pack list`. |
| `entry` | yes | The ES-module bundle; must live under `dist/`. |
| `styles` | no | A stylesheet the SPA injects once per page; must live under `dist/`. |
| `reference` | no | A Markdown fragment describing the blocks for a consuming agent; `pack list` prints its absolute path. |
| `blocks.<name>` | one or more | One table per block type; `<name>` matches the same pattern as `name`. |
| `blocks.<name>.description` | yes | Non-empty prose. |
| `blocks.<name>.schema` | yes | JSON Schema (Draft 2020-12) for the whole block object. |
| `blocks.<name>.interaction` | no | JSON Schema for the human interaction payload; its presence marks the block interactive. |
| `blocks.<name>.examples` | one or more | Example block objects; `pack lint` validates each against the schema. |

Schemas compile with a loader that rejects every external reference (`external schema reference not allowed: <url>`), so a schema can reach neither the network nor the filesystem.

## Validation errors, exact strings

`pack lint` and discovery run the same checks, so these strings appear both as lint failures and as `dropped:` reasons. Manifest-level, in check order:

| Violation | Exact string |
|---|---|
| Unknown key in the TOML | `decode manifest: <parser detail>` |
| Empty `name` | `name must not be empty` |
| Malformed `name` | `name "My_Pack" must match ^[a-z][a-z0-9-]*$` |
| `name` over 32 chars | `name "…" exceeds 32 characters` |
| Empty `version` | `version must not be empty` |
| `entry` not under `dist/` | `entry "pack.js" must be under dist/` |
| `styles` not under `dist/` | `styles "pack.css" must be under dist/` |
| A path escaping the root | `entry path "../pack.js" must resolve inside the pack root` (each path field names itself) |
| No `[blocks.*]` tables | `manifest declares no blocks` |
| Malformed block name | `block name "My.Block" must match ^[a-z][a-z0-9-]*$` |
| Empty block description | `block "rating": description must not be empty` |
| No examples for a block | `block "rating": must declare at least one example` |
| Manifest over the cap | `manifest exceeds 524288 bytes` |

Then the file-level checks:

| Violation | Exact string |
|---|---|
| Wrong `host_api` | `host_api 2, want 1` |
| Bundle not built | `entry "dist/pack.js" not found` |
| Declared styles missing | `styles "dist/pack.css" not found` |
| Declared reference missing | `reference "reference/blocks.md" not found` |
| Declared example missing | `block "rating" example "examples/rating.json" not found` |
| Schema over the cap | `block "rating" schema: read "schema/rating.json": exceeds 524288 bytes` |
| Schema fails to compile | `block "rating" schema: compile "schema/rating.json": <compiler detail>` (interaction schemas prefix `block "rating" interaction schema:`) |
| Example fails its schema (lint only) | `block "rating" example "examples/rating.json": <schema violation>` |

Discovery is fail-soft: a violation drops that pack — the directory and reason land in the `dropped` list shown by `pack list` and `/api/packs` — while every other pack still loads. `pack lint` is fail-loud: the first violation is the (non-zero) exit. The HTTP surface reports only a dropped directory's base name; the absolute path appears in `pack list`.

## Discovery tiers

The daemon scans two tiers of pack roots and re-scans on access after a 2-second TTL, so installing a pack needs no restart:

- **Dev** — each directory in `packDirs` (`~/.cc-present/config.json`), in order.
- **Plugin** — each installed Claude plugin (read from `$CLAUDE_CONFIG_DIR/plugins/installed_plugins.json`, default `~/.claude`) whose `.claude/components/` directory holds a `cc-present.toml`. The components directory is the pack root, so a plugin ships exactly one pack.

Same-name conflicts resolve by tier, and a config switch beats them all:

| Situation | Outcome | Drop reason |
|---|---|---|
| Dev and plugin packs share a name | The dev pack wins | `pack "my-pack" shadowed by dev dir` on the plugin copy |
| Two same-tier packs share a name | Both drop — a deliberate mutual drop, so neither silently wins | `duplicate pack name "my-pack" in same tier` on each |
| Name listed in the config's `disabledPacks` | Dropped unconditionally | `pack "my-pack" disabled by config` |

## Size caps and path containment

Every file discovery reads — the manifest, each schema, each example — caps at **512 KiB** (524,288 bytes). Every path field must be local to the pack root; files open through the root itself, so a symlink or `..` component that escapes reads as "not found", and the link is never followed. `entry` and `styles` must additionally live under `dist/`, because `GET /packs/<pack>/<file>?v=<version>` serves only the `dist/` subtree — anything else is a 404.
