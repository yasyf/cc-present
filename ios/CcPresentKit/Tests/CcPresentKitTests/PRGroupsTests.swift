import CcPresentKit
import Foundation
import Testing

private typealias PRGroups = Block.PRGroups

private func row(_ number: Int, _ head: String, carried: [String]? = nil) -> PRGroups.Row {
    PRGroups.Row(
        number: number,
        head: head,
        title: "PR \(number)",
        change: "C.",
        plan: "Matches the plan.",
        risk: "R.",
        review: "V.",
        blocker: "Nothing.",
        carried: carried
    )
}

private let first = PRGroups.Group(id: "g1", title: "G1", summary: "S.", prs: [row(8, "abc1234"), row(9, "def5678")])
private let second = PRGroups.Group(id: "g2", title: "G2", summary: "S.", prs: [row(10, "1112223", carried: ["0009998"])])

@Suite("pr-groups derivations")
struct PRGroupsDerivationTests {
    @Test func sameHeadMatchesEitherPrefix() {
        #expect(PRGroups.sameHead("abc1234", "abc1234def"))
        #expect(PRGroups.sameHead("abc1234def", "abc1234"))
        #expect(!PRGroups.sameHead("abc1234", "abc1235"))
        #expect(!PRGroups.sameHead("", "abc1234"))
    }

    @Test func rowVerdictIsUndecidedWithoutADecision() {
        #expect(PRGroups.rowVerdict(first.prs[0], nil) == .undecided)
    }

    @Test func rowVerdictStandsAtTheRowHeadMatchedByPrefix() {
        let decision = Decision(verdict: "approved", head: "abc1234ffff")
        #expect(PRGroups.rowVerdict(first.prs[0], decision) == .current(decision, carried: false))
    }

    @Test func rowVerdictGoesStaleWithoutARecordedHead() {
        let decision = Decision(verdict: "approved")
        #expect(PRGroups.rowVerdict(first.prs[0], decision) == .stale(decision))
    }

    @Test func rowVerdictGoesStaleAtAnotherHead() {
        let decision = Decision(verdict: "approved", head: "9999999")
        #expect(PRGroups.rowVerdict(first.prs[0], decision) == .stale(decision))
    }

    @Test func rowVerdictStandsAtACarriedHeadAndSaysSo() {
        let decision = Decision(verdict: "approved", head: "0009998")
        #expect(PRGroups.rowVerdict(second.prs[0], decision) == .current(decision, carried: true))
    }

    @Test func staleLineNamesTheOldHeadTheTimeAndTheNewHead() {
        let line = PRGroups.staleLine(first.prs[0], Decision(verdict: "approved", head: "9999999aaaa", at: "2026-10-26T21:03:00Z"))
        #expect(line.hasPrefix("Approved at 9999999 on "))
        #expect(line.hasSuffix("; new head abc1234 needs your tap."))
        #expect(!line.contains("an earlier time"))
    }

    @Test func staleLineNamesTheHeadTheHumanTappedForACarriedVerdict() {
        let decision = Decision(
            verdict: "approved",
            head: "8888888",
            at: "2026-10-26T21:03:00Z",
            carriedFrom: "9999999",
            carryNote: "replay"
        )
        let line = PRGroups.staleLine(first.prs[0], decision)
        #expect(line.hasPrefix("Approved at 9999999 on "))
        #expect(line.hasSuffix("; new head abc1234 needs your tap."))
    }

    @Test func staleLineWithoutAStampSaysAnEarlierTime() {
        #expect(PRGroups.staleLine(first.prs[0], Decision(verdict: "rejected", head: "9999999"))
            == "Held at 9999999 on an earlier time; new head abc1234 needs your tap.")
    }

    @Test func givenLineNamesTheHeadAStandingVerdictWasGivenAt() {
        let line = PRGroups.givenLine(
            first.prs[0],
            Decision(verdict: "approved", head: "abc1234", at: "2026-10-26T21:03:00Z"),
            carried: false
        )
        #expect(line.hasPrefix("Approved at abc1234 on "))
        #expect(!line.contains("carried"))
    }

    @Test func givenLineNamesTheTappedHeadAndTheReasonForAnAgentCarry() {
        let decision = Decision(
            verdict: "approved",
            head: "abc1234",
            at: "2026-10-26T21:03:00Z",
            carriedFrom: "9999999aaaa",
            carryNote: "patch-identical replay"
        )
        guard case let .current(standing, carried) = PRGroups.rowVerdict(first.prs[0], decision) else {
            Issue.record("a carry at the row head should stand")
            return
        }
        let line = PRGroups.givenLine(first.prs[0], standing, carried: carried)
        #expect(line.hasPrefix("Your tap at 9999999 on "))
        #expect(line.hasSuffix(" carries to abc1234: patch-identical replay"))
    }

    @Test func givenLineSaysTheAgentCarriedAVerdictToTheRowHead() {
        #expect(PRGroups.givenLine(second.prs[0], Decision(verdict: "rejected", head: "0009998"), carried: true)
            == "Held at 0009998; the agent carried it to 1112223")
    }

    @Test func toggleEntryRecordsTheTargetAtTheRowHeadKeepingTheNote() {
        #expect(PRGroups.toggleEntry(first.prs[0], Decision(verdict: "rejected", note: "wait", head: "abc1234"), target: "approved")
            == TriageVerdict(verdict: "approved", note: "wait", head: "abc1234"))
    }

    @Test func toggleEntryClearsTheSameStandingVerdict() {
        #expect(PRGroups.toggleEntry(first.prs[0], Decision(verdict: "approved", head: "abc1234"), target: "approved")
            == TriageVerdict(verdict: "cleared"))
    }

    @Test func toggleEntryReRecordsAStaleApprovalAtTheNewHeadInsteadOfClearingIt() {
        #expect(PRGroups.toggleEntry(first.prs[0], Decision(verdict: "approved", head: "9999999"), target: "approved")
            == TriageVerdict(verdict: "approved", head: "abc1234"))
    }

    @Test func approveGroupFansOutToEveryRowNotAlreadyApprovedAtItsHead() {
        let merge = PRGroups.approveGroup(first, [
            "8": Decision(verdict: "approved", head: "abc1234"),
            "9": Decision(verdict: "rejected", note: "flaky", head: "def5678"),
        ])
        #expect(merge == ["9": TriageVerdict(verdict: "approved", note: "flaky", head: "def5678")])
    }

    @Test func tallyCountsAStaleVerdictAsWaiting() {
        let verdicts = [
            "8": Decision(verdict: "approved", head: "abc1234"),
            "9": Decision(verdict: "rejected", head: "def5678"),
            "10": Decision(verdict: "approved", head: "7777777"),
        ]
        #expect(PRGroups.tally(first.prs + second.prs, verdicts) == PRGroups.Tally(approved: 1, held: 1, open: 1))
    }

    @Test func shortHeadKeepsSevenCharacters() {
        #expect(PRGroups.shortHead("abc1234def5678") == "abc1234")
        #expect(PRGroups.shortHead("abc") == "abc")
    }
}

@Suite("pr-groups decoding")
struct PRGroupsDecodingTests {
    @Test func decodesEveryRowFieldAndKeepsUnmodeledFieldsOnReencode() throws {
        let json = #"""
        {
          "id": "pg",
          "type": "pr-groups",
          "repo": "yasyf/cc-present",
          "prompt": "Which of these land tonight?",
          "slack": "1730000000.000100",
          "groups": [
            {
              "id": "boards",
              "title": "Board rendering",
              "summary": "Boards draw **natively** on iOS.",
              "prs": [
                {
                  "number": 148,
                  "head": "4256e16aa0c1",
                  "title": "A tap for a redrawn head is recorded",
                  "change": "A verdict at an old head is kept.",
                  "plan": "Matches the plan.",
                  "risk": "Low.",
                  "review": "CI green, one approval.",
                  "blocker": "Nothing.",
                  "md": "## Detail\nThe daemon records the tap.",
                  "carried": ["0009998"],
                  "files": ["internal/daemon/*.go"],
                  "stack": [{"number": 149, "head": "1bdb065"}]
                },
                {
                  "number": 150,
                  "head": "282e2b2",
                  "title": "Guard refuses a foreign host",
                  "change": "C.",
                  "plan": "P.",
                  "risk": "R.",
                  "review": "V.",
                  "blocker": "B."
                }
              ]
            }
          ]
        }
        """#

        let decoded = try JSONDecoder().decode(Block.self, from: Data(json.utf8))
        guard case let .prGroups(block) = decoded else {
            Issue.record("pr-groups decoded as \(decoded.type)")
            return
        }

        #expect(block.repo == "yasyf/cc-present")
        #expect(block.prompt == "Which of these land tonight?")
        #expect(block.groups.map(\.id) == ["boards"])
        #expect(block.groups[0].summary == "Boards draw **natively** on iOS.")
        #expect(block.rows.map(\.id) == ["148", "150"])
        #expect(block.rows[0] == PRGroups.Row(
            number: 148,
            head: "4256e16aa0c1",
            title: "A tap for a redrawn head is recorded",
            change: "A verdict at an old head is kept.",
            plan: "Matches the plan.",
            risk: "Low.",
            review: "CI green, one approval.",
            blocker: "Nothing.",
            md: "## Detail\nThe daemon records the tap.",
            carried: ["0009998"],
            files: ["internal/daemon/*.go"],
            stack: [PRGroups.StackMember(number: 149, head: "1bdb065")]
        ))
        #expect(block.rows[1].md == nil)
        #expect(block.rows[1].stack == nil)

        let reencoded = try JSONDecoder().decode(JSONValue.self, from: JSONEncoder().encode(decoded))
        guard case let .object(fields) = reencoded else {
            Issue.record("pr-groups re-encoded as \(reencoded)")
            return
        }
        #expect(fields["slack"] == .string("1730000000.000100"))
        #expect(fields["type"] == .string("pr-groups"))
    }

    @Test func anAgentCarryReachesTheRowDecision() throws {
        let json = #"""
        [
          {
            "origin": "agent",
            "type": "triage.decided",
            "seq": 1,
            "payload": {
              "schemaVersion": 1,
              "type": "triage.decided",
              "blockId": "pg",
              "verdicts": {
                "148": {
                  "verdict": "approved",
                  "head": "4256e16",
                  "at": "2026-10-26T21:03:00Z",
                  "carriedFrom": "0001111",
                  "carryNote": "patch-identical replay"
                }
              }
            }
          }
        ]
        """#

        let state = try reduce(events: JSONDecoder().decode([Event].self, from: Data(json.utf8)))

        #expect(state.interactions.triage["pg"]?["148"] == Decision(
            verdict: "approved",
            head: "4256e16",
            at: "2026-10-26T21:03:00Z",
            carriedFrom: "0001111",
            carryNote: "patch-identical replay"
        ))
    }
}
