# Troubleshooting

**The binary is missing.** Run `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh"`. It downloads the release binary for this platform into `bin/cc-present`. The SessionStart hook runs it automatically on session start, and `mcp-channel.sh` runs it again before exec'ing the channel server, so a missing binary usually means both paths failed — run it by hand and read its stderr.

**`watch` prints nothing and never exits.** You omitted `--session`. The artifact commands default it from `$CLAUDE_CODE_SESSION_ID`; `watch` takes only the flag, and with an empty session it polls for a subject forever. Relaunch it as `watch --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD"`. Confirm the Monitor wrapping it was launched with `persistent: true`, and check the daemon with `status`.

**The daemon looks stale or version-skewed.** `status` prints the running daemon's version. Every command self-heals skew: an older daemon is evicted and replaced on the next invocation (same-or-newer wins). To force it, `stop` — the next command lazy-respawns a fresh daemon. State survives: the sqlite log, cursors, and the artifact all persist across restarts.

**`channel: pending` or `inactive` printed.** Expected in v1 — the channel is opt-in and unproven until its first delivered tag. Arm the Monitor on `watch` and proceed. If a `<channel source="cc-present">` tag does arrive while the Monitor runs, run `channel-ack --session "$CLAUDE_CODE_SESSION_ID" --cwd "$PWD"`, TaskStop the Monitor, and rely on tags; future `start`s in this window then print `active`.

**The URL stopped working.** The daemon binds a new port when it restarts (upgrade, `stop`, reboot), so a URL from before the restart is dead. Re-run `start` — it resumes the same artifact and reprints the URL on the current port. Give the user the fresh URL verbatim.

**`push` (or `start --doc`) was rejected.** The validation error names the offending block id — a duplicate id, an unknown type, a missing required field, a bad `progress` range, an over-cap image or document. Fix that block in the scratchpad file and re-push; iterate offline with `push --dry-run` until it prints `ok`. A rejected push changes nothing — the board keeps rendering the last good document.

**Browser clicks fail after close / HTTP 409.** `close` is terminal. The REST edge answers any later interaction with `409 presentation is closed`, and a click that raced the close lands in the log as a harmless no-op — recorded outcomes are unaffected. Agent writes are refused too (`artifact <slug> is closed`). There is no un-close: to present again, run `start` — it detects the closed artifact and creates a fresh one.

**`no cc-present artifact for this scope`.** `push`, `update-block`, `reply`, and friends need an existing artifact. Run `start` first; it prints the `session:`/`url:`/`channel:` lines and the rest of the commands resolve from there.

**The Monitor stopped under an event flood, or replayed old events.** Monitors stop themselves under a high event rate — re-arm it; `watch` resumes from its cursor, so nothing is lost. Delivery is at-least-once, so a re-armed Monitor (or a channel/Monitor overlap) can replay frames you already handled: replays are harmless, since decisions, choices, and inputs are last-write-wins and feedback dedupes by its `id`.

**An image block failed to inline.** `image "<path>" not found` means the src is a local path the CLI can't read from your cwd — use an absolute path. `exceeds 5242880` means the file is past the 5 MiB asset cap — compress it, or host it and use an `https://` src.
