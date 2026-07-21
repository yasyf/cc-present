# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.14.2] - 2026-07-21

### Added
- Explicit round intent. A `push` or `update-block` that adds a new top-level
  block to a mid-review round — dirty, and the human has already interacted —
  is rejected until the agent declares intent: `--round current` extends the
  review in progress, `--round new` closes the round into history and opens
  the next with the unanswered blocks carried forward (`round.started` gains a
  `carry` payload all three reducers restamp), and `--round-title` names the
  round it opens. `cc-present round` itself now always carries, so advancing
  never freezes the human's open controls.

### Fixed
- A `round.started` carry id that stops naming a top-level block between the
  daemon's snapshot and the append is skipped instead of failing the
  reduction, so a concurrent removal can no longer poison a subject's replay.
- A cleared choice (empty selection, no write-in) and an empty input text now
  count as unanswered when computing carry — an owed control rides into the
  new round instead of freezing as answered.
- A card child promoted to top level in a push counts as a new top-level block
  for the round-intent guard.
- Tailnet display URLs advertise nothing when every bound leg is stale against
  the live self-address set; a dead port is never named, and reconcile revives
  a live leg on its next pass.
- Triage item visuals resolve in `doc.Locate`, so pointing errors name the
  enclosing triage block.

### Changed
- Repinned daemonkit to the exact session-v1 release.

## [0.14.1] - 2026-07-21

### Changed
- Bumped the embedded cc-interact daemon to v0.14.0: each watcher tracks its own
  watch cursor, and connected watchers are now listed in `cc-present status`.
- Repinned daemonkit to its hard-cut release.

### Fixed
- The revising working set now clears wholesale at round boundaries (`submit`
  and `round.started`) in all three reducer ports, so a rewrite delivered as a
  new block no longer leaves the announced ids' banner dangling forever;
  `revising --clear` is the explicit escape, equivalent to the documented bare
  call.
- `start` and `push` print one tailnet URL per name on a canonical port (live
  self-address leg, then the primary port, then lowest) instead of one URL per
  scattered leg port.

## [0.14.0] - 2026-07-21

### Added
- Two built-in review blocks across all three surfaces (daemon, web, iOS).
  `draft` presents a document as numbered source lines a human annotates by
  selecting a line or range: each annotation carries a content-hashed line
  anchor (`12-18#f6zy`), so notes re-anchor across agent redrafts — a moved
  line keeps its marker with a "was L12" tag, a vanished one drops to a
  Detached notes section with its frozen quote. The daemon validates each
  anchor resolves against the current text and stamps the quote server-side.
  `triage` puts per-item accept/reject verdicts on a list of up to 50 rich
  items (markdown, facts, detail, visuals), with optional per-item notes,
  Accept all / Reject all, and submit gating on every item decided. Closed
  rounds tally triage verdicts item-by-item alongside approvals.
- The line-anchor scheme is now a wire contract (`docs/contract.md`
  "Line anchors"), with the daemon consuming the canonical
  `github.com/yasyf/cc-context/anchor` package and the SPA and iOS carrying
  conformant ports pinned by a shared fixture corpus.

## [0.13.0] - 2026-07-20

### Added
- Four display primitives, each working at the top level, inside a card, and as
  an `option.visual`: `chart` (bar or line data rendered as a themed SVG, at
  most 6 series over 100 categories, values aligned to categories and the axis
  anchored at 0), `term` (a command plus its ANSI-colored output, capped at
  32 KiB), `filetree` (relative-path entries built into a collapsible tree with
  `added`/`modified`/`removed` badges, at most 200 entries), and `record` (one
  entity's labeled profile: up to 16 facts — label required — 8 tone chips, and
  8 https-only links). The shared chip validation also tightens: a card with an
  empty chip label now fails validation instead of rendering an empty chip.

### Changed
- Tailnet URLs without a certificate now print the machine's bare MagicDNS
  label (`http://yasyf-home:52668/…`) instead of raw tailnet IPs, one URL per
  distinct leg port. The bare label sits outside the `ts.net` HSTS preload, so
  browsers open it over plain http, and synckit v0.23.0 trusts it as an origin
  alongside the full name. Raw-IP URLs remain only when tailscale reports no
  usable name — down, or a collision-quarantined MagicDNS name.

## [0.12.1] - 2026-07-20

### Fixed
- Tailnet URLs open in browsers now. `ts.net` sits on the HSTS preload list, so
  the `http://<name>.ts.net` URLs v0.11.0 printed could never load — browsers
  force-upgrade them to `https://` against a plaintext listener. Tailnet legs
  now serve TLS and plain HTTP on one port by sniffing each connection's first
  byte: browsers get `https://<magicdns>:<port>` with a real certificate minted
  through `tailscale cert` (asynchronously at start, renewed by the reconcile
  pass, rotated without rebinding), while mesh clients keep dialing the same
  legs over plain http by IP. Printed URLs always match what serves — the https
  name URL appears only while a valid certificate for the live cert domain is
  held, the HSTS-exempt `http://<tailnet-ip>` form prints otherwise, and the
  `http://<name>.ts.net` form is never composed again. A specific non-loopback
  bind no longer advertises tailnet IPs nothing serves.

## [0.12.0] - 2026-07-20

### Added
- A dead event stream shows a visible "Can't connect to this board" panel after
  ~8 seconds instead of an eternal loading skeleton — a hard `/events` failure
  (a mistyped slug, a stopped daemon) previously rendered identically to
  loading, forever.
- Universal escape hatch on every choice. A chrome-guaranteed write-in "Other"
  row (takes a 1–9 index, commits on Enter/blur) and an "Add note" thread on any
  choice — authors can no longer box the human into their listed options.
  `choice.selected` carries `other`, `feedback.created` accepts choice targets
  (threaded, survives re-picks), and a write-in alone counts as decided
  everywhere: tally, step status, receipts, and round snapshots.
- Live revision loop. A new agent-origin `revising.changed` event (and
  `cc-present revising <ids…> --note`) announces which steps are being rewritten
  after a consequential pick. Clients show a pulsing rail dot and a warn-only
  banner — controls stay live — that decays passive after 120 seconds; the
  completing `update-block` clears the mark and lands an "Updated after your
  earlier pick" callout, and a fresh-id upsert badges "Claude added this step."
  The authoring skill's choreography replaces verbal conditionals and
  hand-authored "Updated:" prefixes.
- Visual-first decisions. The `diagram` block and `option.visual` (shipped in
  0.11.0) gain a per-step visual stage that tracks the active option. Mermaid
  ink now uses Blue Pencil theme tokens and re-inks on theme flip; broken
  source degrades to an error card with the source visible. iOS diagram
  rendering reaches parity through the shared single-block webview, and code
  blocks gain Highlightr syntax color. `push` prints a non-blocking nudge when
  a board's choices ship without a single visual.
- `recommended` on options — a validated schema field (at most one per
  single-select) rendered as a stamp badge, replacing free-text hint prefixes.
- Momentum. Auto-advance generalizes from lone approvals to lone single-select
  choices (450 ms cue, any interaction cancels); the header reads
  "Step N of M · K decisions left" with both numbers live-derived; a segmented
  tap-to-jump strip appears past 10 steps.

### Changed
- Focus-mode cards are question-first. The prompt is a real heading pinned
  above the scroll region (which also fixes titles clipping under sticky diff
  and table headers), the tier eyebrow is gone, context clamps to ~6 lines with
  expand-in-place, and heavy context blocks collapse to titled disclosures.
  Facts render as a comparative grid when labels align across options. The
  submit bar mounts only at the Review summary, whose receipts show chosen
  labels and write-in text.
- iOS ships all of the above in lockstep: question-first hierarchy, write-ins
  and notes, generalized auto-advance, revision affordances, and webview
  diagrams, conformance-tested against the shared reducer fixtures.

### Fixed
- Mermaid node labels rendered invisible: mermaid honors `htmlLabels` only at
  the top level of its config (the nested flowchart setting is silently
  ignored), so labels emitted as HTML were stripped by SVG sanitization. Labels
  now render as SVG text, and theme colors are probe-resolved so `light-dark()`
  tokens ink correctly in both themes.
- Single-block pages honor `?theme=dark|light`, and the iOS webview passes its
  appearance through the URL and reloads on trait flips — diagrams no longer
  render light-on-light in dark mode.
- The daemon caps `input.submitted` text at 64 KiB.

## [0.11.0] - 2026-07-19

### Added
- Tailnet display URLs. `start` and `push` print `tailnet:` line(s) whenever the
  daemon serves the tailnet — the MagicDNS name when tailscale publishes one
  (deduped by port, so v4+v6 legs on one port yield one URL), raw tailnet IPs
  otherwise, and under `pair`'s wildcard bind the primary port itself. `push`
  now prints `url:` too, after `revision:`. The `tailnet:` URL is what to hand
  another mesh machine — never a `tailscale serve` proxy, which the contract
  bans.
- Dynamic tailnet legs. A 30-second reconcile binds legs gained after boot, so
  a daemon started before `tailscale up` grows tailnet reach without a restart;
  the handshake's `extra_addrs` rewrites atomically as legs bind (cc-interact
  v0.12.0), and legs are never pruned — a vanished address is inert. Synckit
  state created after daemon start still needs one restart.
- `GET /api/health` returns `{"version":…}` — the one non-vacuous liveness
  probe, since the SPA fallback answers unmatched paths 200. Unknown `/api/*`
  paths return 404, never the shell.
- The `diagram` block — text-to-diagram, `kind: mermaid`, source capped at
  8 KiB — rendered lazily client-side and through the iOS single-block
  webview; and `option.visual`, attaching one `code`, `diagram`, `image`, or
  `diff` block per choice option, rendered inside the option's detail.

### Fixed
- A binary built without the web step embedded a Vite shell referencing
  gitignored assets and served it as `text/html` for every asset request — a
  blank board with a strict-MIME console error. `internal/web/dist` is
  build-output-only now (`.gitkeep` restored by the vite build keeps `go:embed`
  compiling): the daemon refuses to start on a missing or internally
  inconsistent embed, and asset-shaped misses 404 instead of answering with the
  shell.

## [0.10.0] - 2026-07-17

### Added
- Synckit mesh trust. When `~/.config/synckit/state.json` exists, the daemon
  automatically trusts every machine in the synckit host registry: their tailnet
  IPs (resolved via `tailscale status`, refreshed on a 30s TTL, failing closed to
  an empty set) reach boards with no bearer token, and with a loopback bind the
  daemon additionally listens on its own tailnet addresses (best-effort, recorded
  in the handshake's `extra_addrs`). A read-only `cc-present trust` inspector
  shows the detected mesh, resolved IPs, and live listeners. Token auth, `pair`,
  and the loopback bypass are unchanged; contract.md § Authentication documents
  the three acceptance paths and bans `tailscale serve`/Funnel by name.

### Security
- The cc-interact bump (v0.6.0 → v0.9.0) picks up the upstream Origin-gated
  loopback bypass and the new `Sec-Fetch-Site: cross-site` rejection on
  origin-less requests — CSRF hardening for both no-token paths.

## [0.9.4] - 2026-07-14

### Fixed
- Mobile option rows now respect the keymap chip and the touch floor. When facts
  fold below the body on narrow screens — and on factless rows at any width — a long
  option label no longer runs under the 1–9 keymap chip; the folded facts row drops a
  stray gap left by a chip-avoidance rule the cascade could never apply; and the
  detail and lightbox modal close buttons reach the 44px touch target the mobile
  layout enforces everywhere else.

## [0.9.3] - 2026-07-14

### Added
- Context-rich option primitives. `choice` options and `approval` blocks carry a
  scannable `facts` cluster (`{label?, value, tone?}`, tone tinting the outlier) and
  an expandable `detail` drill-down (`{pros?, cons?, md?, mode?}`) — pros and cons
  plus a full markdown body, revealed inline (joining the expand-all key) or in a
  modal. Two tiers: `label` + `hint` + `facts` + the clamped `md` stay scannable up
  front; `detail` opens on demand, so depth never costs clarity. The `present` skill
  now says never present an option blind — every option must carry the tradeoffs a
  person needs to decide — and the `opener-board.json` example models it. Non-breaking:
  existing `label`/`hint`/`md` are untouched, and there are no reducer, event, or REST
  changes, since drill-down is pure client state.

## [0.9.2] - 2026-07-14

### Changed
- The present and author-pack skills invoke bare `cc-present` — Claude Code ≥ 2.1.91 puts
  each plugin's `bin/` on the Bash tool PATH; `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present"` remains
  the documented fallback when the bare name is missing or a stale shell snapshot pins an
  older version.

## [0.9.1] - 2026-07-13

### Changed
- Artifacts are window-owned. The daemon canonicalizes every request's scope to
  a constant sentinel, so an artifact is keyed by its Claude window (session id
  plus claude pid) and every command resolves it from any working directory.
  Previously the scope was the raw cwd of `start`, and a command run from a
  different directory failed with `no cc-present artifact for this scope`.
  One window now holds one live board: `start` resumes it and `start --new`
  replaces it, from anywhere; session-rotation rebind no longer depends on the
  SessionStart hook's cwd; `--cwd` is accepted and recorded but no longer
  affects resolution; the missing-artifact error now reads `no cc-present
  artifact for this window`. Boards created by earlier daemons keep their
  directory scopes and are not resolvable after the upgrade — they linger in
  the open-sessions list until cleaned up by hand.

## [0.9.0] - 2026-07-13

### Added
- Pack host API v2: pack renderers get `ui.tokens`, `ui.toast`, `usePackState`,
  and block context. A manifest's `host_api` is now a floor, so the daemon
  loads any pack whose minimum it meets; the example pack's survey wizard
  demonstrates the v2 surface.
- Blue Pencil web UI: editorial palette, drawn-mark confirmations, sign-off
  tally, exhaustive lifecycle states (toasts, skeletons, placeholders), richer
  content blocks (highlighted diffs, code headers with copy, table overflow,
  lightbox, expand-all), and phone-width breakpoints. The iOS client adopts
  the palette and decided-row receipts.

### Changed
- CLI ergonomics: `push --dry-run` reports every violation at once and
  `update-block` gains `--dry-run`; `outcomes --no-doc` omits the document;
  JSON decode errors carry line and column.

## [0.8.1] - 2026-07-13

### Fixed
- `install-binary.sh` reads only the plugin's own version from `plugin.json`;
  the 0.8.0 dependencies block added a second `version` key that corrupted the
  release download URL, so fresh installs got no binary.

## [0.8.0] - 2026-07-13

### Changed
- Declare the [captain-hook](https://github.com/yasyf/captain-hook) plugin as a
  dependency and attach the guard pack with `uvx --isolated`. captain-hook is
  the sole hook dispatcher, so installing cc-present now pulls it in and the
  Artifact guard fires without extra setup. Add the `yasyf/captain-hook`
  marketplace before installing cc-present.

## [0.7.0] - 2026-07-13

### Added
- Bundled capt-hook guard pack: blocks the built-in `Artifact` tool and steers
  presentation to a cc-present live board (attached at session start while the
  plugin is enabled; pinnable as `github:yasyf/cc-present@latest`).

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

[Unreleased]: https://github.com/yasyf/cc-present/compare/v0.14.1...main
[0.14.1]: https://github.com/yasyf/cc-present/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/yasyf/cc-present/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/yasyf/cc-present/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/yasyf/cc-present/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/yasyf/cc-present/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/yasyf/cc-present/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/yasyf/cc-present/compare/v0.9.4...v0.10.0
[0.9.4]: https://github.com/yasyf/cc-present/compare/v0.9.3...v0.9.4
[0.9.3]: https://github.com/yasyf/cc-present/compare/v0.9.2...v0.9.3
[0.9.2]: https://github.com/yasyf/cc-present/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/yasyf/cc-present/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/yasyf/cc-present/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/yasyf/cc-present/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/yasyf/cc-present/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/yasyf/cc-present/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/yasyf/cc-present/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/yasyf/cc-present/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/yasyf/cc-present/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yasyf/cc-present/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yasyf/cc-present/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/yasyf/cc-present/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/yasyf/cc-present/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yasyf/cc-present/releases/tag/v0.1.0
