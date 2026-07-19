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

    /// passive names the revising ids whose decay window has elapsed without resolving
    /// (the agent likely died): their banner turns muted. Per-id, so one stale step
    /// never bleeds its passivity onto another. Read it through `revisingPassive`.
    private(set) var passive: Set<String> = []

    @ObservationIgnored private let decayInterval: TimeInterval
    @ObservationIgnored private(set) var decayTasks: [String: Task<Void, Never>] = [:]

    /// Creates a seen store. `decayInterval` is the orphaned-revising decay window,
    /// 120s in production; tests shorten it.
    public init(decayInterval: TimeInterval = 120) {
        self.decayInterval = decayInterval
    }

    deinit {
        for task in decayTasks.values {
            task.cancel()
        }
    }

    /// ingest folds one frame's before/after reductions into the store. A `docReplaced`
    /// frame clears every mark — a wholesale swap is a fresh slate, not a per-block
    /// change; otherwise marks record only past the caught-up boundary (`isLoading`
    /// false) so replayed history never badges. The banner tracks `next.revising`.
    public func ingest(prev: BoardState, next: BoardState, isLoading: Bool, docReplaced: Bool = false, now: Date = Date()) {
        if docReplaced {
            marks = [:]
        } else if !isLoading {
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

    /// revisingPassive reports whether a revising id's own decay window has elapsed, so
    /// its banner reads muted. Per-id: one step's staleness never marks another.
    public func revisingPassive(_ id: String) -> Bool {
        passive.contains(id)
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

    private func syncRevising(_ newRevising: Revising, now _: Date) {
        guard newRevising != revising else { return }
        let prevNote = revising.note
        revising = newRevising
        let nextIds = newRevising.blockIds
        let nextSet = Set(nextIds)

        // Ids that left the set: cancel their window and clear passivity. The union
        // reaches ids whose window already fired (task removed, passive flag set).
        for id in Set(decayTasks.keys).union(passive) where !nextSet.contains(id) {
            decayTasks[id]?.cancel()
            decayTasks[id] = nil
            passive.remove(id)
        }
        // A newly-revising id opens its own window; an id still counting down or
        // already passive keeps its clock, so a sibling joining never restarts it.
        for id in nextIds where decayTasks[id] == nil && !passive.contains(id) {
            armDecay(id)
        }
        // A changed working-set note is a fresh announcement: un-stick any passive id
        // and restart its window. A note-only change before decay leaves the clock be.
        if newRevising.note != prevNote {
            for id in nextIds where passive.contains(id) {
                armDecay(id)
            }
        }
    }

    private func armDecay(_ id: String) {
        decayTasks[id]?.cancel()
        passive.remove(id)
        let interval = decayInterval
        decayTasks[id] = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .seconds(interval))
            guard let self, !Task.isCancelled else { return }
            passive.insert(id)
            decayTasks[id] = nil
        }
    }
}
