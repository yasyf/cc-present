@testable import CcPresentKit
import CoreGraphics
import Foundation
import Testing

private actor RecordingTransport: InteractionPoster {
    private(set) var received: [Interaction] = []

    func postInteraction(subject _: String, interaction: Interaction) async throws -> Int64 {
        received.append(interaction)
        return Int64(received.count)
    }
}

private func isolatedDefaults() -> UserDefaults {
    UserDefaults(suiteName: "cc-present.viewed-tests.\(UUID().uuidString)")!
}

private func frame(type: String, seq: Int64, _ fields: [String: Any] = [:]) throws -> SSEClient.Message {
    var object = fields
    object["schemaVersion"] = 1
    object["type"] = type
    return try .frame(Event.wireFrame(JSONSerialization.data(withJSONObject: object), seq: seq))
}

@MainActor
@Suite("Viewed store")
struct ViewedStoreTests {
    @Test("mark dedupes in first-seen order and persists per subject and round")
    func markPersistsAcrossInstances() {
        let defaults = isolatedDefaults()
        let store = ViewedStore(subject: "s", defaults: defaults)
        store.scope(round: 2)
        store.mark(["a", "b"])
        store.mark(["b", "c"])
        #expect(store.viewed == ["a", "b", "c"])

        let relaunched = ViewedStore(subject: "s", defaults: defaults)
        relaunched.scope(round: 2)
        #expect(relaunched.viewed == ["a", "b", "c"])

        let other = ViewedStore(subject: "t", defaults: defaults)
        other.scope(round: 2)
        #expect(other.viewed.isEmpty)
    }

    @Test("scoping to the next round clears the set, and a stale stored round never reloads")
    func roundChangeClears() {
        let defaults = isolatedDefaults()
        let store = ViewedStore(subject: "s", defaults: defaults)
        store.scope(round: 1)
        store.mark(["a"])
        store.scope(round: 2)
        #expect(store.viewed.isEmpty)

        let relaunched = ViewedStore(subject: "s", defaults: defaults)
        relaunched.scope(round: 2)
        #expect(relaunched.viewed.isEmpty)
    }
}

@Suite("Viewed dwell")
struct DwellTests {
    private let start = ContinuousClock.now

    @Test("the focus dwell completes once after 800 ms unbroken")
    func focusDwellCompletesOnce() {
        var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.focusDwell)
        let deadline = dwell.update(active: true, at: start)
        #expect(deadline == start.advanced(by: .milliseconds(800)))
        let early = dwell.complete(at: start.advanced(by: .milliseconds(799)))
        let onTime = dwell.complete(at: start.advanced(by: .milliseconds(800)))
        let again = dwell.complete(at: start.advanced(by: .milliseconds(900)))
        let rearmed = dwell.update(active: true, at: start.advanced(by: .seconds(1)))
        #expect(!early)
        #expect(onTime)
        #expect(!again)
        #expect(rearmed == nil)
    }

    @Test("staying on screen keeps the first deadline; leaving restarts the 1.5 s wait")
    func boardDwellRestartsOnLeave() {
        var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.boardDwell)
        let first = dwell.update(active: true, at: start)
        let held = dwell.update(active: true, at: start.advanced(by: .milliseconds(500)))
        #expect(held == first)

        let left = dwell.update(active: false, at: start.advanced(by: .milliseconds(1000)))
        let afterLeaving = dwell.complete(at: start.advanced(by: .milliseconds(1500)))
        #expect(left == nil)
        #expect(!afterLeaving)

        let back = start.advanced(by: .milliseconds(1200))
        let restarted = dwell.update(active: true, at: back)
        let early = dwell.complete(at: back.advanced(by: .milliseconds(1499)))
        let onTime = dwell.complete(at: back.advanced(by: .milliseconds(1500)))
        #expect(restarted == back.advanced(by: .milliseconds(1500)))
        #expect(!early)
        #expect(onTime)
    }
}

@Suite("Viewed on-screen rule")
struct OnScreenTests {
    private let viewport = CGRect(x: 0, y: 0, width: 400, height: 800)

    @Test("a row at least half visible counts; under half does not")
    func halfVisible() {
        #expect(isViewedOnScreen(row: CGRect(x: 0, y: 700, width: 400, height: 200), viewport: viewport))
        #expect(!isViewedOnScreen(row: CGRect(x: 0, y: 750, width: 400, height: 200), viewport: viewport))
        #expect(!isViewedOnScreen(row: CGRect(x: 0, y: 900, width: 400, height: 200), viewport: viewport))
    }

    @Test("a row taller than the viewport counts while it spans the midline")
    func tallRowSpansMidline() {
        #expect(isViewedOnScreen(row: CGRect(x: 0, y: -1000, width: 400, height: 3000), viewport: viewport))
        #expect(!isViewedOnScreen(row: CGRect(x: 0, y: -2500, width: 400, height: 2800), viewport: viewport))
    }
}

@MainActor
@Suite("Viewed on submit")
struct ViewedSubmitTests {
    @Test("the submit payload encodes viewed beside the revision")
    func submitEncodesViewed() throws {
        let data = try JSONEncoder().encode(Interaction.submit(revision: 3, viewed: ["a", "c1"]))
        let object = try #require(JSONSerialization.jsonObject(with: data) as? [String: Any])
        #expect(object["type"] as? String == "submit")
        #expect(object["revision"] as? Int == 3)
        #expect(object["viewed"] as? [String] == ["a", "c1"])
    }

    @Test("acting on a block marks it viewed and the store's submit carries the set")
    func storeSubmitCarriesViewed() async throws {
        let transport = RecordingTransport()
        let store = BoardStore(subject: "s", transport: transport, defaults: isolatedDefaults())
        try store.ingest(frame(type: "doc.replaced", seq: 1, [
            "revision": 1,
            "doc": ["version": 1, "title": "T", "blocks": [["id": "a1", "type": "approval", "prompt": "Ship?"]]],
        ]))
        store.viewed.mark(["m1"])
        store.send(.decision(blockId: "a1", verdict: .approved))

        await store.submit(revision: 1).value

        let received = await transport.received
        #expect(received.last == .submit(revision: 1, viewed: ["m1", "a1"]))
    }

    @Test("a submit that closes the round starts the next round's viewed set empty")
    func roundCloseClearsStore() throws {
        let store = BoardStore(subject: "s", transport: RecordingTransport(), defaults: isolatedDefaults())
        try store.ingest(frame(type: "doc.replaced", seq: 1, [
            "revision": 1,
            "doc": ["version": 1, "title": "T", "blocks": [["id": "m", "type": "markdown", "md": "x"]]],
        ]))
        store.viewed.mark(["m"])
        try store.ingest(frame(type: "submit", seq: 2, ["revision": 1, "viewed": ["m"]]))

        #expect(store.state.rounds.current == 2)
        #expect(store.state.rounds.history.first?.viewed == ["m": true])
        #expect(store.viewed.round == 2)
        #expect(store.viewed.viewed.isEmpty)
    }
}
