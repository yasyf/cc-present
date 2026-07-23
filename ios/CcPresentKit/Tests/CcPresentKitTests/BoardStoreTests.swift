@testable import CcPresentKit
import Foundation
import Testing

/// ScriptedTransport is a fake InteractionPoster: it hands back preset seqs in
/// order and records every interaction it was asked to post. No network.
private actor ScriptedTransport: InteractionPoster {
    private var seqs: [Int64]
    private(set) var received: [Interaction] = []

    init(seqs: [Int64]) {
        self.seqs = seqs
    }

    func postInteraction(subject _: String, interaction: Interaction) async throws -> Int64 {
        received.append(interaction)
        return seqs.removeFirst()
    }
}

/// frame builds an SSE echo message: a self-describing wire frame stamped with seq.
private func frame(type: String, seq: Int64, _ fields: [String: Any] = [:]) throws -> SSEClient.Message {
    var object = fields
    object["schemaVersion"] = 1
    object["type"] = type
    let data = try JSONSerialization.data(withJSONObject: object)
    return try .frame(Event.wireFrame(data, seq: seq))
}

@MainActor
@Suite("BoardStore optimistic engine")
struct BoardStoreTests {
    @Test("an optimistic decision is visible before the POST resolves")
    func optimisticApplyVisibleImmediately() async {
        let transport = ScriptedTransport(seqs: [1])
        let store = BoardStore(subject: "s", transport: transport)

        let task = store.send(.decision(blockId: "b1", verdict: .approved))
        // Applied synchronously, before the POST task has even run.
        #expect(store.state.interactions.decisions["b1"]?.verdict == "approved")
        #expect(store.pendingCount == 1)

        await task.value
        #expect(store.pendingCount == 1) // tagged, still awaiting its echo
        let received = await transport.received
        #expect(received == [.decision(blockId: "b1", verdict: .approved)])
    }

    @Test("the echo drops the pending item when the POST reply lands first (response then frame)")
    func reconcileResponseThenFrame() async throws {
        let transport = ScriptedTransport(seqs: [5])
        let store = BoardStore(subject: "s", transport: transport)

        let task = store.send(.choice(blockId: "c1", optionIds: ["o1", "o2"]))
        await task.value // POST resolves, pending tagged with seq 5
        #expect(store.pendingCount == 1)

        try store.ingest(frame(type: "choice.selected", seq: 5, ["blockId": "c1", "optionIds": ["o1", "o2"]]))
        #expect(store.pendingCount == 0) // seq match dropped the overlay
        #expect(store.state.interactions.choices["c1"]?.optionIds == ["o1", "o2"])
    }

    @Test("a write-in choice posts `other` and drops on its echo")
    func writeInChoiceRoundTrips() async throws {
        let transport = ScriptedTransport(seqs: [9])
        let store = BoardStore(subject: "s", transport: transport)

        let task = store.choose(blockId: "c1", optionIds: [], other: "hand-rolled")
        #expect(store.state.interactions.choices["c1"]?.other == "hand-rolled")
        #expect(store.state.interactions.choices["c1"]?.optionIds == [])

        await task.value
        try store.ingest(frame(type: "choice.selected", seq: 9, ["blockId": "c1", "optionIds": [String](), "other": "hand-rolled"]))
        #expect(store.pendingCount == 0)
        let received = await transport.received
        #expect(received == [.choice(blockId: "c1", optionIds: [], other: "hand-rolled")])
    }

    @Test("the echo drops the pending item when the frame outruns the POST reply (frame then response)")
    func reconcileFrameThenResponse() async throws {
        let transport = ScriptedTransport(seqs: [7])
        let store = BoardStore(subject: "s", transport: transport)

        let task = store.send(.input(blockId: "i1", text: "hello"))
        #expect(store.pendingCount == 1)
        // Deliver the echo synchronously, before the POST task body runs: the
        // pending item is untagged, so identity — not seq — must drop it.
        try store.ingest(frame(type: "input.submitted", seq: 7, ["blockId": "i1", "text": "hello"]))
        #expect(store.pendingCount == 0)

        await task.value // tag now finds the confirmed event already in the log
        #expect(store.pendingCount == 0)
        #expect(store.state.interactions.inputs["i1"]?.text == "hello")
    }

    @Test("append-only feedback is never double-counted across the echo")
    func feedbackNotDoubleCounted() async throws {
        let transport = ScriptedTransport(seqs: [3])
        let store = BoardStore(subject: "s", transport: transport)

        let id = "feedback-uuid"
        let task = store.send(.feedback(id: id, blockId: "b1", text: "looks good"))
        #expect(store.state.interactions.feedback["b1"]?.count == 1)

        await task.value
        try store.ingest(frame(type: "feedback.created", seq: 3, ["id": id, "blockId": "b1", "text": "looks good"]))
        #expect(store.pendingCount == 0)
        #expect(store.state.interactions.feedback["b1"]?.count == 1)
    }

    @Test("send records the interacting block, and submit (no block) leaves it untouched")
    func lastInteractedTracksBlockScopedSends() {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: [1, 2, 3]))
        #expect(store.lastInteracted == nil)

        store.send(.decision(blockId: "b1", verdict: .approved))
        #expect(store.lastInteracted == "b1")

        store.send(.feedback(id: "f", blockId: "b2", text: "note"))
        #expect(store.lastInteracted == "b2")

        // submit is document-scoped: it carries no block, so the pin holds.
        store.send(.submit(revision: 1))
        #expect(store.lastInteracted == "b2")
    }

    @Test("a closed board records no last-interacted block")
    func lastInteractedIgnoresClosedBoard() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        try store.ingest(frame(type: "present.closed", seq: 1))
        #expect(store.isClosed)

        store.send(.decision(blockId: "b1", verdict: .approved))
        #expect(store.lastInteracted == nil)
    }

    @Test("an interaction after present.closed is a no-op — no overlay, no POST")
    func interactionAfterCloseIsNoOp() async throws {
        let transport = ScriptedTransport(seqs: [])
        let store = BoardStore(subject: "s", transport: transport)

        try store.ingest(frame(type: "present.closed", seq: 1))
        #expect(store.isClosed)

        let task = store.send(.decision(blockId: "b1", verdict: .approved))
        await task.value
        #expect(store.pendingCount == 0)
        #expect(store.state.interactions.decisions["b1"] == nil)
        let received = await transport.received
        #expect(received.isEmpty)
    }

    @Test("the caught-up boundary clears the loading flag")
    func caughtUpClearsLoading() {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        #expect(store.isLoading)
        store.ingest(.caughtUp(seq: 0))
        #expect(!store.isLoading)
    }

    @Test("a failed POST rolls the optimistic overlay back")
    func failedPostRollsBack() async {
        let transport = FailingTransport()
        let store = BoardStore(subject: "s", transport: transport)

        let task = store.send(.decision(blockId: "b1", verdict: .rejected))
        #expect(store.state.interactions.decisions["b1"]?.verdict == "rejected")
        #expect(store.pendingCount == 1)

        await task.value
        #expect(store.pendingCount == 0)
        #expect(store.state.interactions.decisions["b1"] == nil)
    }

    @Test("replayed frames never mark; live frames past the boundary do")
    func revisionsGateOnCaughtUp() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        try store.ingest(frame(type: "block.upserted", seq: 1, ["block": ["id": "a", "type": "markdown", "md": "a0"]]))
        try store.ingest(frame(type: "block.upserted", seq: 2, ["block": ["id": "a", "type": "markdown", "md": "a1"]]))
        // Still replaying: an in-log rewrite must not badge.
        #expect(store.revisions.mark(for: "a") == nil)

        store.ingest(.caughtUp(seq: 2))
        try store.ingest(frame(type: "block.upserted", seq: 3, ["block": ["id": "a", "type": "markdown", "md": "a2"]]))
        #expect(store.revisions.mark(for: "a")?.kind == .revised)
    }

    @Test("two live frames applied back-to-back each mark — no render coalesces them")
    func revisionsBackToBackFramesEachMark() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        try store.ingest(frame(type: "block.upserted", seq: 1, ["block": ["id": "a", "type": "markdown", "md": "a0"]]))
        try store.ingest(frame(type: "block.upserted", seq: 2, ["block": ["id": "b", "type": "markdown", "md": "b0"]]))
        store.ingest(.caughtUp(seq: 2))

        // Two frames land with no observation cycle between them.
        try store.ingest(frame(type: "block.upserted", seq: 3, ["block": ["id": "a", "type": "markdown", "md": "a1"]]))
        try store.ingest(frame(type: "block.upserted", seq: 4, ["block": ["id": "b", "type": "markdown", "md": "b1"]]))
        #expect(store.revisions.mark(for: "a")?.kind == .revised)
        #expect(store.revisions.mark(for: "b")?.kind == .revised)
    }

    @Test("a revising.changed then its completing upsert lifts the note onto the mark")
    func revisionsNoteSurvivesReviseThenUpsert() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        try store.ingest(frame(type: "block.upserted", seq: 1, ["block": ["id": "x", "type": "markdown", "md": "old"]]))
        store.ingest(.caughtUp(seq: 1))

        // Announce, then complete in the very next frame: per-frame ingestion keeps the
        // note the completing upsert clears; a per-render observer would coalesce both.
        try store.ingest(frame(type: "revising.changed", seq: 2, ["blockIds": ["x"], "note": "reworking per your pick"]))
        try store.ingest(frame(type: "block.upserted", seq: 3, ["block": ["id": "x", "type": "markdown", "md": "new"]]))

        let mark = store.revisions.mark(for: "x")
        #expect(mark?.kind == .revised)
        #expect(mark?.note == "reworking per your pick")
        #expect(!store.revisions.isRevising("x"))
    }

    @Test("marks are recorded with no focus deck attached, surviving a later mode switch")
    func revisionsRecordedInBoardMode() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        try store.ingest(frame(type: "block.upserted", seq: 1, ["block": ["id": "x", "type": "markdown", "md": "old"]]))
        store.ingest(.caughtUp(seq: 1))
        // A frame lands while the board is in board mode — no FocusDeckView exists.
        try store.ingest(frame(type: "block.upserted", seq: 2, ["block": ["id": "y", "type": "markdown", "md": "fresh"]]))
        #expect(store.revisions.mark(for: "y")?.kind == .added)
    }

    @Test("a live doc.replaced frame clears marks instead of re-badging the whole board")
    func docReplacedClearsRevisionMarks() throws {
        let store = BoardStore(subject: "s", transport: ScriptedTransport(seqs: []))
        let firstDoc: [String: Any] = ["version": 1, "title": "", "blocks": [["id": "x", "type": "markdown", "md": "old"]]]
        try store.ingest(frame(type: "doc.replaced", seq: 1, ["doc": firstDoc, "revision": 0]))
        store.ingest(.caughtUp(seq: 1))
        // A live rewrite marks x past the caught-up boundary.
        try store.ingest(frame(type: "block.upserted", seq: 2, ["block": ["id": "x", "type": "markdown", "md": "new"]]))
        #expect(store.revisions.mark(for: "x") != nil)

        // A wholesale re-push is a fresh slate: marks clear, and neither the changed x
        // nor the fresh y is re-badged.
        let nextDoc: [String: Any] = ["version": 2, "title": "", "blocks": [
            ["id": "x", "type": "markdown", "md": "fresh"],
            ["id": "y", "type": "markdown", "md": "added"],
        ]]
        try store.ingest(frame(type: "doc.replaced", seq: 3, ["doc": nextDoc, "revision": 1]))
        #expect(store.revisions.mark(for: "x") == nil)
        #expect(store.revisions.mark(for: "y") == nil)
    }
}

/// FailingTransport always throws, to exercise optimistic rollback.
private struct FailingTransport: InteractionPoster {
    struct Boom: Error {}
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        throw Boom()
    }
}
