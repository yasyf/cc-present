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
