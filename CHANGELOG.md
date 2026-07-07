# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[Unreleased]: https://github.com/yasyf/cc-present/compare/v0.3.0...main
[0.3.0]: https://github.com/yasyf/cc-present/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/yasyf/cc-present/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/yasyf/cc-present/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yasyf/cc-present/releases/tag/v0.1.0
