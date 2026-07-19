@testable import CcPresentApp
import CcPresentKit
import Testing

@Suite("Focus revision dot priority")
struct FocusRevisionDotTests {
    @Test("revising outranks every change kind")
    func revisingWins() {
        #expect(revisionDotState(isRevising: true, changeKind: .added) == .revising)
        #expect(revisionDotState(isRevising: true, changeKind: .revised) == .revising)
        #expect(revisionDotState(isRevising: true, changeKind: nil) == .revising)
    }

    @Test("an added step outranks a changed one, and a changed step shows a ring")
    func addedAndChanged() {
        #expect(revisionDotState(isRevising: false, changeKind: .added) == .added)
        #expect(revisionDotState(isRevising: false, changeKind: .revised) == .changed)
    }

    @Test("a settled step carries no overlay")
    func none() {
        #expect(revisionDotState(isRevising: false, changeKind: nil) == .none)
    }
}

@Suite("Focus revision copy")
struct FocusRevisionCopyTests {
    @Test("the revising banner appends the note when present, omits it gracefully otherwise")
    func bannerText() {
        #expect(revisingBannerText(note: "reworking per your pick") == "Claude is rewriting this step — reworking per your pick")
        #expect(revisingBannerText(note: nil) == "Claude is rewriting this step")
        #expect(revisingBannerText(note: "") == "Claude is rewriting this step")
    }

    @Test("the callout distinguishes an added from a revised step, appending the note")
    func calloutText() {
        let revised = RevisionState.Mark(kind: .revised, note: "based on step 1", at: .init())
        let added = RevisionState.Mark(kind: .added, note: "compare option C", at: .init())
        #expect(revisionCalloutText(revised) == "Updated after your earlier pick — based on step 1")
        #expect(revisionCalloutText(added) == "Claude added this step — compare option C")
    }

    @Test("the callout omits an absent note gracefully")
    func calloutNoteOptional() {
        #expect(revisionCalloutText(RevisionState.Mark(kind: .revised, note: nil, at: .init())) == "Updated after your earlier pick")
        #expect(revisionCalloutText(RevisionState.Mark(kind: .added, note: nil, at: .init())) == "Claude added this step")
    }
}
