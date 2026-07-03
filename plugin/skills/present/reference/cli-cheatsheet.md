# CLI cheatsheet

Every command is a thin call to the local daemon, which lazy-starts on first use. Document and block JSON always travels as a file path (or `-` for stdin), never inlined in the argument list. `--session`/`--cwd` resolve the artifact for this window and scope; the artifact commands default `--session` to `$CLAUDE_CODE_SESSION_ID`, but `watch`, `status`, and `channel-ack` do **not** — pass it explicitly.

| Command | What it does |
|---|---|
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" start --session <id> --cwd <dir> [--doc <file\|->] [--title <t>] [--new]` | Create or resume this scope's artifact and print exactly three lines: `session: <subject-id>`, `url: http://127.0.0.1:<port>/p/<slug>--<hash>`, `channel: active\|pending\|inactive` (`active`: channel proven — skip the Monitor; `pending`/`inactive`: arm the Monitor on `watch`). `--doc` seeds the document (validated first); `--title` names the URL slug when no `--doc` is given; `--new` forces a fresh artifact. A resume that lands on a closed artifact goes fresh automatically. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" push <file\|-> [--dry-run]` | Replace the whole document; prints `revision: <n>`. `--dry-run` validates offline (no daemon, no artifact needed), prints `ok` or the first error naming the offending block id, and exits non-zero on failure. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" update-block <file\|-> [--after <id>]` | Insert or replace a single block (the file holds one block's JSON). An existing id is replaced in place, wholesale; a new one inserts after `--after`, appending when absent or unknown. Silent on success. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" remove-block <id>` | Remove a top-level block by id. Unknown id is a no-op. Silent on success. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" reply --block <id> --body <markdown>` | Append an agent reply to a block's thread; renders in the browser immediately. Returns at once; silent on success. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" outcomes` | Print the reduced state as indented JSON: the current document plus `interactions` (decisions, choices, inputs, feedback, replies, submitted, closed). The post-submit drain. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" close [--summary <text>]` | Terminally close the artifact, recording `--summary` on the `present.closed` event; prints `closed: <slug>`. After close, browser interactions get 409 and agent writes are refused. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" watch --session <id> --cwd <dir> [--once]` | Print one JSON event per line, each payload self-describing via its embedded `type`, and exit on the terminal `present.closed`. It delivers human events, presence frames, and that terminal close; your own agent writes never appear. Resumes from a persisted cursor, so a restart re-delivers nothing already emitted. `--once` exits after the first event instead. Run it under a persistent Monitor. `--session` is required in practice: without it, `watch` polls for a subject forever. |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" status [--session <id> --cwd <dir>]` | Show the daemon version, its HTTP address, and this scope's subject (`open`/`closed`, or `none`). |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" stop` | Stop the background daemon (it lazy-respawns on the next command). |
| `"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" channel-ack --session <id> --cwd <dir>` | Mark this window's channel proven after the first delivered `<channel>` tag, flipping later `start`s to `channel: active`. Run it once when a tag arrives while the Monitor is armed, then stop the Monitor. |

`image` blocks whose `src` is a local file path are inlined by `start --doc`, `push`, and `update-block`: the file is read (5 MiB cap), stored content-addressed on the daemon, and the src rewritten to its `asset:<sha256>` ref. `push --dry-run` performs the same read locally, so a missing or oversized image fails the dry run too.

The remaining subcommands — `daemon`, `channel`, `session-record`, `guard-edit`, `completion` — are plumbing the plugin's hooks and MCP config invoke; never run them from the skill flow.

The daemon keeps state under `~/.cc-present` (sqlite db, control socket, HTTP handshake, per-consumer cursors).
