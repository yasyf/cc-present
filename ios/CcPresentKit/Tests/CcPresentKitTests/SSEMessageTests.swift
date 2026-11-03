@testable import CcPresentKit
import IOSKit
import Testing

@Suite("SSE message mapping")
struct SSEMessageTests {
    @Test("a default frame maps to a stamped Event; the caught-up marker maps to its seq")
    func messageMapping() throws {
        let frame = SSEEvent(event: "", data: #"{"schemaVersion":1,"type":"block.removed","id":"b1"}"#, lastEventID: "12")
        let frameMessage = try #require(SSEClient.message(from: frame))
        guard case let .frame(event) = frameMessage else {
            Issue.record("expected a frame message"); return
        }
        #expect(event.type == "block.removed")
        #expect(event.seq == 12)
        guard case let .blockRemoved(payload) = try event.payload else {
            Issue.record("expected a block.removed payload"); return
        }
        #expect(payload.id == "b1")

        let marker = SSEEvent(event: "caught-up", data: #"{"seq":99}"#, lastEventID: nil)
        #expect(SSEClient.message(from: marker) == .caughtUp(seq: 99))
    }

    @Test("an unknown named event and a malformed frame are dropped")
    func dropsUnknownAndMalformed() {
        #expect(SSEClient.message(from: SSEEvent(event: "ping", data: "{}", lastEventID: nil)) == nil)
        #expect(SSEClient.message(from: SSEEvent(event: "", data: "not json", lastEventID: "3")) == nil)
        #expect(SSEClient.message(from: SSEEvent(event: "caught-up", data: "{}", lastEventID: nil)) == nil)
    }
}
