@testable import CcPresentKit
import Foundation
import Testing

private let encoder = JSONEncoder()
private let decoder = JSONDecoder()

private func decodeBlock(_ json: String) throws -> Block {
    try decoder.decode(Block.self, from: Data(json.utf8))
}

private func decodeEvent(_ json: String) throws -> Event {
    try decoder.decode(Event.self, from: Data(json.utf8))
}

@Suite("Pack blocks")
struct PackBlockTests {
    @Test("a dotted-type block decodes to .pack with id and packType populated")
    func dottedTypeDecodesToPack() throws {
        let block = try decodeBlock(#"{"id":"cal-1","type":"example.callout","tone":"warn","body":"heads up"}"#)
        guard case let .pack(pack) = block else {
            Issue.record("expected a .pack case, got \(block)")
            return
        }
        #expect(pack.id == "cal-1")
        #expect(pack.packType == "example.callout")
        #expect(block.id == "cal-1")
        #expect(block.type == "example.callout")
        #expect(pack.raw == .object([
            "id": .string("cal-1"),
            "type": .string("example.callout"),
            "tone": .string("warn"),
            "body": .string("heads up"),
        ]))
    }

    @Test("a pack block round-trips Equatable-equal through encode then decode")
    func packBlockRoundTripsEqual() throws {
        let json = #"{"id":"rate-1","type":"example.rating","min":1,"max":5,"value":3}"#
        let block = try decodeBlock(json)
        let again = try decoder.decode(Block.self, from: encoder.encode(block))
        #expect(again == block)
    }

    @Test("re-encoding a pack block preserves the whole body byte-meaning-faithfully")
    func packBlockPreservesUnknownFields() throws {
        // Every JSON scalar shape plus nesting: a round trip through .pack must keep
        // ints integral, floats floating, and unmodeled fields intact.
        let json = #"""
        {"id":"cal-2","type":"example.callout","tone":"info","count":3,"ratio":1.5,
         "enabled":true,"disabled":false,"empty":null,"items":["a","b"],
         "meta":{"nested":42,"deep":{"flag":true}}}
        """#
        let data = Data(json.utf8)
        let block = try decodeBlock(json)
        guard case let .pack(pack) = block else {
            Issue.record("expected a .pack case, got \(block)")
            return
        }

        let original = try decoder.decode(JSONValue.self, from: data)
        let reencoded = try decoder.decode(JSONValue.self, from: encoder.encode(block))
        #expect(pack.raw == original)
        #expect(reencoded == original)
    }

    @Test("a dot-free unknown block type still throws a decode error naming the block id")
    func dotFreeUnknownTypeThrows() throws {
        let json = Data(#"{"id":"weird","type":"bogus","x":1}"#.utf8)
        #expect(throws: DecodingError.self) {
            _ = try decoder.decode(Block.self, from: json)
        }
        do {
            _ = try decoder.decode(Block.self, from: json)
            Issue.record("expected a thrown error")
        } catch let DecodingError.dataCorrupted(context) {
            #expect(context.debugDescription.contains("weird"))
            #expect(context.debugDescription.contains("bogus"))
        }
    }

    @Test("a document mixing built-in blocks and one pack block decodes and round-trips")
    func mixedDocumentDecodes() throws {
        let json = #"""
        {"version":1,"title":"Mixed","blocks":[
         {"id":"s","type":"section","title":"Sec"},
         {"id":"ex-1","type":"example.callout","tone":"info","body":"hi"},
         {"id":"m","type":"markdown","md":"text"}
        ]}
        """#
        let doc = try decoder.decode(Doc.self, from: Data(json.utf8))
        #expect(doc.blocks.count == 3)
        guard case .section = doc.blocks[0] else {
            Issue.record("first block is not a section")
            return
        }
        guard case let .pack(pack) = doc.blocks[1] else {
            Issue.record("second block is not a pack")
            return
        }
        #expect(pack.id == "ex-1")
        #expect(pack.packType == "example.callout")
        guard case .markdown = doc.blocks[2] else {
            Issue.record("third block is not markdown")
            return
        }
        let again = try decoder.decode(Doc.self, from: encoder.encode(doc))
        #expect(again == doc)
    }

    @Test("a pack.interaction wire frame decodes to the typed payload")
    func packInteractionWireFrameDecodes() throws {
        let frame = Data(#"{"type":"pack.interaction","blockId":"ex-rating","payload":{"value":4}}"#.utf8)
        let event = try Event.wireFrame(frame, seq: 7)
        #expect(event.type == "pack.interaction")
        guard case let .packInteraction(payload) = try event.payload else {
            Issue.record("expected a .packInteraction payload")
            return
        }
        #expect(payload.blockId == "ex-rating")
        #expect(payload.payload == .object(["value": .int(4)]))
    }

    @Test("pack.interaction reduces last-write-wins and the closed round snapshots packs")
    func packInteractionReducesLWWAndSnapshots() throws {
        let events = try [
            decodeEvent(#"""
            {"origin":"agent","type":"doc.replaced","seq":1,"payload":{"doc":{"version":1,"title":"T",
             "blocks":[{"id":"ex-rating","type":"example.rating","label":"Rate"}]},"revision":1}}
            """#),
            decodeEvent(#"""
            {"origin":"human","type":"pack.interaction","seq":2,"payload":{"blockId":"ex-rating","payload":{"value":2}}}
            """#),
            decodeEvent(#"""
            {"origin":"human","type":"pack.interaction","seq":3,"payload":{"blockId":"ex-rating","payload":{"value":5}}}
            """#),
            decodeEvent(#"""
            {"origin":"human","type":"submit","seq":4,"payload":{"revision":1}}
            """#),
        ]
        let state = try reduce(events: events)

        // Last write wins: the seq-3 value replaces the seq-2 value.
        #expect(state.interactions.packs["ex-rating"] == .object(["value": .int(5)]))

        // The submit closed the dirty round and snapshotted the pack value into history.
        #expect(state.rounds.history.count == 1)
        let round = try #require(state.rounds.history.first)
        #expect(round.packs["ex-rating"] == .object(["value": .int(5)]))
        #expect(round.submittedRevision == 1)
    }
}
