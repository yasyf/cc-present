// The pure reduction of an append-only event log into the document plus the
// human interaction state, ported field-for-field from the Go reducer in
// internal/state/reduce.go. Doc content and human verdicts are held separately
// and keyed by block id, so an agent re-upserting a block never clobbers a
// human's decision. The reduction is pure: replaying the log from seq 0
// reconstructs a fresh tab's state.

import Foundation

/// Decision is a human's last-write-wins verdict on a block, with an optional note.
public struct Decision: Decodable, Equatable, Sendable {
    public var verdict: String
    public var note: String?

    public init(verdict: String, note: String? = nil) {
        self.verdict = verdict
        self.note = note
    }
}

/// Selection is a human's last-write-wins option selection on a choice block.
public struct Selection: Decodable, Equatable, Sendable {
    public var optionIds: [String]

    public init(optionIds: [String]) {
        self.optionIds = optionIds
    }
}

/// InputValue is a human's last-write-wins text entry on an input block. Round is
/// the round its enclosing top-level block was in when the entry was committed,
/// stamped by the reducer.
public struct InputValue: Decodable, Equatable, Sendable {
    public var text: String
    public var round: Int

    public init(text: String, round: Int) {
        self.text = text
        self.round = round
    }
}

/// PackValue is a human's last-write-wins interaction on a pack block: the payload
/// exactly as the REST edge validated it. The reducer stays pack-blind — it never
/// inspects a pack payload's shape.
public struct PackValue: Codable, Equatable, Sendable {
    public var payload: JSONValue

    public init(payload: JSONValue) {
        self.payload = payload
    }
}

/// Feedback is one entry in a block's append-only feedback list.
public struct Feedback: Decodable, Equatable, Sendable {
    public var id: String
    public var text: String

    public init(id: String, text: String) {
        self.id = id
        self.text = text
    }
}

/// Reply is one entry in a block's append-only agent reply thread.
public struct Reply: Decodable, Equatable, Sendable {
    public var id: String
    public var md: String

    public init(id: String, md: String) {
        self.id = id
        self.md = md
    }
}

/// Submitted records whether a human has submitted and the last revision submitted.
public struct Submitted: Decodable, Equatable, Sendable {
    public var value: Bool
    public var revision: Int

    public init(value: Bool, revision: Int) {
        self.value = value
        self.revision = revision
    }
}

/// Closed records whether the agent has closed the presentation and its summary.
public struct Closed: Decodable, Equatable, Sendable {
    public var value: Bool
    public var summary: String?

    public init(value: Bool, summary: String? = nil) {
        self.value = value
        self.summary = summary
    }
}

/// Interactions holds every human interaction, keyed by block id, plus the submit
/// and close signals. Decisions, choices, and inputs are last-write-wins;
/// feedback and replies are append-only.
public struct Interactions: Decodable, Equatable, Sendable {
    public var decisions: [String: Decision]
    public var choices: [String: Selection]
    public var inputs: [String: InputValue]
    public var packs: [String: PackValue]
    public var feedback: [String: [Feedback]]
    public var replies: [String: [Reply]]
    public var submitted: Submitted
    public var closed: Closed

    public init(
        decisions: [String: Decision] = [:],
        choices: [String: Selection] = [:],
        inputs: [String: InputValue] = [:],
        packs: [String: PackValue] = [:],
        feedback: [String: [Feedback]] = [:],
        replies: [String: [Reply]] = [:],
        submitted: Submitted = Submitted(value: false, revision: 0),
        closed: Closed = Closed(value: false, summary: nil)
    ) {
        self.decisions = decisions
        self.choices = choices
        self.inputs = inputs
        self.packs = packs
        self.feedback = feedback
        self.replies = replies
        self.submitted = submitted
        self.closed = closed
    }

    private enum CodingKeys: String, CodingKey {
        case decisions, choices, inputs, packs, feedback, replies, submitted, closed
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        decisions = try container.decodeIfPresent([String: Decision].self, forKey: .decisions) ?? [:]
        choices = try container.decodeIfPresent([String: Selection].self, forKey: .choices) ?? [:]
        inputs = try container.decodeIfPresent([String: InputValue].self, forKey: .inputs) ?? [:]
        packs = try container.decodeIfPresent([String: PackValue].self, forKey: .packs) ?? [:]
        feedback = try container.decodeIfPresent([String: [Feedback]].self, forKey: .feedback) ?? [:]
        replies = try container.decodeIfPresent([String: [Reply]].self, forKey: .replies) ?? [:]
        submitted = try container.decodeIfPresent(Submitted.self, forKey: .submitted)
            ?? Submitted(value: false, revision: 0)
        closed = try container.decodeIfPresent(Closed.self, forKey: .closed) ?? Closed(value: false, summary: nil)
    }
}

/// RoundRecord is a closed round: the top-level blocks live at close (frozen
/// copies) plus the interaction values snapshotted to those blocks' ids.
/// SubmittedRevision is set only when the round closed on a submit.
public struct RoundRecord: Decodable, Equatable, Sendable {
    public var number: Int
    public var title: String
    public var blocks: [Block]
    public var decisions: [String: Decision]
    public var choices: [String: Selection]
    public var inputs: [String: InputValue]
    public var packs: [String: PackValue]
    public var feedback: [String: [Feedback]]
    public var submittedRevision: Int?

    public init(
        number: Int,
        title: String = "",
        blocks: [Block],
        decisions: [String: Decision] = [:],
        choices: [String: Selection] = [:],
        inputs: [String: InputValue] = [:],
        packs: [String: PackValue] = [:],
        feedback: [String: [Feedback]] = [:],
        submittedRevision: Int? = nil
    ) {
        self.number = number
        self.title = title
        self.blocks = blocks
        self.decisions = decisions
        self.choices = choices
        self.inputs = inputs
        self.packs = packs
        self.feedback = feedback
        self.submittedRevision = submittedRevision
    }

    private enum CodingKeys: String, CodingKey {
        case number, title, blocks, decisions, choices, inputs, packs, feedback, submittedRevision
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        number = try container.decode(Int.self, forKey: .number)
        title = try container.decodeIfPresent(String.self, forKey: .title) ?? ""
        blocks = try container.decodeIfPresent([Block].self, forKey: .blocks) ?? []
        decisions = try container.decodeIfPresent([String: Decision].self, forKey: .decisions) ?? [:]
        choices = try container.decodeIfPresent([String: Selection].self, forKey: .choices) ?? [:]
        inputs = try container.decodeIfPresent([String: InputValue].self, forKey: .inputs) ?? [:]
        packs = try container.decodeIfPresent([String: PackValue].self, forKey: .packs) ?? [:]
        feedback = try container.decodeIfPresent([String: [Feedback]].self, forKey: .feedback) ?? [:]
        submittedRevision = try container.decodeIfPresent(Int.self, forKey: .submittedRevision)
    }
}

/// Rounds tracks the round partition. Current is 1-based; BlockRounds maps a
/// top-level block id to the round of its last agent touch; History holds the
/// closed rounds in ascending order. A round is dirty when a live top-level block
/// carries the current round.
public struct Rounds: Decodable, Equatable, Sendable {
    public var current: Int
    public var currentTitle: String
    public var blockRounds: [String: Int]
    public var history: [RoundRecord]

    public init(
        current: Int = 1,
        currentTitle: String = "",
        blockRounds: [String: Int] = [:],
        history: [RoundRecord] = []
    ) {
        self.current = current
        self.currentTitle = currentTitle
        self.blockRounds = blockRounds
        self.history = history
    }

    private enum CodingKeys: String, CodingKey {
        case current, currentTitle, blockRounds, history
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let cur = try container.decodeIfPresent(Int.self, forKey: .current) ?? 0
        current = cur == 0 ? 1 : cur
        currentTitle = try container.decodeIfPresent(String.self, forKey: .currentTitle) ?? ""
        blockRounds = try container.decodeIfPresent([String: Int].self, forKey: .blockRounds) ?? [:]
        history = try container.decodeIfPresent([RoundRecord].self, forKey: .history) ?? []
    }
}

/// BoardState is the full reduction: the current document, the human
/// interactions, and the round partition. It is the Swift analogue of the Go
/// `state.State`.
public struct BoardState: Decodable, Equatable, Sendable {
    public var doc: Doc
    public var interactions: Interactions
    public var rounds: Rounds

    public init(doc: Doc, interactions: Interactions, rounds: Rounds) {
        self.doc = doc
        self.interactions = interactions
        self.rounds = rounds
    }

    private enum CodingKeys: String, CodingKey {
        case doc, interactions, rounds
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        doc = try container.decodeIfPresent(Doc.self, forKey: .doc) ?? Doc(version: 1, title: "", blocks: [])
        interactions = try container.decodeIfPresent(Interactions.self, forKey: .interactions) ?? Interactions()
        rounds = try container.decodeIfPresent(Rounds.self, forKey: .rounds) ?? Rounds()
    }

    static var initial: BoardState {
        BoardState(doc: Doc(version: 1, title: "", blocks: []), interactions: Interactions(), rounds: Rounds())
    }
}

/// reduce folds the log into a BoardState. Events are processed in ascending seq
/// order; last-write-wins interactions resolve by that order. The document starts
/// empty, so a block.upserted before any doc.replaced appends to it.
/// present.closed is terminal: any event ordered after it is a no-op, so a human
/// interaction that races the close never poisons replay. channel.changed presence
/// frames are skipped regardless of origin; any other unknown event type throws.
public func reduce(events: [Event]) throws -> BoardState {
    var state = BoardState.initial
    let ordered = events.sorted { ($0.seq ?? 0) < ($1.seq ?? 0) }
    for event in ordered {
        if state.interactions.closed.value {
            continue
        }
        try state.apply(event)
    }
    return state
}

private extension BoardState {
    mutating func apply(_ event: Event) throws {
        switch try event.payload {
        case let .docReplaced(payload):
            doc = payload.doc
            rounds.blockRounds = [:]
            for block in doc.blocks {
                rounds.blockRounds[block.id] = rounds.current
            }
        case let .blockUpserted(payload):
            upsert(payload.block, after: payload.after)
            rounds.blockRounds[payload.block.id] = rounds.current
        case let .blockRemoved(payload):
            remove(payload.id)
            rounds.blockRounds.removeValue(forKey: payload.id)
        case let .replyCreated(payload):
            interactions.replies[payload.blockId, default: []].append(Reply(id: payload.id, md: payload.md))
        case let .presentClosed(payload):
            interactions.closed = Closed(value: true, summary: payload.summary)
        case let .decisionCreated(payload):
            if payload.verdict == .cleared {
                interactions.decisions.removeValue(forKey: payload.blockId)
            } else {
                interactions.decisions[payload.blockId] = Decision(
                    verdict: payload.verdict.rawValue,
                    note: payload.note
                )
            }
        case let .choiceSelected(payload):
            interactions.choices[payload.blockId] = Selection(optionIds: payload.optionIds)
        case let .packInteraction(payload):
            interactions.packs[payload.blockId] = PackValue(payload: payload.payload)
        case let .feedbackCreated(payload):
            interactions.feedback[payload.blockId, default: []].append(Feedback(id: payload.id, text: payload.text))
        case let .inputSubmitted(payload):
            interactions.inputs[payload.blockId] = InputValue(text: payload.text, round: inputRound(payload.blockId))
        case let .submit(payload):
            interactions.submitted = Submitted(value: true, revision: payload.revision)
            if dirty() {
                rounds.history.append(closeRound(revision: payload.revision))
                rounds.current += 1
                rounds.currentTitle = ""
            }
        case let .roundStarted(payload):
            if dirty() {
                rounds.history.append(closeRound(revision: nil))
                rounds.current += 1
            }
            rounds.currentTitle = payload.title ?? ""
        case .channelChanged:
            return
        }
    }

    mutating func upsert(_ block: Block, after: String?) {
        let id = block.id
        if let index = doc.blocks.firstIndex(where: { $0.id == id }) {
            doc.blocks[index] = block
            return
        }
        if let after, let index = doc.blocks.firstIndex(where: { $0.id == after }) {
            doc.blocks.insert(block, at: index + 1)
            return
        }
        doc.blocks.append(block)
    }

    mutating func remove(_ id: String) {
        if let index = doc.blocks.firstIndex(where: { $0.id == id }) {
            doc.blocks.remove(at: index)
        }
    }

    func dirty() -> Bool {
        for block in doc.blocks where rounds.blockRounds[block.id] == rounds.current {
            return true
        }
        return false
    }

    func closeRound(revision: Int?) -> RoundRecord {
        let cur = rounds.current
        var live: [Block] = []
        for block in doc.blocks where rounds.blockRounds[block.id] == cur {
            live.append(block)
        }
        let ids = idsOf(live)
        return RoundRecord(
            number: cur,
            title: rounds.currentTitle,
            blocks: live,
            decisions: filterMap(interactions.decisions, ids),
            choices: filterMap(interactions.choices, ids),
            inputs: filterMap(interactions.inputs, ids),
            packs: filterMap(interactions.packs, ids),
            feedback: filterMap(interactions.feedback, ids),
            submittedRevision: revision
        )
    }

    /// inputRound resolves the round an input value belongs to: the round of its
    /// enclosing top-level block (the block itself when top-level, else the card
    /// one level up that contains it), mirroring idsOf's one-level child
    /// resolution. An id with no block in the doc (an orphaned interaction) falls
    /// back to the current round so the reduction stays total.
    func inputRound(_ id: String) -> Int {
        for block in doc.blocks {
            if block.id == id {
                return stampedRound(id)
            }
            if case let .card(card) = block {
                for child in card.children where child.id == id {
                    return stampedRound(block.id)
                }
            }
        }
        return rounds.current
    }

    func stampedRound(_ id: String) -> Int {
        rounds.blockRounds[id] ?? rounds.current
    }
}

/// idsOf collects the ids of a block slice plus one level of card children,
/// mirroring where interactive blocks may nest.
private func idsOf(_ blocks: [Block]) -> Set<String> {
    var ids = Set<String>()
    for block in blocks {
        ids.insert(block.id)
        if case let .card(card) = block {
            for child in card.children {
                ids.insert(child.id)
            }
        }
    }
    return ids
}

private func filterMap<T>(_ map: [String: T], _ ids: Set<String>) -> [String: T] {
    var out: [String: T] = [:]
    for (id, value) in map where ids.contains(id) {
        out[id] = value
    }
    return out
}
