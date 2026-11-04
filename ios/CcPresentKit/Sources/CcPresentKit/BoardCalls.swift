// The board calls of the cc-present REST plane (internal/daemon/rest.go), layered on
// ioskit's HTTPClient; they mirror the browser reference in web/src/api.ts.

import Foundation
import IOSKit

/// Interaction is one human action the client submits to POST /api/interactions.
/// It is the discriminated union over the human event payloads, tagged with the
/// wire `type`, and it encodes to exactly the frame shape the SSE echo delivers
/// (`{type, blockId?, …}`), so BoardStore reuses it as both the POST body and the
/// optimistic overlay event. Feedback carries a client-generated `id`, like the
/// request nonce, so a retry is idempotent. Submit carries the blocks the human
/// opened this round as `viewed`.
public enum Interaction: Encodable, Equatable, Sendable {
    case decision(blockId: String, verdict: Verdict, note: String? = nil)
    case feedback(id: String, blockId: String, text: String)
    case choice(blockId: String, optionIds: [String], other: String? = nil)
    case input(blockId: String, text: String)
    case annotation(id: String, blockId: String, anchor: String, text: String, quote: String)
    case annotationRemoved(id: String, blockId: String)
    case triage(blockId: String, verdicts: [String: TriageVerdict])
    case submit(revision: Int, viewed: [String])

    /// type is the wire discriminant, matching the human event names.
    public var type: String {
        switch self {
        case .decision: "decision.created"
        case .feedback: "feedback.created"
        case .choice: "choice.selected"
        case .input: "input.submitted"
        case .annotation: "annotation.created"
        case .annotationRemoved: "annotation.removed"
        case .triage: "triage.decided"
        case .submit: "submit"
        }
    }

    /// blockId is the enclosing block for a block-scoped interaction; submit is
    /// document-scoped and carries none.
    public var blockId: String? {
        switch self {
        case let .decision(blockId, _, _): blockId
        case let .feedback(_, blockId, _): blockId
        case let .choice(blockId, _, _): blockId
        case let .input(blockId, _): blockId
        case let .annotation(_, blockId, _, _, _): blockId
        case let .annotationRemoved(_, blockId): blockId
        case let .triage(blockId, _): blockId
        case .submit: nil
        }
    }

    private enum CodingKeys: String, CodingKey {
        case type, blockId, verdict, note, id, text, optionIds, other, anchor, quote, verdicts, revision, viewed
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(type, forKey: .type)
        switch self {
        case let .decision(blockId, verdict, note):
            try container.encode(blockId, forKey: .blockId)
            try container.encode(verdict, forKey: .verdict)
            try container.encodeIfPresent(note, forKey: .note)
        case let .feedback(id, blockId, text):
            try container.encode(id, forKey: .id)
            try container.encode(blockId, forKey: .blockId)
            try container.encode(text, forKey: .text)
        case let .choice(blockId, optionIds, other):
            try container.encode(blockId, forKey: .blockId)
            try container.encode(optionIds, forKey: .optionIds)
            try container.encodeIfPresent(other, forKey: .other)
        case let .input(blockId, text):
            try container.encode(blockId, forKey: .blockId)
            try container.encode(text, forKey: .text)
        case let .annotation(id, blockId, anchor, text, quote):
            try container.encode(id, forKey: .id)
            try container.encode(blockId, forKey: .blockId)
            try container.encode(anchor, forKey: .anchor)
            try container.encode(text, forKey: .text)
            try container.encode(quote, forKey: .quote)
        case let .annotationRemoved(id, blockId):
            try container.encode(id, forKey: .id)
            try container.encode(blockId, forKey: .blockId)
        case let .triage(blockId, verdicts):
            try container.encode(blockId, forKey: .blockId)
            try container.encode(verdicts, forKey: .verdicts)
        case let .submit(revision, viewed):
            try container.encode(revision, forKey: .revision)
            try container.encode(viewed, forKey: .viewed)
        }
    }
}

/// PacksResponse is the slice of GET /api/packs the client models: each registered
/// pack's block types with their declared `interactive` and `optional` flags. It drops the fields
/// the native client never reads (bundle, styles, schemas, dropped) so decoding is
/// forgiving of the wider contract shape (docs/contract.md — PacksResponse).
public struct PacksResponse: Decodable, Equatable, Sendable {
    /// Pack is one registered block pack: only its block-type declarations are modeled.
    public struct Pack: Decodable, Equatable, Sendable {
        /// BlockType is one block a pack contributes, tagged with its interactivity; an
        /// optional interactive block keeps its focus step but never joins the submit tally.
        public struct BlockType: Decodable, Equatable, Sendable {
            public let type: String
            public let interactive: Bool
            public let optional: Bool

            public init(type: String, interactive: Bool, optional: Bool = false) {
                self.type = type
                self.interactive = interactive
                self.optional = optional
            }
        }

        public let blocks: [BlockType]

        public init(blocks: [BlockType]) {
            self.blocks = blocks
        }
    }

    public let packs: [Pack]

    public init(packs: [Pack]) {
        self.packs = packs
    }

    /// interactiveTypes is the set of pack block types the manifest declares
    /// interactive — the classification the focus deck and SubmitBar tally by.
    public var interactiveTypes: Set<String> {
        Set(packs.flatMap(\.blocks).filter(\.interactive).map(\.type))
    }

    /// optionalTypes is the set of pack block types the manifest marks optional, which
    /// the submit tally skips while their focus steps stay navigable.
    public var optionalTypes: Set<String> {
        Set(packs.flatMap(\.blocks).filter(\.optional).map(\.type))
    }
}

/// InteractionPoster is the one call BoardStore needs from the network: submit a
/// human interaction and return the seq the daemon assigned it. HTTPClient is the
/// production conformer; tests inject a scripted fake.
public protocol InteractionPoster: Sendable {
    func postInteraction(subject: String, interaction: Interaction) async throws -> Int64
}

extension HTTPClient: InteractionPoster {
    /// postInteraction POSTs one interaction to /api/interactions as
    /// `{subject, nonce, interaction}` with a fresh nonce, and returns the seq the
    /// daemon assigned the appended event. It throws APIError for a non-2xx reply,
    /// so a 409 close or a 400 validation failure surfaces to the caller.
    public func postInteraction(subject: String, interaction: Interaction) async throws -> Int64 {
        let body = InteractionRequest(subject: subject, nonce: UUID().uuidString, interaction: interaction)
        return try await (post("api/interactions", json: body) as SeqResponse).seq
    }
}

public extension HTTPClient {
    /// packs GETs /api/packs, the daemon's pack manifest, so the focus deck can
    /// classify which pack block types are interactive instead of assuming all are.
    func packs() async throws -> PacksResponse {
        try await get("api/packs")
    }

    /// snapshot GETs /api/state, the subject's reduced board with each closed round
    /// stubbed, plus the seq of the last event it folds in, where the stream resumes.
    func snapshot(subject: String) async throws -> Snapshot {
        try await get("api/state", query: [URLQueryItem(name: "session", value: subject)])
    }

    /// rounds GETs /api/rounds, every closed round with its frozen blocks.
    func rounds(subject: String) async throws -> [RoundRecord] {
        try await (get("api/rounds", query: [URLQueryItem(name: "session", value: subject)]) as RoundsResponse).history
    }

    /// assetRequest builds an authorized GET for /assets/<sha>. SwiftUI's
    /// AsyncImage cannot attach headers, so the app hands it this pre-decorated
    /// request instead of a bare URL.
    func assetRequest(sha: String) -> URLRequest {
        request("assets/\(sha)")
    }

    func pr(subject: String, blockId: String, repo: String, number: Int) async throws -> PRLive {
        try await get("api/pr", query: [
            URLQueryItem(name: "subject", value: subject),
            URLQueryItem(name: "block", value: blockId),
            URLQueryItem(name: "repo", value: repo),
            URLQueryItem(name: "number", value: String(number)),
        ])
    }
}

public struct PRLive: Decodable, Equatable, Sendable {
    public enum State: String, Decodable, Sendable {
        case open, draft, merged, landed, closed
    }

    public struct Checks: Decodable, Equatable, Sendable {
        public let state: String
        public let total: Int
        public let passed: Int
        public let failed: Int
        public let pending: Int
        public let skipped: Int
        public let failing: [String]?
    }

    public struct Review: Decodable, Equatable, Sendable {
        public let author: String
        public let state: String
    }

    public struct File: Decodable, Equatable, Sendable {
        public let path: String
        public let previousPath: String?
        public let status: String
        public let additions: Int
        public let deletions: Int
        public let patch: String?
    }

    public let repo: String
    public let number: Int
    public let title: String
    public let url: URL
    public let author: String
    public let state: State
    public let base: String
    public let head: String
    public let headSha: String
    public let mergedSha: String?
    public let additions: Int
    public let deletions: Int
    public let changedFiles: Int
    public let checks: Checks
    public let reviewDecision: String
    public let reviews: [Review]
    public let labels: [String]
    public let files: [File]
    public let fetchedAt: String
}

/// InteractionRequest is the POST /api/interactions body: the subject, a fresh
/// dedup nonce, and the interaction union.
private struct InteractionRequest: Encodable {
    let subject: String
    let nonce: String
    let interaction: Interaction
}

/// SeqResponse is the daemon's reply to an accepted interaction: the assigned seq.
private struct SeqResponse: Decodable {
    let seq: Int64
}

/// Snapshot is the daemon's GET /api/state reply: the reduced board and the seq of
/// the last event folded into it.
public struct Snapshot: Decodable, Sendable {
    public let seq: Int64
    public let state: BoardState
}

private struct RoundsResponse: Decodable {
    let history: [RoundRecord]
}
