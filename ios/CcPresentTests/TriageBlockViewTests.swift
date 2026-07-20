@testable import CcPresentApp
import CcPresentKit
import Testing

@Suite("Triage verdict helpers")
struct TriageVerdictHelperTests {
    @Test func flipToANewVerdictCarriesTheNoteForward() {
        #expect(triageFlip(current: "approved", note: "keep me", target: "rejected")
            == TriageVerdict(verdict: "rejected", note: "keep me"))
    }

    @Test func rePressingTheActiveVerdictClearsAndDropsTheNote() {
        #expect(triageFlip(current: "approved", note: "gone", target: "approved")
            == TriageVerdict(verdict: "cleared"))
    }

    @Test func settingAVerdictOnAnUndecidedItemHasNoNote() {
        #expect(triageFlip(current: nil, note: nil, target: "approved")
            == TriageVerdict(verdict: "approved"))
    }

    @Test func bulkSetsEveryItemAndCarriesExistingNotesForward() {
        let items = [Block.Item(id: "i1", label: "One"), Block.Item(id: "i2", label: "Two")]
        let verdicts = ["i1": Decision(verdict: "rejected", note: "flaky")]

        let post = triageBulk(items: items, verdicts: verdicts, target: "approved")

        #expect(post == [
            "i1": TriageVerdict(verdict: "approved", note: "flaky"),
            "i2": TriageVerdict(verdict: "approved"),
        ])
    }
}
