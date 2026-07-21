// The append-only event log the client reduces into document plus interaction
// state. Each Event mirrors the Go state.Event envelope ({ origin, type, seq,
// payload }) that the fixtures in internal/state/testdata/*.json use, and the
// per-type payloads mirror web/src/events.ts. The reducer discriminates on
// `type`; `payload` decodes the matching payload struct on demand.

import Foundation

/// Origin is who appended an event. Browser SSE frames carry no origin, so it is
/// optional on Event.
public enum Origin: String, Codable, Sendable {
    case agent
    case human
    case system
}

/// Verdict is a human decision on a block. `cleared` removes a prior decision.
public enum Verdict: String, Codable, Sendable {
    case approved
    case rejected
    case cleared
}

// --- Agent-origin payloads ---

/// DocReplacedPayload replaces the whole document. Revision is transport metadata;
/// retained ids keep their prior round stamp instead of the current-round restamp.
public struct DocReplacedPayload: Decodable, Equatable, Sendable {
    public var doc: Doc
    public var revision: Int
    public var retained: [String]?
}

/// BlockUpsertedPayload upserts a block, inserting after `after` (or appending).
public struct BlockUpsertedPayload: Decodable, Equatable, Sendable {
    public var block: Block
    public var after: String?
}

/// BlockRemovedPayload removes the block with `id`, top-level or card child.
public struct BlockRemovedPayload: Decodable, Equatable, Sendable {
    public var id: String
}

/// ReplyCreatedPayload appends to a block's agent reply thread.
public struct ReplyCreatedPayload: Decodable, Equatable, Sendable {
    public var id: String
    public var blockId: String
    public var md: String
}

/// RoundStartedPayload titles the current round.
public struct RoundStartedPayload: Decodable, Equatable, Sendable {
    public var title: String?
}

// --- System-origin payloads ---

/// PresentClosedPayload closes the presentation, terminal for the reduction.
public struct PresentClosedPayload: Decodable, Equatable, Sendable {
    public var summary: String?
}

/// ChannelChangedPayload is the cc-interact presence frame. The reducer skips it.
public struct ChannelChangedPayload: Decodable, Equatable, Sendable {
    public var connected: Bool
}

// --- Human-origin payloads ---

/// DecisionCreatedPayload records a last-write-wins verdict on a block.
public struct DecisionCreatedPayload: Decodable, Equatable, Sendable {
    public var blockId: String
    public var verdict: Verdict
    public var note: String?
}

/// ChoiceSelectedPayload records a last-write-wins option selection. `other` is a
/// free-text write-in outside the authored option set.
public struct ChoiceSelectedPayload: Decodable, Equatable, Sendable {
    public var blockId: String
    public var optionIds: [String]
    public var other: String?
}

/// FeedbackCreatedPayload appends to a block's feedback list.
public struct FeedbackCreatedPayload: Decodable, Equatable, Sendable {
    public var id: String
    public var blockId: String
    public var text: String
}

/// InputSubmittedPayload records a last-write-wins text entry on an input block.
public struct InputSubmittedPayload: Decodable, Equatable, Sendable {
    public var blockId: String
    public var text: String
}

/// AnnotationCreatedPayload upserts one anchored mark on a draft block, keyed by id.
/// Anchor and quote are server-normalized on echo, so a client-sent value is
/// advisory and never assumed to round-trip.
public struct AnnotationCreatedPayload: Decodable, Equatable, Sendable {
    public var id: String
    public var blockId: String
    public var anchor: String
    public var text: String
    public var quote: String
}

/// AnnotationRemovedPayload splices one mark off a draft block by id.
public struct AnnotationRemovedPayload: Decodable, Equatable, Sendable {
    public var id: String
    public var blockId: String
}

/// TriageVerdict is one item's decision inside a triage merge: a verdict string
/// (approved, rejected, or cleared) with an optional note. It is the wire shape
/// shared by the outgoing interaction and the echoed payload; an empty note is
/// omitted on the wire (the daemon's omitempty).
public struct TriageVerdict: Codable, Equatable, Sendable {
    public var verdict: String
    public var note: String?

    public init(verdict: String, note: String? = nil) {
        self.verdict = verdict
        self.note = note
    }

    private enum CodingKeys: String, CodingKey {
        case verdict, note
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        verdict = try container.decode(String.self, forKey: .verdict)
        note = try container.decodeIfPresent(String.self, forKey: .note)
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(verdict, forKey: .verdict)
        if let note, !note.isEmpty {
            try container.encode(note, forKey: .note)
        }
    }
}

/// TriageDecidedPayload is one partial-map merge over a triage block's items, each
/// item last-write-wins; a `cleared` verdict deletes that item's entry.
public struct TriageDecidedPayload: Decodable, Equatable, Sendable {
    public var blockId: String
    public var verdicts: [String: TriageVerdict]
}

/// PackInteractionPayload records a last-write-wins interaction on a pack block.
/// `payload` is the pack-declared JSON body, held verbatim so a client that does
/// not model the pack still round-trips it.
public struct PackInteractionPayload: Decodable, Equatable, Sendable {
    public var blockId: String
    public var payload: JSONValue
}

/// SubmitPayload records a human submit with the submitted revision.
public struct SubmitPayload: Decodable, Equatable, Sendable {
    public var revision: Int
}

/// RevisingChangedPayload replaces the agent's declared revising working set.
public struct RevisingChangedPayload: Decodable, Equatable, Sendable {
    public var blockIds: [String]
    public var note: String?
}

/// EventPayload is an event's decoded, type-specific payload. `channelChanged` is
/// the presence frame the reducer skips; every other case folds into state.
public enum EventPayload: Equatable, Sendable {
    case docReplaced(DocReplacedPayload)
    case blockUpserted(BlockUpsertedPayload)
    case blockRemoved(BlockRemovedPayload)
    case replyCreated(ReplyCreatedPayload)
    case roundStarted(RoundStartedPayload)
    case presentClosed(PresentClosedPayload)
    case decisionCreated(DecisionCreatedPayload)
    case choiceSelected(ChoiceSelectedPayload)
    case feedbackCreated(FeedbackCreatedPayload)
    case inputSubmitted(InputSubmittedPayload)
    case annotationCreated(AnnotationCreatedPayload)
    case annotationRemoved(AnnotationRemovedPayload)
    case triageDecided(TriageDecidedPayload)
    case packInteraction(PackInteractionPayload)
    case submit(SubmitPayload)
    case revisingChanged(RevisingChangedPayload)
    case channelChanged(ChannelChangedPayload)
}

/// EventError is a failure decoding an event or its payload.
public enum EventError: Error, Equatable {
    case unknownType(String)
    case malformedWireFrame
}

/// Event is one entry in a subject's log. Type is the reduction discriminant; the
/// typed `payload` is decoded on demand. Decoding an Event follows the fixture
/// envelope shape; `wireFrame(_:seq:)` lifts a flat SSE `data:` frame into the
/// same shape.
public struct Event: Decodable, Equatable, Sendable {
    public var origin: Origin?
    public var type: String
    public var seq: Int64?

    private let rawPayload: JSONValue

    /// payload decodes this event's type-specific payload. It throws
    /// `EventError.unknownType` for a type outside the known taxonomy, so a
    /// reducer that short-circuits on close never sees a post-close unknown.
    public var payload: EventPayload {
        get throws {
            switch type {
            case "doc.replaced": return try .docReplaced(rawPayload.decode(DocReplacedPayload.self))
            case "block.upserted": return try .blockUpserted(rawPayload.decode(BlockUpsertedPayload.self))
            case "block.removed": return try .blockRemoved(rawPayload.decode(BlockRemovedPayload.self))
            case "reply.created": return try .replyCreated(rawPayload.decode(ReplyCreatedPayload.self))
            case "round.started": return try .roundStarted(rawPayload.decode(RoundStartedPayload.self))
            case "present.closed": return try .presentClosed(rawPayload.decode(PresentClosedPayload.self))
            case "decision.created": return try .decisionCreated(rawPayload.decode(DecisionCreatedPayload.self))
            case "choice.selected": return try .choiceSelected(rawPayload.decode(ChoiceSelectedPayload.self))
            case "feedback.created": return try .feedbackCreated(rawPayload.decode(FeedbackCreatedPayload.self))
            case "input.submitted": return try .inputSubmitted(rawPayload.decode(InputSubmittedPayload.self))
            case "annotation.created": return try .annotationCreated(rawPayload.decode(AnnotationCreatedPayload.self))
            case "annotation.removed": return try .annotationRemoved(rawPayload.decode(AnnotationRemovedPayload.self))
            case "triage.decided": return try .triageDecided(rawPayload.decode(TriageDecidedPayload.self))
            case "pack.interaction": return try .packInteraction(rawPayload.decode(PackInteractionPayload.self))
            case "submit": return try .submit(rawPayload.decode(SubmitPayload.self))
            case "revising.changed": return try .revisingChanged(rawPayload.decode(RevisingChangedPayload.self))
            case "channel.changed": return try .channelChanged(rawPayload.decode(ChannelChangedPayload.self))
            default: throw EventError.unknownType(type)
            }
        }
    }

    private init(origin: Origin?, type: String, seq: Int64?, rawPayload: JSONValue) {
        self.origin = origin
        self.type = type
        self.seq = seq
        self.rawPayload = rawPayload
    }

    private enum CodingKeys: String, CodingKey {
        case origin
        case type
        case seq
        case payload
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        origin = try container.decodeIfPresent(Origin.self, forKey: .origin)
        type = try container.decode(String.self, forKey: .type)
        seq = try container.decodeIfPresent(Int64.self, forKey: .seq)
        rawPayload = try container.decode(JSONValue.self, forKey: .payload)
    }

    /// wireFrame lifts a flat SSE `data:` frame into an Event. The frame carries
    /// no origin and its self-describing `type` sits alongside the payload fields;
    /// seq rides on the SSE `id:` line and is passed in.
    public static func wireFrame(_ data: Data, seq: Int64? = nil) throws -> Event {
        let raw = try JSONDecoder().decode(JSONValue.self, from: data)
        guard case let .object(fields) = raw, case let .string(type)? = fields["type"] else {
            throw EventError.malformedWireFrame
        }
        return Event(origin: nil, type: type, seq: seq, rawPayload: raw)
    }
}

/// JSONValue is a decoded arbitrary JSON value, used to hold an event payload
/// until it is decoded into its typed struct. It re-encodes losslessly, keeping
/// integers integral so a revision or seq survives the round to a typed decode.
public enum JSONValue: Codable, Equatable, Sendable {
    case null
    case bool(Bool)
    case int(Int64)
    case double(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() {
            self = .null
        } else if let value = try? container.decode(Bool.self) {
            self = .bool(value)
        } else if let value = try? container.decode(Int64.self) {
            self = .int(value)
        } else if let value = try? container.decode(Double.self) {
            self = .double(value)
        } else if let value = try? container.decode(String.self) {
            self = .string(value)
        } else if let value = try? container.decode([JSONValue].self) {
            self = .array(value)
        } else if let value = try? container.decode([String: JSONValue].self) {
            self = .object(value)
        } else {
            throw DecodingError.dataCorrupted(
                DecodingError.Context(codingPath: decoder.codingPath, debugDescription: "unrepresentable JSON value")
            )
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null: try container.encodeNil()
        case let .bool(value): try container.encode(value)
        case let .int(value): try container.encode(value)
        case let .double(value): try container.encode(value)
        case let .string(value): try container.encode(value)
        case let .array(value): try container.encode(value)
        case let .object(value): try container.encode(value)
        }
    }

    func decode<T: Decodable>(_: T.Type) throws -> T {
        let data = try JSONEncoder().encode(self)
        return try JSONDecoder().decode(T.self, from: data)
    }
}
