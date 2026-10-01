# pr pack blocks

Three block types under the `pr` pack put a GitHub pull request on a board:
`pr.card` for the PR itself, `pr.diff` for an excerpt of its diff, and
`pr.commits` for its commit subjects. Reference them by dotted wire type inside
any `Doc.blocks` array or a card's `children`. None is interactive.

## Resolve before you push

You write only `repo`, `number`, and the `pr.diff` selectors. The pack's
resolver fills in the rest from `gh`, so the board shows what GitHub says, not
what you remember:

```bash
PACK=$(cc-present pack list | awk '$1 == "pr" { getline; print $2 }')
bun "$PACK/scripts/resolve.ts" --write board.json
cc-present push board.json
```

The resolver walks a whole document, a block array, or a single block file, so
the same command prepares an `update-block` file. It fills every `pr.card` and
`pr.commits` block on each run, and every `pr.diff` block that has no `patch`
yet. Every resolved block carries `fetched_at`, the UTC instant the data left
GitHub, and the block renders it.

- One GraphQL call per PR serves both `pr.card` and `pr.commits`; a closed,
  unmerged PR adds one REST commit search to find a merge queue's `(#N)`
  squash. A diff is one REST call.
- Responses cache for ten minutes under `$XDG_CACHE_HOME/cc-present/pr`,
  which defaults to `~/.cache/cc-present/pr`. `--refresh` skips the cache.
- Without `--write` the resolved JSON goes to stdout; `-` reads stdin.
- An unresolved block fails `push --dry-run` on its missing required fields,
  such as `title` on a card or `patch` on a diff.

## pr.card

One pull request in compact form: state, title, author, branches, line counts,
CI summary, review verdicts, labels, and the merged or landed commit.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"pr.card"` | yes | The dotted wire type. |
| `repo` | string | yes | `owner/name`. |
| `number` | integer | yes | PR number. |
| `note` | string | no | Markdown under the title: why this PR is on the board. You write it; the resolver keeps it. |
| `title` | string | resolved | PR title. |
| `url` | string | resolved | PR URL. |
| `author` | string | resolved | Author's GitHub handle. |
| `state` | `open` \| `draft` \| `merged` \| `landed` \| `closed` | resolved | `landed` is a closed PR whose `(#N)` squash commit is on the default branch, the shape a merge queue leaves; commit search reads only the default branch. |
| `base`, `head` | string | resolved | Branch names. |
| `merged_sha` | string | resolved | Merge commit, or the landed squash commit. |
| `additions`, `deletions`, `changed_files` | integer | resolved | Line and file counts. |
| `checks` | object | resolved | Head commit's rollup: `state`, `total`, `passed`, `failed`, `pending`, `skipped`, and up to ten `failing` check names. |
| `review_decision` | `approved` \| `changes_requested` \| `review_required` \| `none` | resolved | Branch protection's verdict. |
| `reviews` | `{author, state}[]` | resolved | Each reviewer's latest approving or blocking review. |
| `labels` | string[] | resolved | Label names. |
| `fetched_at` | date-time | resolved | When the data was fetched. |

Before resolving:

```json
{ "id": "pr-8", "type": "pr.card", "repo": "yasyf/cc-present", "number": 8, "note": "Bumps the interaction library this release depends on." }
```

After resolving, as in the pack's `examples/card.json`, with `note` kept:

```json
{
  "id": "pr-8",
  "type": "pr.card",
  "repo": "yasyf/cc-present",
  "number": 8,
  "note": "Bumps the interaction library this release depends on.",
  "title": "deps: cc-interact v0.35.1 — build the daemon client identity once per process",
  "url": "https://github.com/yasyf/cc-present/pull/8",
  "author": "yasyf",
  "state": "merged",
  "base": "main",
  "head": "deps-cc-interact-v0-35-1",
  "merged_sha": "8bc5b8ac0311411a028dea616dc123645ac18e7e",
  "additions": 3,
  "deletions": 3,
  "changed_files": 2,
  "checks": { "state": "success", "total": 12, "passed": 9, "failed": 0, "pending": 0, "skipped": 3 },
  "review_decision": "none",
  "reviews": [],
  "labels": [],
  "fetched_at": "2026-10-01T08:01:52.638Z"
}
```

## pr.diff

An excerpt of a PR's unified diff, one syntax-highlighted panel per file. The
block collapses past `max_lines` behind a "Show N more lines" control, and its
header opens and closes the whole excerpt.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"pr.diff"` | yes | The dotted wire type. |
| `repo`, `number` | string, integer | no | The PR the excerpt comes from; both or neither. Required for the resolver to fetch a patch. |
| `title` | string | no | Header text; defaults to `Diff excerpt`. |
| `note` | string | no | Markdown above the diff: what to look at. |
| `paths` | string[] | no | Resolver selector. Each entry is a glob (`go/**/*.go`) or a path or directory prefix; files appear in the order of the first entry that matches them, so list the most decisive file first. |
| `match` | string | no | Resolver selector. A regular expression; only hunks whose text matches stay, and files left with no hunks drop. |
| `max_lines` | integer, 5 to 400 | no | Lines shown before "show more"; defaults to `60`. |
| `patch` | string, at most 64 KiB (65,536 bytes) | yes | Unified diff text. The resolver fills it from the selectors; you can pin one inline instead, and the resolver leaves a present `patch` alone. |
| `fetched_at` | date-time | resolved | Set when the resolver fills `patch`. |

Before resolving:

```json
{
  "id": "pr-8-diff",
  "type": "pr.diff",
  "repo": "yasyf/cc-present",
  "number": 8,
  "title": "Dependency pin",
  "paths": ["go.mod", "go.sum"],
  "match": "cc-interact",
  "max_lines": 40
}
```

A pinned excerpt needs no resolver run:

```json
{ "id": "pin", "type": "pr.diff", "title": "README.md", "patch": "--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old opener\n+new opener" }
```

The resolver refuses a selection that matches nothing or exceeds 64 KiB; narrow
`paths` or `match` and rerun it.

## pr.commits

A PR's commit subjects, oldest first, each linked to its commit. The resolver
reads the last 100 commits; when a PR has more, the block says how many earlier
ones it omits.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"pr.commits"` | yes | The dotted wire type. |
| `repo` | string | yes | `owner/name`. |
| `number` | integer | yes | PR number. |
| `title` | string | no | Header text; defaults to `Commits`. |
| `commits` | `{sha, subject, author}[]` | resolved | One entry per commit, 1 to 100. |
| `total` | integer | resolved | The PR's full commit count. |
| `fetched_at` | date-time | resolved | When the data was fetched. |

```json
{ "id": "pr-8-commits", "type": "pr.commits", "repo": "yasyf/cc-present", "number": 8 }
```
