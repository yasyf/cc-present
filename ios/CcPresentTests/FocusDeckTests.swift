@testable import CcPresentApp
import CcPresentKit
import Foundation
import Testing

// The deck's index preservation and clamp are exercised over genuinely reduced doc
// state: block.removed frames go through the same public reducer BoardStore drives
// its state from (BoardStore.ingest is internal to the kit), then focusSteps + the
// FocusDeckModel re-anchor over the churned deck.

private func approval(_ id: String) -> Block {
    .approval(Block.Approval(id: id))
}

private func choice(_ id: String) -> Block {
    .choice(Block.Choice(id: id, options: [Block.Option(id: "\(id)o1", label: "one")]))
}

private func reduced(_ events: [Event]) throws -> BoardState {
    try reduce(events: events)
}

private func docReplaced(_ blocks: [Block], seq: Int64) throws -> Event {
    let docData = try JSONEncoder().encode(Doc(title: "deck", blocks: blocks))
    let docObject = try JSONSerialization.jsonObject(with: docData)
    let frame: [String: Any] = ["type": "doc.replaced", "doc": docObject, "revision": 0]
    return try Event.wireFrame(JSONSerialization.data(withJSONObject: frame), seq: seq)
}

private func blockRemoved(_ id: String, seq: Int64) throws -> Event {
    try Event.wireFrame(JSONSerialization.data(withJSONObject: ["type": "block.removed", "id": id]), seq: seq)
}

private func currentBlocks(_ state: BoardState) -> [Block] {
    state.doc.blocks.filter { state.rounds.blockRounds[$0.id] == state.rounds.current }
}

private struct ClampCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let anchorIndex: Int
    let removed: String
    let expectedAnchor: String

    var description: String {
        name
    }
}

private let fourApprovals: [Block] = [approval("a1"), approval("a2"), approval("a3"), approval("a4")]

private let clampCases: [ClampCase] = [
    ClampCase(
        name: "a surviving anchor keeps its place when an earlier block vanishes",
        blocks: fourApprovals,
        anchorIndex: 2,
        removed: "a1",
        expectedAnchor: "a3"
    ),
    ClampCase(
        name: "a surviving anchor keeps its place when a later block vanishes",
        blocks: fourApprovals,
        anchorIndex: 3,
        removed: "a2",
        expectedAnchor: "a4"
    ),
    ClampCase(
        name: "a vanished anchor clamps to the step now at its index",
        blocks: fourApprovals,
        anchorIndex: 2,
        removed: "a3",
        expectedAnchor: "a4"
    ),
    ClampCase(
        name: "a vanished tail anchor clamps to the new last step",
        blocks: fourApprovals,
        anchorIndex: 3,
        removed: "a4",
        expectedAnchor: "a3"
    ),
    ClampCase(
        name: "the last surviving anchor collapses to the summary sentinel",
        blocks: [approval("a1")],
        anchorIndex: 0,
        removed: "a1",
        expectedAnchor: deckEnd
    ),
]

@MainActor
@Test("the deck clamps a vanished anchor and preserves a surviving one", arguments: clampCases)
private func deckReanchorsOverChurn(_ testCase: ClampCase) throws {
    let before = try reduced([docReplaced(testCase.blocks, seq: 1)])
    let steps = focusSteps(currentBlocks(before), [])
    let model = FocusDeckModel(anchorId: steps[testCase.anchorIndex].id)
    model.reconcile(steps)

    let after = try reduced([docReplaced(testCase.blocks, seq: 1), blockRemoved(testCase.removed, seq: 2)])
    let churned = focusSteps(currentBlocks(after), [])
    model.reconcile(churned)

    #expect(model.anchorId == testCase.expectedAnchor, "case: \(testCase.name)")
}

@MainActor
@Test("next-undecided wraps across the deck")
private func nextUndecidedWraps() {
    let steps = focusSteps([approval("a1"), approval("a2"), approval("a3")], [])
    let model = FocusDeckModel(anchorId: steps[1].id)
    model.reconcile(steps)
    // a2 (current) and a1 decided; only a3 is undecided.
    let interactions = Interactions(decisions: [
        "a1": Decision(verdict: "approved"),
        "a2": Decision(verdict: "approved"),
    ])
    model.next(steps, interactions, [], [])
    #expect(model.anchorId == "a3")
    // From a3 with a3 still undecided, next wraps forward and stays on a3.
    model.next(steps, interactions, [], [])
    #expect(model.anchorId == "a3")
}

@MainActor
@Test("next lands on the summary when nothing is undecided")
private func nextLandsOnSummary() {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    let interactions = Interactions(decisions: [
        "a1": Decision(verdict: "approved"),
        "a2": Decision(verdict: "rejected"),
    ])
    model.next(steps, interactions, [], [])
    #expect(model.anchorId == deckEnd)
}

@MainActor
@Test("next skips a revising step on the momentum pass, preferring a settled one")
private func nextSkipsRevisingFirst() {
    let steps = focusSteps([approval("a1"), approval("a2"), approval("a3")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // a1 decided; a2 undecided but under a live rewrite; a3 undecided and settled.
    let interactions = Interactions(decisions: ["a1": Decision(verdict: "approved")])
    model.next(steps, interactions, [], ["a2"])
    #expect(model.anchorId == "a3")
}

@MainActor
@Test("next falls back to a revising step when it is the only undecided one")
private func nextFallsBackToRevising() {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // a1 decided; only a2 is undecided, and it is being revised — never locked out.
    let interactions = Interactions(decisions: ["a1": Decision(verdict: "approved")])
    model.next(steps, interactions, [], ["a2"])
    #expect(model.anchorId == "a2")
}

@MainActor
@Test("a round change resets the deck to step 0")
private func roundChangeResets() {
    let steps = focusSteps([approval("a1"), approval("a2"), approval("a3")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    model.move(steps, 2)
    #expect(model.anchorId == "a3")
    let next = focusSteps([approval("b1"), approval("b2")], [])
    model.reset(next)
    #expect(model.anchorId == "b1")
}

// The auto-advance schedule/cancel decision is driven the way production does: the
// deck feeds successive AdvanceKey transitions to reconcileAdvance, so these exercise
// the real guard rather than poking the private timer.

private func armed(_ stepId: String, _ signature: String = "approved") -> AdvanceKey {
    AdvanceKey(stepId: stepId, signature: signature)
}

private func undecidedKey(_ stepId: String) -> AdvanceKey {
    AdvanceKey(stepId: stepId, signature: "")
}

@MainActor
@Test("an optimistic verdict then its SSE echo still auto-advances")
private func optimisticThenEchoAdvances() async {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // The optimistic patch flips the current step undecided→decided and arms the timer.
    model.reconcileAdvance(from: undecidedKey("a1"), to: armed("a1"))
    // The SSE echo re-renders the same key; the armed timer must survive it.
    model.reconcileAdvance(from: armed("a1"), to: armed("a1"))
    #expect(model.anchorId == "a1")
    await model.advance?.value
    #expect(model.anchorId == "a2")
}

@MainActor
@Test("a rolled-back verdict cancels the pending auto-advance")
private func rollbackCancelsAdvance() async {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    model.reconcileAdvance(from: undecidedKey("a1"), to: armed("a1"))
    let advance = model.advance
    // The optimistic decision fails and reverts to undecided before the timer fires.
    model.reconcileAdvance(from: armed("a1"), to: undecidedKey("a1"))
    await advance?.value
    #expect(model.anchorId == "a1")
}

@MainActor
@Test("an open feedback composer suppresses the auto-advance at fire time")
private func composerSuppressesAdvance() async {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    model.composer.set("a1", composing: true)
    model.reconcileAdvance(from: undecidedKey("a1"), to: armed("a1"))
    await model.advance?.value
    #expect(model.anchorId == "a1")
}

@MainActor
@Test("re-deciding an already-decided step never schedules an advance")
private func alreadyDecidedNeverAdvances() async {
    let steps = focusSteps([approval("a1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // Swiping an already-decided approval yields an unchanged key, so no advance is
    // scheduled and the anchor stays put — the finding-3 case the SwipeableFocusCard
    // pairs with restoring its own visibility.
    model.reconcileAdvance(from: armed("a1"), to: armed("a1"))
    try? await Task.sleep(for: .milliseconds(650))
    #expect(model.anchorId == "a1")
}

@MainActor
@Test("a single-select choice pick arms the advance and lands on the next step")
private func choicePickAdvances() async {
    let steps = focusSteps([choice("c1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // A pick signs the step non-empty and arms; a following note re-renders the same
    // signature (feedback never arms), so the armed timer survives.
    model.reconcileAdvance(from: undecidedKey("c1"), to: armed("c1", "c1o1"))
    model.reconcileAdvance(from: armed("c1", "c1o1"), to: armed("c1", "c1o1"))
    #expect(model.anchorId == "c1")
    await model.advance?.value
    #expect(model.anchorId == "a2")
}

@MainActor
@Test("re-picking a single-select choice re-arms toward the next step")
private func choiceRepickReArms() async {
    let steps = focusSteps([choice("c1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    model.reconcileAdvance(from: undecidedKey("c1"), to: armed("c1", "c1o1"))
    // A re-pick changes the signature on the same step, re-arming a fresh timer.
    model.reconcileAdvance(from: armed("c1", "c1o1"), to: armed("c1", "c1o2"))
    await model.advance?.value
    #expect(model.anchorId == "a2")
}

@MainActor
@Test("committing a write-in arms the auto-advance")
private func choiceOtherArms() async {
    let steps = focusSteps([choice("c1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // The write-in signs the step by its text — an empty option set still arms.
    model.reconcileAdvance(from: undecidedKey("c1"), to: armed("c1", "custom"))
    await model.advance?.value
    #expect(model.anchorId == "a2")
}

@MainActor
@Test("clearing a pick signs the step empty, so the deck cancels instead of advancing")
private func clearingPickCancelsAdvance() async {
    let steps = focusSteps([choice("c1"), approval("a2")], [])
    let model = FocusDeckModel(anchorId: steps[0].id)
    model.reconcile(steps)
    // An existing-but-empty selection ({optionIds: [], other: nil}) is undecided, so
    // advanceSignature signs empty — a cleared pick must not auto-advance.
    let cleared = Interactions(choices: ["c1": Selection(optionIds: [], other: nil)])
    #expect(advanceSignature(steps[0], cleared) == "")
    // A decided step cleared back to empty cancels the armed timer, holding the step.
    model.reconcileAdvance(from: armed("c1", "c1o1"), to: undecidedKey("c1"))
    try? await Task.sleep(for: .milliseconds(650))
    #expect(model.anchorId == "c1")
}
