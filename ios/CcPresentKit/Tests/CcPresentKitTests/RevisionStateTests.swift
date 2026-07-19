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

    @Test("an orphaned revising set decays passive after its window")
    func decayFlips() async {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "n")), isLoading: false)
        #expect(store.revisingSince != nil)
        #expect(!store.revisingDecayed)

        // Await the flip's own task, not a wall clock a starved main actor can outlast.
        await store.decayTask?.value
        #expect(store.revisingDecayed)
    }

    @Test("resolving the revising set stops the decay clock")
    func resolvingClearsDecay() async throws {
        let store = RevisionState(decayInterval: 0.05)
        store.ingest(prev: state([]), next: state([], revising: Revising(blockIds: ["x"], note: "n")), isLoading: false)
        store.ingest(prev: state([]), next: state([], revising: Revising()), isLoading: false)
        #expect(store.revisingSince == nil)

        try await Task.sleep(for: .seconds(0.15))
        #expect(!store.revisingDecayed)
    }
}
