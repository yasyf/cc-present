@testable import CcPresentUI
import CcPresentKit
import Foundation
import IOSKit
import Testing

private func decode<T: Decodable>(_ type: T.Type, _ json: String) throws -> T {
    try JSONDecoder().decode(type, from: Data(json.utf8))
}

@Suite("PR block helpers")
@MainActor
struct PRBlockHelperTests {
    @Test func checksSummaryOmitsEmptyBuckets() throws {
        let green = try decode(PRLive.Checks.self, #"{"state":"success","total":4,"passed":4,"failed":0,"pending":0,"skipped":0}"#)
        let mixed = try decode(PRLive.Checks.self, #"{"state":"failure","total":6,"passed":2,"failed":1,"pending":2,"skipped":1}"#)
        #expect(prChecksSummary(green) == "4/4 passed")
        #expect(prChecksSummary(mixed) == "2/6 passed · 1 failed · 2 pending · 1 skipped")
    }

    @Test func filePatchHeadsThePatchWithTheOldAndNewPaths() throws {
        let file = try decode(
            PRLive.File.self,
            #"{"path":"b.ts","previousPath":"a.ts","status":"renamed","additions":1,"deletions":1,"patch":"@@ -1 +1 @@\n-x\n+y"}"#
        )
        #expect(prFilePatch(file) == "--- a/a.ts\n+++ b/b.ts\n@@ -1 +1 @@\n-x\n+y")
    }

    @Test func verdictsMapToTones() {
        #expect(["approved", "changes_requested", "review_required", "none"].map(prTone) == ["good", "bad", "warn", "dim"])
    }

    @Test func fetchedAgoAgesTheFetchInstant() throws {
        let now = try #require(parseTimestamp("2026-10-14T21:00:00Z"))
        let halfMinute = try #require(parseTimestamp("2026-10-14T20:59:30Z"))
        let quarterHour = try #require(parseTimestamp("2026-10-14T20:45:00Z"))
        let threeHours = try #require(parseTimestamp("2026-10-14T18:00:00Z"))
        #expect(prFetchedAgo(halfMinute, now: now) == "just now")
        #expect(prFetchedAgo(quarterHour, now: now) == "15m ago")
        #expect(prFetchedAgo(threeHours, now: now) == "3h ago")
    }

    @Test func fileStatusLettersFollowGitHubStatuses() {
        #expect(["added", "removed", "modified", "renamed", "copied", "changed", "unchanged", "odd"].map(prFileStatusLetter)
            == ["A", "D", "M", "R", "C", "M", "=", "odd"])
    }

    @Test func aDaemonErrorShowsItsBodyAndAnEmptyBodyNamesTheStatus() {
        #expect(prFailureText(APIError.status(code: 502, body: "gh: rate limited\n")) == "gh: rate limited")
        #expect(prFailureText(APIError.status(code: 404, body: "  ")) == "GitHub fetch failed (404)")
    }
}

@Suite("pr-groups block lines")
@MainActor
struct PRGroupsLineTests {
    @Test func tallyLineAlwaysNamesApprovalsAndOnlyNonzeroHoldsAndWaits() {
        #expect(prGroupsTallyLine(Block.PRGroups.Tally(approved: 0, held: 0, open: 3)) == "0 approved · 3 waiting on you")
        #expect(prGroupsTallyLine(Block.PRGroups.Tally(approved: 2, held: 1, open: 0)) == "2 approved · 1 held")
    }

    @Test func answeredLineSaysApprovalsLandAndNamesHolds() {
        #expect(prGroupsAnsweredLine(Block.PRGroups.Tally(approved: 2)) == "2 approved, landing")
        #expect(prGroupsAnsweredLine(Block.PRGroups.Tally(approved: 2, held: 1)) == "2 approved, landing · 1 held")
    }
}
