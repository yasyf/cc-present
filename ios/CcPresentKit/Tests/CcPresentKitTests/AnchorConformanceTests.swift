@testable import CcPresentKit
import Foundation
import Testing

/// repoRoot resolves the repository root from this source file's path so the
/// fixture loads regardless of the working directory `swift test` runs in.
private var repoRoot: URL {
    URL(fileURLWithPath: #filePath) // .../ios/CcPresentKit/Tests/CcPresentKitTests/AnchorConformanceTests.swift
        .deletingLastPathComponent() // CcPresentKitTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // CcPresentKit
        .deletingLastPathComponent() // ios
        .deletingLastPathComponent() // repo root
}

struct AnchorCorpus: Decodable {
    struct HashCase: Decodable {
        let line: String
        let hash: String
    }

    struct ParseCase: Decodable {
        let ref: String
        let line: Int?
        let end: Int?
        let hash: String?
        let error: Bool?
    }

    struct ResolveCase: Decodable {
        let ref: String
        let lines: [String]
        let start: Int?
        let end: Int?
        let moved: Bool?
        let from: Int?
        let error: String?
    }

    let hash: [HashCase]
    let parse: [ParseCase]
    let resolve: [ResolveCase]
}

private let anchorCorpus: AnchorCorpus? = {
    let url = repoRoot.appendingPathComponent("internal/anchor/testdata/anchors.json")
    guard let data = try? Data(contentsOf: url) else {
        return nil
    }
    return try? JSONDecoder().decode(AnchorCorpus.self, from: data)
}()

private let hashCases = anchorCorpus?.hash ?? []
private let parseCases = anchorCorpus?.parse ?? []
private let resolveCases = anchorCorpus?.resolve ?? []

@Suite("Anchor conformance")
struct AnchorConformanceTests {
    @Test("the anchor corpus is present in full")
    func corpusIsComplete() {
        #expect(hashCases.count >= 10, "expected >= 10 hash cases, found \(hashCases.count)")
        #expect(parseCases.count >= 12, "expected >= 12 parse cases, found \(parseCases.count)")
        #expect(resolveCases.count >= 9, "expected >= 9 resolve cases, found \(resolveCases.count)")
    }

    @Test("hashing matches every Go corpus case", arguments: hashCases)
    func hashes(_ testCase: AnchorCorpus.HashCase) {
        #expect(Anchor.of(testCase.line) == testCase.hash)
    }

    @Test("parsing matches every Go corpus case", arguments: parseCases)
    func parses(_ testCase: AnchorCorpus.ParseCase) throws {
        if testCase.error == true {
            do {
                let parsed = try Anchor.parse(testCase.ref)
                Issue.record("expected malformed error for \(testCase.ref), got \(parsed)")
            } catch is Anchor.AnchorError {
            } catch {
                Issue.record("expected AnchorError for \(testCase.ref), got \(error)")
            }
            return
        }

        let parsed = try Anchor.parse(testCase.ref)
        #expect(parsed == Anchor.Ref(
            line: testCase.line ?? 0,
            end: testCase.end ?? 0,
            hash: testCase.hash ?? ""
        ))
    }

    @Test("resolution matches every Go corpus case", arguments: resolveCases)
    func resolves(_ testCase: AnchorCorpus.ResolveCase) throws {
        let ref = try Anchor.parse(testCase.ref)
        if let expectedError = testCase.error {
            do {
                let resolution = try Anchor.resolve(ref, lines: testCase.lines)
                Issue.record("expected \(expectedError) for \(testCase.ref), got \(resolution)")
            } catch let error as Anchor.AnchorError {
                switch (expectedError, error) {
                case ("not found", .notFound(hash: _)),
                     ("ambiguous", .ambiguous(hash: _, candidates: _)):
                    break
                default:
                    Issue.record("expected \(expectedError) for \(testCase.ref), got \(error)")
                }
            } catch {
                Issue.record("expected AnchorError for \(testCase.ref), got \(error)")
            }
            return
        }

        let resolution = try Anchor.resolve(ref, lines: testCase.lines)
        #expect(resolution == Anchor.Resolution(
            start: testCase.start ?? 0,
            end: testCase.end ?? 0,
            moved: testCase.moved ?? false,
            from: testCase.from ?? 0
        ))
    }

    @Test("U+FEFF is not trimmed before hashing")
    func byteOrderMarkChangesHash() {
        #expect(Anchor.of("\u{FEFF}return nil") != Anchor.of("return nil"))
    }
}
