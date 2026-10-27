@testable import CcPresentKit
import Foundation
import Testing

private struct InertTransport: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        0
    }
}

private let snapshotJSON = """
{"seq":7,"revision":1,"state":{"schemaVersion":1,
 "doc":{"version":1,"title":"T","blocks":[{"id":"m2","type":"markdown","md":"now"}]},
 "interactions":{"decisions":{},"choices":{},"inputs":{},"packs":{},"feedback":{},"replies":{},"annotations":{},"triage":{},"viewed":{},
  "submitted":{"value":false,"revision":0},"closed":{"value":false}},
 "rounds":{"current":2,"blockRounds":{"m2":2},"history":[
  {"number":1,"title":"First","outline":[{"id":"k1","type":"card","label":"Rollout","children":["ka1"]}],
   "feedback":{"ka1":[{"id":"f1","text":"why now?"}]}}]},
 "revising":{"blockIds":[]}}}
"""

@MainActor
@Suite("BoardStore snapshot load")
struct SnapshotTests {
    @Test("a snapshot's closed round decodes as a stub with its outline and notes")
    func decodesStub() throws {
        let snapshot = try JSONDecoder().decode(Snapshot.self, from: Data(snapshotJSON.utf8))
        let round = try #require(snapshot.state.rounds.history.first)
        #expect(snapshot.seq == 7)
        #expect(round.isStub)
        #expect(round.blocks.isEmpty)
        #expect(round.outline?.map(\.id) == ["k1"])
        #expect(round.outline?.first?.children == ["ka1"])
        #expect(round.feedback["ka1"]?.map(\.id) == ["f1"])
    }

    @Test("the store renders the snapshot and folds later frames onto it")
    func loadsThenReduces() throws {
        let store = BoardStore(subject: "s", transport: InertTransport())
        try store.load(JSONDecoder().decode(Snapshot.self, from: Data(snapshotJSON.utf8)))
        #expect(store.state.doc.blocks.map(\.id) == ["m2"])
        #expect(store.state.rounds.current == 2)

        let data = try JSONSerialization.data(withJSONObject: [
            "schemaVersion": 1, "type": "feedback.created", "id": "f2", "blockId": "m2", "text": "ok",
        ])
        try store.ingest(.frame(Event.wireFrame(data, seq: 8)))
        #expect(store.state.interactions.feedback["m2"]?.map(\.id) == ["f2"])
        #expect(store.state.rounds.history.first?.isStub == true)
    }

    @Test("fillRounds swaps a stub for its full record")
    func fillsRounds() throws {
        let store = BoardStore(subject: "s", transport: InertTransport())
        try store.load(JSONDecoder().decode(Snapshot.self, from: Data(snapshotJSON.utf8)))
        let full = RoundRecord(number: 1, title: "First", blocks: [.markdown(Block.Markdown(id: "k1", md: "frozen"))])
        store.fillRounds([full])
        #expect(store.state.rounds.history == [full])
    }
}
