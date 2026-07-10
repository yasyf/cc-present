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
}

/// FailingTransport always throws, to exercise optimistic rollback.
private struct FailingTransport: InteractionPoster {
    struct Boom: Error {}
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        throw Boom()
    }
}
