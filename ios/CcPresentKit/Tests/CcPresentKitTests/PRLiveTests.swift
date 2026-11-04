import CcPresentKit
import Foundation
import IOSKit
import Testing

@Suite("PR live read")
struct PRLiveTests {
    @Test func decodesTheDaemonsPullRequestRead() throws {
        let json = #"""
        {
          "repo": "yasyf/cc-present",
          "number": 148,
          "title": "fix: a pr-groups tap for a head since redrawn is recorded",
          "url": "https://github.com/yasyf/cc-present/pull/148",
          "author": "yasyf",
          "state": "merged",
          "base": "main",
          "head": "pr-groups-redrawn",
          "headSha": "4256e16aa0c1b2c3d4e5f60718293a4b5c6d7e8f",
          "mergedSha": "1bdb065ffeeddccbbaa99887766554433221100a",
          "additions": 120,
          "deletions": 14,
          "changedFiles": 3,
          "checks": {"state": "failure", "total": 6, "passed": 2, "failed": 1, "pending": 2, "skipped": 1, "failing": ["ios"]},
          "reviewDecision": "approved",
          "reviews": [{"author": "octocat", "state": "approved"}],
          "labels": ["bug"],
          "files": [
            {"path": "internal/daemon/rest.go", "status": "modified", "additions": 100, "deletions": 10, "patch": "@@ -1 +1 @@\n-x\n+y"},
            {"path": "web/src/b.ts", "previousPath": "web/src/a.ts", "status": "renamed", "additions": 20, "deletions": 4},
            {"path": "logo.png", "status": "added", "additions": 0, "deletions": 0}
          ],
          "fetchedAt": "2026-11-03T19:11:11.906262929Z"
        }
        """#

        let pr = try JSONDecoder().decode(PRLive.self, from: Data(json.utf8))

        #expect(pr.state == .merged)
        #expect(pr.url.absoluteString == "https://github.com/yasyf/cc-present/pull/148")
        #expect(pr.mergedSha == "1bdb065ffeeddccbbaa99887766554433221100a")
        #expect(pr.checks.failing == ["ios"])
        #expect(pr.reviews.map(\.author) == ["octocat"])
        #expect(pr.files.map(\.previousPath) == [nil, "web/src/a.ts", nil])
        #expect(pr.files.map(\.patch) == ["@@ -1 +1 @@\n-x\n+y", nil, nil])
        #expect(parseTimestamp(pr.fetchedAt) != nil)
    }

    @Test func anOpenPullRequestOmitsItsMergeCommitAndFailingChecks() throws {
        let json = #"""
        {
          "repo": "a/b", "number": 8, "title": "T", "url": "https://github.com/a/b/pull/8", "author": "x",
          "state": "draft", "base": "main", "head": "feature", "headSha": "abc1234",
          "additions": 0, "deletions": 0, "changedFiles": 0,
          "checks": {"state": "none", "total": 0, "passed": 0, "failed": 0, "pending": 0, "skipped": 0},
          "reviewDecision": "none", "reviews": [], "labels": [], "files": [],
          "fetchedAt": "2026-11-03T19:11:11Z"
        }
        """#

        let pr = try JSONDecoder().decode(PRLive.self, from: Data(json.utf8))

        #expect(pr.state == .draft)
        #expect(pr.mergedSha == nil)
        #expect(pr.checks.failing == nil)
        #expect(parseTimestamp(pr.fetchedAt) != nil)
    }
}
