@testable import CcPresentKit
import Foundation
import Testing

private func md(_ id: String, _ text: String) -> Block {
    .markdown(Block.Markdown(id: id, md: text))
}

private func state(_ blocks: [Block], revising: Revising = .init()) -> BoardState {
    BoardState(
        doc: Doc(version: 1, title: "", blocks: blocks),
        interactions: Interactions(),
        rounds: Rounds(),
        revising: revising
    )
}

@MainActor
@Suite("RevisionState seen store")
struct RevisionStateTests {
    @Test("replay frames never mark — only live frames past the boundary badge")
    func liveGate() {
        let store = RevisionState()
        let prev = state([md("x", "old")])
        let next = state([md("x", "new")])

        store.ingest(prev: prev, next: next, isLoading: true)
        #expect(store.mark(for: "x") == nil)

        store.ingest(prev: prev, next: next, isLoading: false)
        #expect(store.mark(for: "x")?.kind == .revised)
    }

    @Test("a same-id rewrite lifts the note from the pre-clear revising state")
    func noteLiftOnRevise() {
        let store = RevisionState()
        // The reducer clears revising[x] on the completing upsert, so `next` no longer
        // carries the note; it must be lifted from `prev`.
        let prev = state([md("x", "old")], revising: Revising(blockIds: ["x"], note: "reworking per your pick"))
        let next = state([md("x", "new")], revising: Revising())

        store.ingest(prev: prev, next: next, isLoading: false)

        let mark = store.mark(for: "x")
        #expect(mark?.kind == .revised)
        #expect(mark?.note == "reworking per your pick")
    }

    @Test("a fresh id is added and takes the doc-level drafting note")
    func addedTakesDocLevelNote() {
        let store = RevisionState()
        let prev = state([md("x", "keep")], revising: Revising(blockIds: [], note: "drafting a comparison step"))
        let next = state([md("x", "keep"), md("y", "new step")], revising: Revising())

        store.ingest(prev: prev, next: next, isLoading: false)

        let mark = store.mark(for: "y")
        #expect(mark?.kind == .added)
        #expect(mark?.note == "drafting a comparison step")
        #expect(store.mark(for: "x") == nil)
    }

    @Test("an unannounced rewrite marks revised with no note")
    func revisedWithoutAnnouncement() {
        let store = RevisionState()
        let prev = state([md("x", "old")])
        let next = state([md("x", "new")])

        store.ingest(prev: prev, next: next, isLoading: false)

        #expect(store.mark(for: "x")?.kind == .revised)
        #expect(store.mark(for: "x")?.note == nil)
    }

    @Test("an identical re-upsert marks nothing")
    func noopUpsertDoesNotMark() {
        let store = RevisionState()
        let same = state([md("x", "same")])

        store.ingest(prev: same, next: same, isLoading: false)

        #expect(store.mark(for: "x") == nil)
    }

    @Test("viewing a step clears its mark")
    func markSeenClears() {
        let store = RevisionState()
        store.ingest(prev: state([md("x", "old")]), next: state([md("x", "new")]), isLoading: false)
        #expect(store.mark(for: "x") != nil)

        store.markSeen("x")
        #expect(store.mark(for: "x") == nil)
    }

    @Test("removing a block drops its mark")
    func removedDropsMark() {
        let store = RevisionState()
        store.ingest(prev: state([md("x", "old")]), next: state([md("x", "new")]), isLoading: false)
        #expect(store.mark(for: "x") != nil)

        store.ingest(prev: state([md("x", "new")]), next: state([]), isLoading: false)
        #expect(store.mark(for: "x") == nil)
    }

    @Test("the revising set and doc-level drafting note track the latest frame")
    func revisingMirror() {
        let store = RevisionState()
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["a", "b"], note: "n")), isLoading: false)
        #expect(store.isRevising("a"))
        #expect(store.revisingNote(for: "a") == "n")
        #expect(store.docDraftingNote == nil)

        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: [], note: "drafting")), isLoading: false)
        #expect(!store.isRevising("a"))
        #expect(store.docDraftingNote == "drafting")
    }

    @Test("a live doc.replaced clears every mark and seeds none from the new doc")
    func docReplacedClearsMarks() {
        let store = RevisionState()
        store.ingest(prev: state([md("x", "old")]), next: state([md("x", "new")]), isLoading: false)
        #expect(store.mark(for: "x") != nil)

        // A wholesale re-push past the boundary is a fresh slate: clear marks, and do
        // not re-badge the swapped-in doc (neither the changed x nor the fresh y).
        store.ingest(
            prev: state([md("x", "new")]),
            next: state([md("x", "fresh"), md("y", "brand new")]),
            isLoading: false,
            docReplaced: true
        )
        #expect(store.mark(for: "x") == nil)
        #expect(store.mark(for: "y") == nil)
    }

    @Test("each revising id opens its own decay window and goes passive independently")
    func perIdDecay() async {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x", "y"], note: "n")), isLoading: false)
        #expect(store.decayTasks["x"] != nil)
        #expect(store.decayTasks["y"] != nil)
        #expect(!store.revisingPassive("x"))

        // Await each id's own window, not a shared clock.
        await store.decayTasks["x"]?.value
        await store.decayTasks["y"]?.value
        #expect(store.revisingPassive("x"))
        #expect(store.revisingPassive("y"))
    }

    @Test("a note-only re-announcement does not restart an id's decay window")
    func noteOnlyKeepsWindow() async {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "first")), isLoading: false)
        let window = store.decayTasks["x"]

        // Same id, new note, before decay: the window is the very same task — not reset.
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "second")), isLoading: false)
        #expect(store.decayTasks["x"] == window)
        #expect(store.revisingNote(for: "x") == "second")
        #expect(!store.revisingPassive("x"))

        await store.decayTasks["x"]?.value
        #expect(store.revisingPassive("x"))
    }

    @Test("re-announcing a decayed id resets its window and clears its passive flag")
    func reAnnounceResetsAfterDecay() async {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "first")), isLoading: false)
        await store.decayTasks["x"]?.value
        #expect(store.revisingPassive("x"))

        // A fresh announcement un-sticks the decayed id and restarts its clock.
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "second")), isLoading: false)
        #expect(!store.revisingPassive("x"))
        #expect(store.decayTasks["x"] != nil)

        await store.decayTasks["x"]?.value
        #expect(store.revisingPassive("x"))
    }

    @Test("resolving the revising set cancels every decay window")
    func resolvingCancelsDecay() async throws {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "n")), isLoading: false)
        #expect(store.decayTasks["x"] != nil)

        store.ingest(prev: state([]), next: state([], revising: Revising()), isLoading: false)
        #expect(store.decayTasks["x"] == nil)

        try await Task.sleep(for: .seconds(0.1))
        #expect(!store.revisingPassive("x"))
    }
}
