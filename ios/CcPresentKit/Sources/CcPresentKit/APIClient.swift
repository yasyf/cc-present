// The REST edge of the cc-present client: a thin, Sendable wrapper over
// URLSession that speaks the daemon's HTTP plane (internal/daemon/rest.go). It
// carries a base URL and an optional bearer token, mirrors the browser reference
// in web/src/api.ts, and stays free of app state so BoardStore can inject a fake
// InteractionPoster in tests.

import Foundation

/// Interaction is one human action the client submits to POST /api/interactions.
/// It is the discriminated union over the human event payloads, tagged with the
/// wire `type`, and it encodes to exactly the frame shape the SSE echo delivers
/// (`{type, blockId?, …}`), so BoardStore reuses it as both the POST body and the
/// optimistic overlay event. Feedback carries a client-generated `id`, like the
/// request nonce, so a retry is idempotent.
public enum Interaction: Encodable, Equatable, Sendable {
    case decision(blockId: String, verdict: Verdict, note: String? = nil)
    case feedback(id: String, blockId: String, text: String)
    case choice(blockId: String, optionIds: [String], other: String? = nil)
    case input(blockId: String, text: String)
    case annotation(id: String, blockId: String, anchor: String, text: String, quote: String)
    case annotationRemoved(id: String, blockId: String)
    case triage(blockId: String, verdicts: [String: TriageVerdict])
    case submit(revision: Int)

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
        case type, blockId, verdict, note, id, text, optionIds, other, anchor, quote, verdicts, revision
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
        case let .submit(revision):
            try container.encode(revision, forKey: .revision)
        }
    }
}

/// SessionSummary is one row of GET /api/sessions: the artifact's subject, its
/// URL slug, title, open/closed status, last-touched timestamp (RFC 3339), and
/// current revision. The endpoint is added in a parallel workstream; this mirrors
/// its documented response shape.
public struct SessionSummary: Decodable, Equatable, Sendable, Identifiable {
    public let subject: String
    public let slug: String
    public let title: String
    public let status: String
    public let updatedAt: String
    public let revision: Int

    public var id: String {
        subject
    }

    public init(subject: String, slug: String, title: String, status: String, updatedAt: String, revision: Int) {
        self.subject = subject
        self.slug = slug
        self.title = title
        self.status = status
        self.updatedAt = updatedAt
        self.revision = revision
    }
}

/// PacksResponse is the slice of GET /api/packs the client models: each registered
/// pack's block types with their declared `interactive` flag. It drops the fields
/// the native client never reads (bundle, styles, schemas, dropped) so decoding is
/// forgiving of the wider contract shape (docs/contract.md — PacksResponse).
public struct PacksResponse: Decodable, Equatable, Sendable {
    /// Pack is one registered block pack: only its block-type declarations are modeled.
    public struct Pack: Decodable, Equatable, Sendable {
        /// BlockType is one block a pack contributes, tagged with its interactivity.
        public struct BlockType: Decodable, Equatable, Sendable {
            public let type: String
            public let interactive: Bool

            public init(type: String, interactive: Bool) {
                self.type = type
                self.interactive = interactive
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
}

/// InteractionPoster is the one call BoardStore needs from the network: submit a
/// human interaction and return the seq the daemon assigned it. APIClient is the
/// production conformer; tests inject a scripted fake.
public protocol InteractionPoster: Sendable {
    func postInteraction(subject: String, interaction: Interaction) async throws -> Int64
}

/// APIError is a non-success REST response the caller can branch on.
public enum APIError: Error, Equatable {
    case nonHTTPResponse
    case status(code: Int, body: String)
}

/// APIClient talks to a cc-present daemon over its REST plane. It is a value type
/// with no mutable state, so it is freely shared across tasks.
public struct APIClient: InteractionPoster {
    public let baseURL: URL
    public let bearerToken: String?
    private let urlSession: URLSession

    /// Creates a client for the daemon at `baseURL` (e.g. `http://127.0.0.1:8765`).
    /// A `bearerToken`, when present, rides on the Authorization header of every
    /// request, including the raw asset request AsyncImage cannot decorate itself.
    public init(baseURL: URL, bearerToken: String? = nil, urlSession: URLSession = .shared) {
        self.baseURL = baseURL
        self.bearerToken = bearerToken
        self.urlSession = urlSession
    }

    /// postInteraction POSTs one interaction to /api/interactions as
    /// `{subject, nonce, interaction}` with a fresh nonce, and returns the seq the
    /// daemon assigned the appended event. It throws APIError for a non-2xx reply,
    /// so a 409 close or a 400 validation failure surfaces to the caller.
    public func postInteraction(subject: String, interaction: Interaction) async throws -> Int64 {
        var request = URLRequest(url: baseURL.appending(path: "api/interactions"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        authorize(&request)
        let body = InteractionRequest(subject: subject, nonce: UUID().uuidString, interaction: interaction)
        request.httpBody = try JSONEncoder().encode(body)

        let (data, response) = try await urlSession.data(for: request)
        try APIClient.check(response, data: data)
        return try JSONDecoder().decode(SeqResponse.self, from: data).seq
    }

    /// sessions GETs /api/sessions, the daemon's roster of live artifacts.
    public func sessions() async throws -> [SessionSummary] {
        var request = URLRequest(url: baseURL.appending(path: "api/sessions"))
        request.httpMethod = "GET"
        authorize(&request)

        let (data, response) = try await urlSession.data(for: request)
        try APIClient.check(response, data: data)
        return try JSONDecoder().decode([SessionSummary].self, from: data)
    }

    /// packs GETs /api/packs, the daemon's pack manifest, so the focus deck can
    /// classify which pack block types are interactive instead of assuming all are.
    public func packs() async throws -> PacksResponse {
        var request = URLRequest(url: baseURL.appending(path: "api/packs"))
        request.httpMethod = "GET"
        authorize(&request)

        let (data, response) = try await urlSession.data(for: request)
        try APIClient.check(response, data: data)
        return try JSONDecoder().decode(PacksResponse.self, from: data)
    }

    /// assetRequest builds an authorized GET for /assets/<sha>. SwiftUI's
    /// AsyncImage cannot attach headers, so the app hands it this pre-decorated
    /// request instead of a bare URL.
    public func assetRequest(sha: String) -> URLRequest {
        var request = URLRequest(url: baseURL.appending(path: "assets/\(sha)"))
        request.httpMethod = "GET"
        authorize(&request)
        return request
    }

    private func authorize(_ request: inout URLRequest) {
        if let bearerToken {
            request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        }
    }

    private static func check(_ response: URLResponse, data: Data) throws {
        guard let http = response as? HTTPURLResponse else {
            throw APIError.nonHTTPResponse
        }
        guard (200 ..< 300).contains(http.statusCode) else {
            throw APIError.status(code: http.statusCode, body: String(decoding: data, as: UTF8.self))
        }
    }
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
