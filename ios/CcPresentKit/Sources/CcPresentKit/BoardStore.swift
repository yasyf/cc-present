// The optimistic engine: a main-actor, observable store whose visible state is a
// pure reduction of the server log with a thin pending overlay laid on top. A user
// intent applies immediately (append a pending interaction, recompute), then POSTs;
// the SSE echo of that interaction drops the pending item and folds the confirmed
// event into the server log. Because the overlay is dropped the instant its echo
// lands, even append-only interactions (feedback) and round-advancing ones (submit)
// are safe to apply optimistically — unlike web/src/reduce.ts, which defers them
// because its react-query cache has no drop-on-echo step. See web/src/present.ts
// and web/src/api.ts for the browser's behavioral reference.

import Foundation
import Observation

/// BoardStore holds one subject's live board. It reduces the SSE-delivered server
/// log plus an optimistic overlay of not-yet-confirmed interactions into a single
/// BoardState the UI renders. Loading, connection, and closed signals drive the
/// chrome; `send` (and its typed convenience methods) submit interactions.
@MainActor
@Observable
public final class BoardStore {
    /// state is the reduction the UI renders: the server log with the pending
    /// overlay applied on top.
    public private(set) var state: BoardState

    /// isLoading is true until the SSE `caught-up` boundary, i.e. while the initial
    /// log replay is still streaming.
    public private(set) var isLoading: Bool = true

    /// connectionState mirrors the transport for the UI's connectivity chrome.
    public private(set) var connectionState: SSEClient.ConnectionState = .connecting

    /// isClosed is true once a present.closed frame lands; the board is then
    /// read-only and `send` is a no-op.
    public var isClosed: Bool {
        state.interactions.closed.value
    }

    /// revisions is the client-local seen store, fed one frame at a time from the SSE
    /// event-application path (not per SwiftUI render), so live-change marks and the
    /// revising banner survive view identity and board/focus mode switches. The focus
    /// deck reads it read-only.
    public let revisions = RevisionState()

    public let subject: String
    @ObservationIgnored private let transport: any InteractionPoster
    @ObservationIgnored private var serverLog: [Event] = []
    @ObservationIgnored private var pending: [PendingInteraction] = []
    @ObservationIgnored private var messageTask: Task<Void, Never>?
    @ObservationIgnored private var stateTask: Task<Void, Never>?

    /// Creates a store for `subject`, posting interactions through `transport`.
    /// Call `connect` to attach an SSEClient's streams.
    public init(subject: String, transport: any InteractionPoster) {
        self.subject = subject
        self.transport = transport
        state = .initial
    }

    deinit {
        messageTask?.cancel()
        stateTask?.cancel()
    }

    // MARK: - Transport wiring

    /// connect consumes an SSEClient connection: message frames drive the reduction
    /// and the `caught-up` boundary flips loading; state frames drive the
    /// connectivity chrome. Calling it again replaces a prior connection.
    public func connect(_ connection: SSEClient.Connection) {
        disconnect()
        messageTask = Task { [weak self] in
            for await message in connection.messages {
                guard let self else { break }
                ingest(message)
            }
        }
        stateTask = Task { [weak self] in
            for await state in connection.states {
                guard let self else { break }
                connectionState = state
            }
        }
    }

    /// disconnect tears down the consumers attached by `connect`, which terminates
    /// the underlying SSE stream.
    public func disconnect() {
        messageTask?.cancel()
        stateTask?.cancel()
        messageTask = nil
        stateTask = nil
    }

    // MARK: - User intent

    /// send applies `interaction` optimistically and POSTs it. It returns
    /// immediately with the in-flight POST task (already complete on a closed
    /// board), which callers may await for confirmation. On a `{seq}` reply the
    /// pending item is tagged with that seq; on failure it is rolled back.
    @discardableResult
    public func send(_ interaction: Interaction) -> Task<Void, Never> {
        guard !isClosed else { return Task {} }
        guard let event = try? Event.wireFrame(JSONEncoder().encode(interaction)) else {
            return Task {}
        }
        let localID = UUID()
        pending.append(PendingInteraction(localID: localID, interaction: interaction, event: event))
        recompute()

        return Task { [self] in
            do {
                let seq = try await transport.postInteraction(subject: subject, interaction: interaction)
                tag(localID: localID, seq: seq)
            } catch {
                rollback(localID: localID)
            }
        }
    }

    /// decide submits an approve/reject/clear verdict on a block.
    @discardableResult
    public func decide(blockId: String, verdict: Verdict) -> Task<Void, Never> {
        send(.decision(blockId: blockId, verdict: verdict))
    }

    /// choose submits the full next selection for a choice block. `other` is a
    /// free-text write-in outside the authored option set; single-select replaces an
    /// authored pick with the write-in (empty `optionIds`), multi-select carries both.
    @discardableResult
    public func choose(blockId: String, optionIds: [String], other: String? = nil) -> Task<Void, Never> {
        send(.choice(blockId: blockId, optionIds: optionIds, other: other))
    }

    /// feedback appends free-text feedback to a block, minting the idempotent id.
    @discardableResult
    public func feedback(blockId: String, text: String) -> Task<Void, Never> {
        send(.feedback(id: UUID().uuidString, blockId: blockId, text: text))
    }

    /// submitInput commits an input block's text.
    @discardableResult
    public func submitInput(blockId: String, text: String) -> Task<Void, Never> {
        send(.input(blockId: blockId, text: text))
    }

    /// submit records a human submit at `revision`.
    @discardableResult
    public func submit(revision: Int) -> Task<Void, Never> {
        send(.submit(revision: revision))
    }

    // MARK: - Reconciliation

    /// ingest folds one SSE message into the store: `caught-up` clears loading; a
    /// frame appends to the log, drops its echo, recomputes, and feeds `revisions`.
    func ingest(_ message: SSEClient.Message) {
        switch message {
        case .caughtUp:
            isLoading = false
        case let .frame(event):
            serverLog.append(event)
            dropEcho(of: event)
            let prev = state
            recompute()
            var docReplaced = false
            if case .docReplaced? = try? event.payload {
                docReplaced = true
            }
            revisions.ingest(prev: prev, next: state, isLoading: isLoading, docReplaced: docReplaced)
        }
    }

    /// pendingCount is the number of unconfirmed optimistic interactions.
    var pendingCount: Int {
        pending.count
    }

    private func tag(localID: UUID, seq: Int64) {
        guard let index = pending.firstIndex(where: { $0.localID == localID }) else { return }
        if serverLog.contains(where: { $0.seq == seq }) {
            // The echo raced ahead of this reply and its identity match missed;
            // the confirmed event is already in the log, so drop the overlay.
            pending.remove(at: index)
        } else {
            pending[index].assignedSeq = seq
        }
        recompute()
    }

    private func rollback(localID: UUID) {
        guard let index = pending.firstIndex(where: { $0.localID == localID }) else { return }
        pending.remove(at: index)
        recompute()
    }

    /// dropEcho removes the pending item a confirmed frame echoes: by seq once the
    /// POST reply has tagged it, else by interaction identity for the race where
    /// the frame outruns the reply.
    private func dropEcho(of event: Event) {
        if let seq = event.seq, let index = pending.firstIndex(where: { $0.assignedSeq == seq }) {
            pending.remove(at: index)
            return
        }
        if let index = pending.firstIndex(where: { $0.echoes(event) }) {
            pending.remove(at: index)
        }
    }

    /// recompute rebuilds state from the server log plus the pending overlay. Each
    /// pending event is stamped with a seq past every server event so the reduction
    /// orders it last — optimistic actions always sit atop confirmed state. A
    /// malformed frame that fails the reduction leaves the prior state intact,
    /// matching SSEClient's drop-on-malformed handling of hostile wire input.
    private func recompute() {
        var events = serverLog
        let base = serverLog.reduce(Int64(0)) { max($0, $1.seq ?? 0) }
        for (offset, item) in pending.enumerated() {
            var event = item.event
            event.seq = base + Int64(offset) + 1
            events.append(event)
        }
        if let next = try? reduce(events: events) {
            state = next
        }
    }
}

/// PendingInteraction is one optimistic overlay entry: the interaction, the event
/// built from it for the reduction, and the seq assigned by the POST reply (nil
/// until it arrives). `localID` is a store-local handle, distinct from the wire
/// dedup nonce, used to reconcile the reply with the overlay entry.
private struct PendingInteraction {
    let localID: UUID
    let interaction: Interaction
    let event: Event
    var assignedSeq: Int64?

    /// echoes reports whether `event` is the server echo of this interaction,
    /// matching type plus the identifying payload fields. Feedback matches on its
    /// minted id; the last-write-wins interactions match on their content, and two
    /// identical pending items are interchangeable under that match.
    func echoes(_ event: Event) -> Bool {
        guard event.type == interaction.type, let payload = try? event.payload else { return false }
        switch (interaction, payload) {
        case let (.decision(blockId, verdict, note), .decisionCreated(echoed)):
            return echoed.blockId == blockId && echoed.verdict == verdict && echoed.note == note
        case let (.feedback(id, blockId, text), .feedbackCreated(echoed)):
            return echoed.id == id && echoed.blockId == blockId && echoed.text == text
        case let (.choice(blockId, optionIds, other), .choiceSelected(echoed)):
            return echoed.blockId == blockId && echoed.optionIds == optionIds && echoed.other == other
        case let (.input(blockId, text), .inputSubmitted(echoed)):
            return echoed.blockId == blockId && echoed.text == text
        case let (.submit(revision), .submit(echoed)):
            return echoed.revision == revision
        default:
            return false
        }
    }
}
