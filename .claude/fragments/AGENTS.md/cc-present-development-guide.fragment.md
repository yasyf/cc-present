# cc-present Development Guide

Ad-hoc live web artifacts for Claude sessions — approval boards, choices, and rich content whose every click streams back to the agent. Distributed via Homebrew: `brew install yasyf/tap/cc-present`.

## Repository Structure

```
cc-present/
├── cmd/cc-present/   # main package — the CLI entry point
├── internal/
│   ├── cli/               # cobra command tree: start, push, update-block, remove-block,
│   │                      #   reply, outcomes, close + cc-interact scaffold (daemon, watch,
│   │                      #   status, stop, session-record, channel-ack, channel)
│   ├── app/               # paths/launcher/client/cmd.Deps wiring (cc-interact consumer scaffold)
│   ├── daemon/            # buildServer: domain ops (start/push/upsert/remove/reply/close) + REST
│   ├── doc/               # block schema structs + Validate
│   ├── state/             # event reducer: doc + human interactions (shared JSON fixtures)
│   ├── packs/             # block-pack discovery, manifest + schema validation, registry
│   ├── assets/            # content-addressed image store + routes
│   ├── web/               # go:embed of the built SPA (committed placeholder dist/index.html)
│   ├── version/           # build version, stamped via -ldflags
│   └── log/               # slog setup
├── web/                   # Vite + React SPA (block renderer), builds into internal/web/dist
├── ios/                   # SwiftUI iOS client — CcPresent.xcodeproj (app + tests) + CcPresentKit SPM package
├── plugin/                # Claude Code plugin payload: manifest, hooks, skills/{present,author-pack}
├── examples/              # sample block documents (opener-board.json)
│   └── packs/example/     # reference block pack: manifest, schemas, JS bundle, examples
├── docs/                  # contract.md — the wire contract (blocks, events, REST) + README assets
├── .claude-plugin/        # marketplace.json — this repo is its own plugin marketplace
├── .github/               # GitHub Actions workflows
├── AGENTS.md              # This file — shared conventions
└── README.md              # Project overview
```

Block packs — plugin-supplied typed blocks — follow the manifest, schema, and serving conventions in `docs/contract.md`; `examples/packs/example/` is the reference pack, and `cc-present pack lint <dir>` validates a pack root.

## iOS / Swift (ios/)

The `ios/` tree is a native SwiftUI client, kept separate from the Go layer. Tests use **Swift Testing** (`@Test` / `#expect`), never XCTest. **SwiftFormat** owns mechanical formatting and **SwiftLint** owns the judgment rules; both run warnings-only — style alone never blocks a commit or fails CI. Drive builds, tests, and simulators through **XcodeBuildMCP** (the `xcodebuildmcp` CLI / MCP), the sanctioned build driver, rather than raw `xcodebuild` / `xcrun` / `simctl`. The Xcode project is a synced-folder `project.pbxproj` (objectVersion 77, fixed synthetic UUIDs, `PBXFileSystemSynchronizedRootGroup` — new files land on disk with no project edit); hand-edit build-setting *values* only — the sole committed object-graph edits are the Info.plist membership-exception set (keeping the custom `Info.plist` out of Copy Bundle Resources) and the CcPresentKit local-package wiring (landing later) — and never let Xcode or any tool regenerate the file.

**TestFlight uploads** run through the `TestFlight (iOS)` workflow (`.github/workflows/testflight-ios.yml`), dispatch-only (`workflow_dispatch`) — no push or PR trigger. It archives and exports with cloud signing (`-allowProvisioningUpdates` plus App Store Connect API-key auth; no certs or profiles in CI) straight to App Store Connect. Four repo secrets drive it: `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY_P8` (the `.p8` private key), and `APPLE_TEAM_ID`. One-time manual setup: create the App Store Connect app record for `com.yasyf.cc-present`, and mint an API key with the **App Manager** role.
