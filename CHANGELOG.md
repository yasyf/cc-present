# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.1] - 2026-07-12

### Fixed
- Agent replies render under every block type in the web and iOS clients;
  previously only approval blocks showed their thread.
- The daemon rejects a reply naming a block id absent from the current
  document instead of silently accepting it.
- `cc-present reply` prints a `replied: <blockId>` confirmation on success.

## [0.6.0] - 2026-07-11

### Added
- Block packs: plugins teach the board new typed blocks. A pack is a
  directory with a `cc-present.toml` manifest, a JSON Schema per block (plus
  an interaction schema when the block is interactive), and a prebuilt JS
  bundle. The daemon discovers packs from configured dev dirs and installed
  Claude plugins — dev shadows installed, and a broken or duplicate candidate
  drops fail-soft with a reason instead of sinking the registry — validates
  every pack file against a 512 KiB cap inside an `os.Root`, and serves the
  registry at `GET /api/packs` and each pack's `dist/` files at
  `GET /packs/{pack}/{file...}`, contained to the pack root.
- `pack.interaction`, the human event for interactive pack blocks: the REST
  edge validates its payload against the block's interaction schema under the
  same 256 KiB body cap as every interaction, and the reducer folds it
  last-write-wins into `interactions.packs` without inspecting the payload's
  shape. Closed rounds snapshot pack values like decisions and inputs.
- `cc-present pack list`, printing the discovered packs, each block's dotted
  type, the reference fragment path, and the dropped candidates with reasons;
  and `cc-present pack lint <dir>`, fail-loud validation of a pack root
  (manifest, schemas, examples) that exits non-zero on the first violation.
- Single-block render mode: `/p/<subject>?block=<id>` renders one block bare
  and reports its height through the `ccPresentHeight` WebKit message
  handler — the embedding surface for native clients.
- iOS pack rendering: a pack block renders through `PackBlockWebView`, a
  content-sized WKWebView on the single-block route, so plugin blocks work on
  the phone with no Swift-side renderer.
- `cc-present pack init [--name <n>] <dir>`, scaffolding a new block pack
  from a go:embedded copy of the reference pack: 19 files renamed to the pack
  name, a generated `.gitignore`, a printed build-and-lint checklist, and a
  refusal to write into a non-empty directory.
- The `author-pack` plugin skill, walking an agent through the pack-authoring
  workflow end to end — scaffold, manifest and schemas, components against
  the host API, build and lint, register, and document — with reference pages
  for the manifest, the host API, and troubleshooting.

### Changed
- `Doc.Validate` (Go API) now takes a `doc.PackTypes`, so a document carrying
  dotted pack block types validates against the installed registry;
  `doc.NoPacks` preserves the packless behavior.

## [0.5.0] - 2026-07-10

### Fixed
- A finished round no longer leaves an empty "Round N" header and a "0/0
  decided" bar on the board. The view now derives a board phase — live,
  waiting, or closed — so between rounds a waiting panel names what comes
  next, and a closed presentation shows only its banner and read-only
  history. The close notice itself moved out of the notification strip
  (whose full-bleed layout stranded it at the window edge) and into the
  in-flow banner.

### Added
- Keyboard-first reviewing: `j`/`k` walk the decidable items, `n` jumps to
  the next undecided one, `a`/`r`/`c` decide the focused approval, `1`–`9`
  toggle choice options, `f` opens the feedback composer or focuses a
  field, `⌘/Ctrl+Enter` submits through the same confirm as the button,
  and `?` shows the shortcut reference. The submit bar gains per-item
  progress dots that jump on click.
- Lifecycle and failure states: a loading skeleton until the event replay
  catches up, error toasts when a post fails (the round stays live and
  retryable — submits are no longer applied optimistically), a pending
  "sending…" marker on feedback until its echo lands, and an in-bar armed
  confirm replacing the native dialog for undecided submits.
- The Ledger visual language: bond-paper and phosphor-ink palettes with a
  deep-teal accent, a monospace structural voice, flat rows with left
  rails that encode each block's state, a round-timeline spine, rotated
  APPROVED/REJECTED stamps, and a mono masthead. Both themes hold WCAG AA
  contrast.

### Changed
- `@cc-interact/react` 0.5.0: toasts render through the library's new
  floating `ToastStack`; the board gates its skeleton on the stream's
  `caughtUp`; failed posts surface via the mutation `onError` hook; posts
  serialize per subject via the mutation `scope` so append order matches
  action order.

## [0.4.0] - 2026-07-08

### Added
- Rounds partition the board over time. A submit on a board the agent has
  touched closes the current round: its blocks collapse into a read-only
  "Round N" group with an outcome summary (verdicts, picks, notes), and only
  blocks upserted afterward render live. A new `round` verb
  (`cc-present round [--title <t>]`) names the next round, or forces a
  boundary mid-round; after a submit it titles the round the submit already
  opened. Blocks carry forward by re-upsert — touching an old block pulls it
  into the current round while its frozen copy stays in the collapsed group.
- Inputs are fresh each round. A carried-forward field renders empty with a
  dim "last round: …" hint showing the previous entry, which also stays
  read-only inside the collapsed round. Values are stamped with the round of
  their enclosing block, not the board's current round, so an SSE echo
  replayed after an optimistic submit cannot restamp them into the new round.
- The REST plane rejects block-scoped interactions on a closed round's blocks
  (400, `block "id" belongs to closed round N`), so a stale tab cannot write
  into history. Submit stays exempt.

### Changed
- Reduced state gains `rounds` — `current`, `currentTitle`, per-block round
  stamps, and a `history` of frozen `RoundRecord`s (blocks plus that round's
  decisions, choices, inputs, and feedback; `submittedRevision` when a submit
  closed it) — and `outcomes` reports it. `InputValue` carries the round it
  was entered in. Old event logs replay unchanged: submits partition them.
- Closed rounds render through @cc-interact/react 0.4.0's `CollapsedGroup`,
  whose cooperative read-only context disables controls inside expanded
  history without `inert`, keeping "show more" clamps working. The submit bar
  tallies the current round only and shows a round chip once history exists.

## [0.3.0] - 2026-07-07

### Added
- Graphite default theme: cool neutral surfaces, a single violet-blue accent,
  and a sans display face, picked over two alternates on a live candidate
  board. The palette is one `light-dark()` token block instead of three
  duplicated copies.
- Dark-mode toggle in the doc header, cycling through system, light, and
  dark. The mode persists to localStorage and a pre-paint guard in
  `index.html` keeps a reload from flashing the wrong scheme.
- Show more clamping for long text: markdown blocks clamp at ~10 lines,
  approval thread replies at 4, and choice option bodies at 3, and only when
  the content actually overflows. Expanding an option never selects it, and a
  redrafted block returns to the collapsed state.
- Content tiers: optional single-line `card.summary`, rendered as a dim lede
  under the title, and choice `option.hint`, rendered beside the label.
  Validation rejects newlines in either, naming the block id. The present
  skill gained density rules so agents author boards that stay scannable.

### Changed
- The web UI restyled on the @cc-interact/react 0.3.0 type, spacing, radius,
  and elevation scales: elevated cards with sans semibold titles over dim
  summary ledes, selection indicators and focus rings on options, a glyphed
  segmented verdict control, in-card markdown headings demoted below card
  titles, hairline table rules, and a raised submit bar.
- Choice options are keyboard-first `role=radio`/`role=checkbox` elements with
  Enter/Space handling; a closed board drops its options from the tab order.

## [0.2.1] - 2026-07-05

### Fixed
- Diagnosable binary install: the SessionStart hook no longer discards the
  installer's output — it captures stdout and stderr to `install-binary.log`
  under the plugin root, so a failed remote install leaves a trace instead of
  nothing. The hook stays non-fatal.
- Bounded install download: `install-binary.sh`'s `curl` gained
  `--connect-timeout`/`--max-time`, so a stalled network fails fast and lands in
  the log instead of hanging the SessionStart hook.

## [0.2.0] - 2026-07-03

### Added
- Explicit replay/live gating: the SPA gates toasts on the framework's
  `caught-up` SSE marker (cc-interact v0.1.10), so a tab remount, HMR, or
  StrictMode double-mount replays history silently instead of storming
  notifications.
- Asset-store GC on close: closing an artifact deletes blobs unreferenced by
  any open subject, honoring a 15-minute mtime grace window; an idempotent
  re-upload refreshes the window so a re-referenced asset is never collected.
- Honest channel presence: `ClaudePID` now resolves the real Claude session
  process via a gopsutil ancestry walk (ported from cc-review), so
  `channel: active` means the MCP channel is attached and `pending`/`inactive`
  signal a real absence.
- `examples/quickstart-board.json`, a quickstart-sized approval board, plus
  `docs/scripts/demo.sh`, which regenerates the README demo screenshot from a
  real run of it.

### Changed
- README restructured to the front-door skeleton: brew as the one install
  path, a quickstart that drives the installed binary against the committed
  example, and internals deferred to `docs/contract.md`.

### Fixed
- The stray second LIVE pill: the library notification bar's hardcoded
  connection badge no longer renders alongside the header's own.

## [0.1.0] - 2026-07-03

### Added
- Lazy per-user daemon (`~/.cc-present`) with a per-artifact append-only event
  log in SQLite, a REST endpoint for human interactions, and an SSE stream that
  replays the log from seq 0. State is a pure reduction, with the agent
  document lane and the human interaction lane kept separate.
- Typed-block web SPA (Vite + React, embedded via `go:embed`): section, card,
  approval, choice, input, markdown, code, diff, image, table, and progress
  blocks, with a document-level Submit.
- CLI surface: `start`, `push`, `update-block`, `remove-block`, `reply`,
  `outcomes`, `close` for the agent side, plus `watch`, `status`, and `stop`
  from the cc-interact substrate.
- Claude Code plugin: the cc-present MCP channel, a SessionStart hook that
  installs the binary and records the session, and the `present` skill driving
  the loop: start, watch, reply, outcomes. The repo doubles as its own plugin
  marketplace.
- `examples/opener-board.json`, a complete sample document.

[Unreleased]: https://github.com/yasyf/cc-present/compare/v0.6.1...main
[0.6.1]: https://github.com/yasyf/cc-present/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/yasyf/cc-present/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/yasyf/cc-present/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yasyf/cc-present/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yasyf/cc-present/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/yasyf/cc-present/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/yasyf/cc-present/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yasyf/cc-present/releases/tag/v0.1.0
