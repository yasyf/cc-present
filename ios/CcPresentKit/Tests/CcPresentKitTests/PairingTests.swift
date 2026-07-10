@testable import CcPresentKit
import Foundation
import Testing

@Suite("Pairing")
struct PairingTests {
    @Test("a well-formed v1 payload decodes")
    func pairPayloadDecodesGood() throws {
        let json = #"{"v":1,"url":"http://192.168.1.5:8765","token":"secret-abc"}"#
        let payload = try JSONDecoder().decode(PairPayload.self, from: Data(json.utf8))
        #expect(payload.version == 1)
        #expect(payload.url == URL(string: "http://192.168.1.5:8765"))
        #expect(payload.token == "secret-abc")
    }

    @Test("an unsupported version is rejected with PairError")
    func pairPayloadRejectsVersion() {
        let json = #"{"v":2,"url":"http://192.168.1.5:8765","token":"secret-abc"}"#
        #expect(throws: PairError.unsupportedVersion(2)) {
            try JSONDecoder().decode(PairPayload.self, from: Data(json.utf8))
        }
    }

    @Test("a payload missing the token fails to decode")
    func pairPayloadRejectsMissingToken() {
        let json = #"{"v":1,"url":"http://192.168.1.5:8765"}"#
        #expect(throws: (any Error).self) {
            try JSONDecoder().decode(PairPayload.self, from: Data(json.utf8))
        }
    }

    @Test("malformed JSON fails to decode")
    func pairPayloadRejectsGarbage() {
        #expect(throws: (any Error).self) {
            try JSONDecoder().decode(PairPayload.self, from: Data("not json".utf8))
        }
    }

    @Test("the machine roster round-trips through an injected directory")
    func machineStoreRoundTrip() throws {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent("cc-present-tests-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }

        let store = MachineStore(directory: directory)
        #expect(try store.load().isEmpty)

        let machines = try [
            Machine(id: "m1", name: "Laptop", baseURL: #require(URL(string: "http://10.0.0.2:8765"))),
            Machine(id: "m2", name: "Studio", baseURL: #require(URL(string: "http://10.0.0.3:8765"))),
        ]
        try store.save(machines)

        let loaded = try store.load()
        #expect(loaded == machines)
    }
}
