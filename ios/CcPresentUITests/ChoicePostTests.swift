@testable import CcPresentUI
import Testing

@Test("a single-select tap replaces the pick and clears any write-in")
private func singleSelectTogglePost() {
    #expect(choiceTogglePost(multi: false, selectedIds: [], otherText: nil, optionId: "o1") == ChoicePost(optionIds: ["o1"], other: nil))
    // Re-tapping the sole pick clears it.
    #expect(choiceTogglePost(multi: false, selectedIds: ["o1"], otherText: nil, optionId: "o1") == ChoicePost(optionIds: [], other: nil))
    // A prior write-in is dropped when an authored option is picked.
    #expect(choiceTogglePost(multi: false, selectedIds: [], otherText: "custom", optionId: "o1") == ChoicePost(optionIds: ["o1"], other: nil))
}

@Test("a multi-select tap adds or removes and preserves the write-in")
private func multiSelectTogglePost() {
    #expect(choiceTogglePost(multi: true, selectedIds: ["o1"], otherText: "custom", optionId: "o2") == ChoicePost(optionIds: ["o1", "o2"], other: "custom"))
    #expect(choiceTogglePost(multi: true, selectedIds: ["o1", "o2"], otherText: "custom", optionId: "o1") == ChoicePost(optionIds: ["o2"], other: "custom"))
}

@Test("committing a write-in replaces single-select picks and coexists on multi")
private func writeInPost() {
    // Single-select: the write-in replaces the authored pick (last-write-wins).
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: nil, draft: "  custom ") == ChoicePost(optionIds: [], other: "custom"))
    // Multi-select: the write-in coexists with the picks.
    #expect(choiceOtherPost(multi: true, selectedIds: ["o1"], otherText: nil, draft: "custom") == ChoicePost(optionIds: ["o1"], other: "custom"))
    // An emptied field clears a prior write-in, keeping the picks.
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: "old", draft: "   ") == ChoicePost(optionIds: ["o1"], other: nil))
    // An emptied field with no prior write-in is a no-op.
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: "") == nil)
    // A zero-width-only draft reads as visually empty (mirroring the daemon), so it never
    // posts the invisible text: a no-op with no prior write-in, a clear with one.
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: "\u{200b}\u{200b}") == nil)
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: "old", draft: "\u{200b}") == ChoicePost(optionIds: ["o1"], other: nil))
    // An over-cap draft never posts, so no optimistic apply races the daemon's reject;
    // a draft exactly at the 64 KiB cap is accepted.
    let atCap = String(repeating: "a", count: 64 << 10)
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: atCap) == ChoicePost(optionIds: [], other: atCap))
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: atCap + "a") == nil)
}
