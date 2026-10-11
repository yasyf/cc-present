@testable import CcPresentKit
import Foundation
import Testing

/// repoRoot resolves the repository root from this source file's path so the
/// fixtures load regardless of the working directory `swift test` runs in.
private var repoRoot: URL {
    URL(fileURLWithPath: #filePath) // .../ios/CcPresentKit/Tests/CcPresentKitTests/ReduceConformanceTests.swift
        .deletingLastPathComponent() // CcPresentKitTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // CcPresentKit
        .deletingLastPathComponent() // ios
        .deletingLastPathComponent() // repo root
}

/// fixtureURLs is every internal/state/testdata/*.json fixture, sorted by name.
/// The Go reducer's own conformance corpus (internal/state/reduce_test.go) drives
/// this same set; a Swift port that diverges on any one of them is a port bug.
private let fixtureURLs: [URL] = {
    let dir = repoRoot.appendingPathComponent("internal/state/testdata")
    // A broken repo-root walk yields an empty set; corpusIsComplete fails loud on
    // that rather than this initializer crashing during test collection.
    let entries = (try? FileManager.default.contentsOfDirectory(
        at: dir, includingPropertiesForKeys: nil
    )) ?? []
    return entries
        .filter { $0.pathExtension == "json" }
        .sorted { $0.lastPathComponent < $1.lastPathComponent }
}()

/// Fixture is the {name, events, expected} schema each testdata file carries,
/// mirroring internal/state/reduce_test.go's fixture struct. `expected` decodes
/// through BoardState's defaulting decoder, which fills the empty maps and the
/// current-round default exactly as reduce_test.go's initMaps does.
private struct Fixture: Decodable {
    let name: String
    let events: [Event]
    let expected: BoardState
}

@Suite("Reduce conformance")
struct ReduceConformanceTests {
    /// The corpus must not silently shrink to nothing: a broken repo-root walk or
    /// an empty testdata directory has to fail loud rather than pass on zero files.
    @Test("the testdata corpus is present in full")
    func corpusIsComplete() {
        #expect(
            fixtureURLs.count >= 24,
            "expected >= 24 fixtures under internal/state/testdata, found \(fixtureURLs.count)"
        )
    }

    @Test("the corpus carries the round-scoped viewed fixtures")
    func viewedFixturesPresent() {
        let names = Set(fixtureURLs.map(\.lastPathComponent))
        #expect(names.isSuperset(of: [
            "submit-viewed.json",
            "submit-viewed-round-partition.json",
            "submit-viewed-stale-round.json",
        ]))
    }

    @Test("stored blocks carrying keys the schema no longer defines reduce as if the keys were absent")
    func ignoresUnknownKeysInStoredBlocks() throws {
        let stored = """
        [
          {"origin": "agent", "type": "doc.replaced", "seq": 1, "payload": {"schemaVersion": 1, "type": "doc.replaced", "revision": 1, "doc": {
            "version": 1, "title": "T", "legacy": true, "blocks": [
              {"id": "c1", "type": "card", "legacy": true, "children": [{"id": "a1", "type": "approval", "allowFeedback": false}]},
              {"id": "a2", "type": "approval", "allowFeedback": true}
            ]
          }}},
          {"origin": "agent", "type": "block.upserted", "seq": 2, "payload": {"schemaVersion": 1, "type": "block.upserted", "block": {"id": "a3", "type": "approval", "allowFeedback": false}}},
          {"origin": "agent", "type": "block.upserted", "seq": 3, "payload": {"schemaVersion": 1, "type": "block.upserted", "after": "a1", "block": {"id": "a4", "type": "approval", "allowFeedback": false}}}
        ]
        """
        let current = """
        [
          {"origin": "agent", "type": "doc.replaced", "seq": 1, "payload": {"schemaVersion": 1, "type": "doc.replaced", "revision": 1, "doc": {
            "version": 1, "title": "T", "blocks": [
              {"id": "c1", "type": "card", "children": [{"id": "a1", "type": "approval"}]},
              {"id": "a2", "type": "approval"}
            ]
          }}},
          {"origin": "agent", "type": "block.upserted", "seq": 2, "payload": {"schemaVersion": 1, "type": "block.upserted", "block": {"id": "a3", "type": "approval"}}},
          {"origin": "agent", "type": "block.upserted", "seq": 3, "payload": {"schemaVersion": 1, "type": "block.upserted", "after": "a1", "block": {"id": "a4", "type": "approval"}}}
        ]
        """
        let decoder = JSONDecoder()
        let got = try reduce(events: decoder.decode([Event].self, from: Data(stored.utf8)))
        let want = try reduce(events: decoder.decode([Event].self, from: Data(current.utf8)))
        #expect(got == want)
        #expect(got.doc.blocks.count == 3)
    }

    @Test("reduce matches every Go state fixture", arguments: fixtureURLs)
    func matchesFixture(_ url: URL) throws {
        let data = try Data(contentsOf: url)
        let fixture = try JSONDecoder().decode(Fixture.self, from: data)
        let got = try reduce(events: fixture.events)
        #expect(
            got == fixture.expected,
            "fixture \(fixture.name) (\(url.lastPathComponent)) diverged from Go reduce"
        )
    }
}
