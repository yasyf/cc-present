@testable import CcPresentUI
import CcPresentKit
import Foundation
import IOSKitUI
import SwiftUI
import Testing
import UIKit

private let fixture = #"""
[
  {
    "id": "section-rollout",
    "type": "section",
    "title": "Ledger sync rollout",
    "md": "One block of every type, rendered for **screenshot proof**."
  },
  {
    "id": "card-cutover",
    "type": "card",
    "title": "Cut the sync worker over to batched writes",
    "summary": "Batches flush every **200 ms** or at 500 rows, whichever comes first.",
    "chips": [{"label": "worker"}, {"label": "needs review", "tone": "flag"}, {"label": "staging", "tone": "demo"}],
    "status": "open",
    "children": [
      {
        "id": "card-cutover-notes",
        "type": "markdown",
        "md": "The worker now groups row writes per account before it commits, which cuts round trips by roughly **8x** on the staging replay."
      },
      {
        "id": "card-cutover-approval",
        "type": "approval",
        "prompt": "Ship batched writes behind the existing flag?"
      }
    ]
  },
  {
    "id": "approval-retention",
    "type": "approval",
    "prompt": "Drop the 90-day export archive after the migration?",
    "detail": {
      "pros": ["Frees 1.2 TB of cold storage", "Removes a nightly job"],
      "cons": ["Exports older than 30 days need a manual restore"],
      "md": "The archive has been read twice in the last year, both times by the same quarterly report.",
      "mode": "inline"
    }
  },
  {
    "id": "choice-conflicts",
    "type": "choice",
    "prompt": "How should the client resolve a conflicting edit?",
    "options": [
      {
        "id": "last-write",
        "label": "Last write wins",
        "hint": "Simplest; silently drops the older edit",
        "facts": [
          {"label": "Effort", "value": "1 day", "tone": "good"},
          {"label": "Data loss", "value": "possible", "tone": "bad"},
          {"label": "Latency", "value": "unchanged"}
        ]
      },
      {
        "id": "field-merge",
        "label": "Merge per field",
        "hint": "Keeps both edits when they touch different fields",
        "recommended": true,
        "detail": {"pros": ["No silent loss for disjoint edits"], "cons": ["Needs a field-level clock"]}
      },
      {
        "id": "prompt-user",
        "label": "Ask the person editing",
        "md": "Shows both versions side by side and lets them pick."
      }
    ]
  },
  {
    "id": "draft-notes",
    "type": "draft",
    "lang": "markdown",
    "title": "Release notes",
    "text": "# Ledger 4.2\n\n- Sync now batches writes, so large imports finish faster.\n- Conflicting edits on different fields are merged instead of dropped.\n- The export archive keeps 30 days of history."
  },
  {
    "id": "triage-alerts",
    "type": "triage",
    "prompt": "Which alerts should page after the cutover?",
    "allowNotes": true,
    "items": [
      {
        "id": "alert-lag",
        "label": "Replication lag over 30 s",
        "hint": "Fired 4 times last week",
        "facts": [{"label": "Noise", "value": "low", "tone": "good"}, {"label": "Owner", "value": "sync"}]
      },
      {"id": "alert-queue", "label": "Write queue above 10k rows", "hint": "New with batching"},
      {"id": "alert-errors", "label": "Commit error rate above 1%", "md": "Measured over a five-minute window."}
    ]
  },
  {
    "id": "input-window",
    "type": "input",
    "label": "Preferred maintenance window",
    "placeholder": "e.g. Tuesday 02:00 to 04:00 UTC",
    "multiline": true
  },
  {
    "id": "markdown-context",
    "type": "markdown",
    "md": "## Why now\n\nImports over **50k rows** time out on the current worker. Batching fixes that without a schema change:\n\n- one commit per account per flush\n- retries reuse the same `batch_id`\n- no client release needed"
  },
  {
    "id": "code-flush",
    "type": "code",
    "lang": "go",
    "title": "internal/sync/batch.go",
    "code": "func (b *Batcher) Flush(ctx context.Context) error {\n\trows := b.drain()\n\tif len(rows) == 0 {\n\t\treturn nil\n\t}\n\treturn b.store.CommitBatch(ctx, rows)\n}",
    "highlight": "2-5"
  },
  {
    "id": "diff-cache",
    "type": "diff",
    "title": "internal/store/cache.go",
    "diff": "--- a/internal/store/cache.go\n+++ b/internal/store/cache.go\n@@ -12,5 +12,8 @@ func (c *Cache) Get(key string) (Entry, bool) {\n \tc.mu.RLock()\n \tdefer c.mu.RUnlock()\n \te, ok := c.items[key]\n-\treturn e, ok\n+\tif !ok || e.expired(c.now()) {\n+\t\treturn Entry{}, false\n+\t}\n+\treturn e, true\n }\n"
  },
  {
    "id": "diagram-flow",
    "type": "diagram",
    "kind": "mermaid",
    "title": "Write path after the cutover",
    "source": "graph LR\n  A[Client] --> B[Sync API]\n  B --> C[Batcher]\n  C --> D[(Ledger DB)]"
  },
  {
    "id": "image-dashboard",
    "type": "image",
    "src": "asset:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
    "alt": "Write latency dashboard during the staging replay",
    "caption": "Staging replay, p95 write latency"
  },
  {
    "id": "table-regions",
    "type": "table",
    "columns": [
      {"key": "region", "label": "Region"},
      {"key": "rows", "label": "Rows / s", "align": "right"},
      {"key": "status", "label": "Status"}
    ],
    "rows": [
      {"region": "us-east", "rows": "4,120", "status": "**live**"},
      {"region": "eu-west", "rows": "2,870", "status": "live"},
      {"region": "ap-south", "rows": "960", "status": "_pending_"}
    ]
  },
  {
    "id": "progress-backfill",
    "type": "progress",
    "label": "Backfilling account batches",
    "value": 7,
    "max": 12,
    "state": "active"
  },
  {
    "id": "chart-latency",
    "type": "chart",
    "kind": "bar",
    "title": "Write latency by day",
    "unit": "ms",
    "categories": ["Mon", "Tue", "Wed", "Thu"],
    "series": [
      {"label": "p50", "values": [42, 38, 35, 31]},
      {"label": "p95", "values": [180, 150, 122, 96]}
    ]
  },
  {
    "id": "term-tests",
    "type": "term",
    "title": "Unit tests",
    "command": "go test ./internal/...",
    "output": "\u001b[32mok\u001b[0m   example.com/ledger/internal/store   0.412s\n\u001b[32mok\u001b[0m   example.com/ledger/internal/sync    1.087s\n\u001b[31mFAIL\u001b[0m example.com/ledger/internal/export  0.233s\n\u001b[33m--- SKIP\u001b[0m: TestArchiveRestore \u001b[2m(requires network)\u001b[0m\n\u001b[1;31m1 package failed\u001b[0m"
  },
  {
    "id": "filetree-changes",
    "type": "filetree",
    "title": "Changed files",
    "entries": [
      {"path": "internal/sync/batch.go", "badge": "added", "note": "new batcher"},
      {"path": "internal/sync/worker.go", "badge": "modified"},
      {"path": "internal/sync/legacy_writer.go", "badge": "removed"},
      {"path": "internal/store/cache.go", "badge": "modified"},
      {"path": "internal/store/cache_test.go"},
      {"path": "docs/sync.md", "badge": "modified"}
    ]
  },
  {
    "id": "record-service",
    "type": "record",
    "title": "sync-worker",
    "chips": [{"label": "tier 1"}, {"label": "on call", "tone": "flag"}, {"label": "canary", "tone": "demo"}],
    "facts": [
      {"label": "Error rate", "value": "0.02%", "tone": "good"},
      {"label": "Queue depth", "value": "8,400 rows", "tone": "warn"},
      {"label": "Failed exports", "value": "3", "tone": "bad"},
      {"label": "Version", "value": "4.2.0-rc1"}
    ],
    "links": [
      {"label": "Runbook", "url": "https://example.com/runbooks/sync-worker"},
      {"label": "Dashboard", "url": "https://example.com/dashboards/sync"}
    ]
  },
  {
    "id": "pr-batching",
    "type": "pr",
    "repo": "example-org/ledger",
    "number": 412,
    "title": "sync: batch row writes per account",
    "summary": "Replaces the per-row commit loop with a batcher that flushes on size or time.",
    "files": ["internal/sync/*.go"],
    "head": "3f9c2ab"
  },
  {
    "id": "pr-groups-release",
    "type": "pr-groups",
    "repo": "example-org/ledger",
    "prompt": "Approve the 4.2 release train?",
    "groups": [
      {
        "id": "sync",
        "title": "Faster sync",
        "summary": "Large imports finish in minutes instead of timing out.",
        "prs": [
          {
            "number": 412,
            "head": "3f9c2ab",
            "title": "sync: batch row writes per account",
            "change": "Imports over 50k rows no longer time out.",
            "plan": "Matches the plan.",
            "risk": "Medium: changes the commit path for every write.",
            "review": "Approved by two reviewers; CI green.",
            "blocker": "None.",
            "md": "Flushes every 200 ms or at 500 rows. The old per-row path stays behind the flag for one release.",
            "files": ["internal/sync/*.go"],
            "stack": [{"number": 413, "head": "b71d0e4"}]
          },
          {
            "number": 418,
            "head": "c4e81f0",
            "title": "store: expire cache entries on read",
            "change": "Stale balances no longer show after a sync.",
            "plan": "Adds a read-time expiry the plan did not call for.",
            "risk": "Low: read path only.",
            "review": "One approval; CI green.",
            "blocker": "Waiting on a second review.",
            "carried": ["a0d93b7"]
          }
        ]
      },
      {
        "id": "exports",
        "title": "Shorter export history",
        "summary": "Exports keep 30 days of history instead of 90.",
        "prs": [
          {
            "number": 421,
            "head": "e2a5c19",
            "title": "export: drop the 90-day archive",
            "change": "Exports older than 30 days need a manual restore.",
            "plan": "Matches the plan.",
            "risk": "Low: the archive was read twice last year.",
            "review": "Changes requested; CI red on one flaky test.",
            "blocker": "Requested changes are unaddressed."
          }
        ]
      }
    ]
  },
  {
    "id": "pack-rating",
    "type": "example.rating",
    "label": "How clear is this rollout plan?",
    "scale": 5
  }
]
"""#

private let expectedTypes = [
    "section", "card", "approval", "choice", "draft", "triage", "input", "markdown", "code", "diff",
    "diagram", "image", "table", "progress", "chart", "term", "filetree", "record", "pr", "pr-groups",
    "example.rating",
]

private struct NoopPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

private func fixtureBlocks() throws -> [Block] {
    try JSONDecoder().decode([Block].self, from: Data(fixture.utf8))
}

@Suite("Block snapshots")
@MainActor
struct BlockSnapshotTests {
    enum Appearance: String, CaseIterable, Sendable {
        case light
        case dark

        var colorScheme: ColorScheme {
            switch self {
            case .light: .light
            case .dark: .dark
            }
        }
    }

    @Test("the snapshot fixture holds every built-in block type and a pack block, in order")
    func fixtureCoversEveryType() throws {
        let blocks = try fixtureBlocks()
        #expect(blocks.map(\.type) == expectedTypes)
    }

    @Test("every block renders to an image in light and dark", arguments: expectedTypes, Appearance.allCases)
    func blockRenders(type: String, appearance: Appearance) throws {
        let blocks = try fixtureBlocks()
        let index = try #require(blocks.firstIndex { $0.type == type })
        let renderer = ImageRenderer(
            content: BlockView(block: blocks[index], store: BoardStore(subject: "snapshots", transport: NoopPoster()))
                .padding(16)
                .frame(width: 390)
                .background(Palette.cardLift)
                .environment(\.colorScheme, appearance.colorScheme)
        )
        renderer.scale = 3
        let image = try #require(renderer.uiImage)
        #expect(image.size.height > 0)
        guard let dir = ProcessInfo.processInfo.environment["SNAPSHOT_DIR"] else { return }
        let png = try #require(image.pngData())
        try png.write(to: URL(filePath: dir).appending(path: "\(String(format: "%02d", index))-\(type)-\(appearance.rawValue).png"))
    }
}
