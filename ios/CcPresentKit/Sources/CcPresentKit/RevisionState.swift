// The client-local seen store for the live-revision loop. Per-launch, never
// persisted, fed only past the caught-up boundary.

import Foundation
import Observation

/// RevisionState is the per-launch seen store: which top-level blocks changed live
/// and are not yet viewed, plus the agent's current revising working set. The focus
/// deck reads it for the per-step warn banner, the arrival callout, and the rail dot
/// states. It is client-local and never enters the reduction.
@MainActor
@Observable
public final class RevisionState {
    /// ChangeKind distinguishes a live rewrite of an existing step from a fresh step
    /// that joined the document.
    public enum ChangeKind: Equatable, Sendable {
        case revised
        case added
    }

    /// Mark is one unseen live change on a top-level block: its kind, the revising
    /// note lifted from the pre-clear state (nil when unannounced), and the
    /// frame-arrival time.
    public struct Mark: Equatable, Sendable {
        public let kind: ChangeKind
        public let note: String?
        public let at: Date

        public init(kind: ChangeKind, note: String?, at: Date) {
            self.kind = kind
            self.note = note
            self.at = at
        }
    }

    /// marks holds each top-level block's unseen live change, keyed by block id. A
    /// mark is dropped when its step is viewed (`markSeen`) or its block is removed.
    public private(set) var marks: [String: Mark] = [:]

    /// revising mirrors the agent's declared working set — the source for the warn
    /// banner and the pulsing rail dots — tracking the latest set on every frame so a
    /// tab opened mid-rewrite still shows the banner.
    public private(set) var revising: Revising = .init()

    /// revisingSince is when the current non-empty revising set was first announced —
    /// the decay clock's start. nil when the set is empty.
    public private(set) var revisingSince: Date?

    /// revisingDecayed is true once the current revising set has outlived the decay
    /// window without resolving (the agent likely died): the banner turns passive.
    public private(set) var revisingDecayed: Bool = false

    @ObservationIgnored private let decayInterval: TimeInterval
    @ObservationIgnored private(set) var decayTask: Task<Void, Never>?

    /// Creates a seen store. `decayInterval` is the orphaned-revising decay window,
    /// 120s in production; tests shorten it.
    public init(decayInterval: TimeInterval = 120) {
        self.decayInterval = decayInterval
    }

    deinit {
        decayTask?.cancel()
    }

    /// ingest folds one frame's before/after reductions into the store. `prev` and
    /// `next` are the reductions immediately around a single SSE frame. Marks are
    /// recorded only past the caught-up boundary (`isLoading` false) so replayed
    /// history never badges; the revising banner tracks `next.revising` every frame.
    public func ingest(prev: BoardState, next: BoardState, isLoading: Bool, now: Date = Date()) {
        if !isLoading {
            recordChanges(prev: prev, next: next, now: now)
        }
        syncRevising(next.revising, now: now)
    }

    /// markSeen drops a block's mark once its step has been viewed — the deck calls it
    /// for the step it leaves, so a callout shows while its step is current and clears
    /// when the human moves on.
    public func markSeen(_ id: String) {
        marks.removeValue(forKey: id)
    }

    /// mark returns a block's unseen live change, or nil when it has none.
    public func mark(for id: String) -> Mark? {
        marks[id]
    }

    /// isRevising reports whether the agent's declared working set names this block.
    public func isRevising(_ id: String) -> Bool {
        revising.blockIds.contains(id)
    }

    /// revisingNote is the shared working-set note surfaced beside a revising step's
    /// banner, or nil when the block is not in the set.
    public func revisingNote(for id: String) -> String? {
        isRevising(id) ? revising.note : nil
    }

    /// docDraftingNote is the doc-level "working" note — a non-empty note with an empty
    /// block set, an announcement for work with no existing block to mark yet.
    public var docDraftingNote: String? {
        revising.blockIds.isEmpty ? revising.note : nil
    }

    private func recordChanges(prev: BoardState, next: BoardState, now: Date) {
        let prevById = Dictionary(prev.doc.blocks.map { ($0.id, $0) }, uniquingKeysWith: { first, _ in first })
        let nextIds = Set(next.doc.blocks.map(\.id))
        marks = marks.filter { nextIds.contains($0.key) }
        for block in next.doc.blocks {
            let id = block.id
            if let old = prevById[id] {
                guard old != block else { continue }
                // Same-id rewrite: `prev` is the last state still carrying the
                // announcement the reducer clears this frame, so lift the note here.
                let note = prev.revising.blockIds.contains(id) ? prev.revising.note : nil
                marks[id] = Mark(kind: .revised, note: note, at: now)
            } else {
                // Fresh id: the doc-level drafting note covers the announcement gap.
                let note = prev.revising.blockIds.isEmpty ? prev.revising.note : nil
                marks[id] = Mark(kind: .added, note: note, at: now)
            }
        }
    }

    private func syncRevising(_ newRevising: Revising, now: Date) {
        guard newRevising != revising else { return }
        revising = newRevising
        let active = !newRevising.blockIds.isEmpty || newRevising.note != nil
        revisingDecayed = false
        if active {
            revisingSince = now
            scheduleDecay()
        } else {
            revisingSince = nil
            decayTask?.cancel()
            decayTask = nil
        }
    }

    private func scheduleDecay() {
        decayTask?.cancel()
        let interval = decayInterval
        decayTask = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .seconds(interval))
            guard let self, !Task.isCancelled, revisingSince != nil else { return }
            revisingDecayed = true
        }
    }
}
