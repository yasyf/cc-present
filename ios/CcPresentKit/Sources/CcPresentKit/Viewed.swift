import CoreGraphics
import Foundation

/// ViewedStore records the blocks the human opened this round, persisted per subject;
/// the submit carries it as `viewed`. Mirrors web/src/viewed.ts.
@MainActor
public final class ViewedStore {
    public nonisolated static let keyPrefix = "cc-present:viewed:v1:"
    public nonisolated static let focusDwell: Duration = .milliseconds(800)
    public nonisolated static let boardDwell: Duration = .milliseconds(1500)

    public let subject: String
    public private(set) var round = 0
    private var ids: [String] = []
    private let defaults: UserDefaults

    public init(subject: String, defaults: UserDefaults = .standard) {
        self.subject = subject
        self.defaults = defaults
    }

    private var key: String {
        Self.keyPrefix + subject
    }

    /// scope loads what an earlier launch recorded for `round`; an older round starts fresh.
    public func scope(round: Int) {
        guard round != self.round else { return }
        self.round = round
        guard let stored = defaults.dictionary(forKey: key) else {
            ids = []
            return
        }
        guard let storedRound = stored["round"] as? Int, let storedIds = stored["ids"] as? [String] else {
            preconditionFailure("cc-present viewed record is corrupt")
        }
        ids = storedRound == round ? storedIds : []
    }

    /// mark adds `newIds` in first-seen order and persists the set when it grew.
    public func mark(_ newIds: [String]) {
        let before = ids.count
        for id in newIds where !ids.contains(id) {
            ids.append(id)
        }
        guard ids.count != before else { return }
        defaults.set(["round": round, "ids": ids], forKey: key)
    }

    public var viewed: [String] {
        ids
    }
}

/// Dwell is the viewed timer as a pure state machine over a clock's instants: a block
/// counts once it stays active for `duration` unbroken, and it completes at most once.
public struct Dwell<Instant: InstantProtocol>: Equatable where Instant.Duration == Duration {
    public let duration: Duration
    public private(set) var since: Instant?
    public private(set) var done = false

    public init(duration: Duration) {
        self.duration = duration
    }

    /// update records whether the block is active at `now` and returns the instant its
    /// dwell completes while it stays active, or nil when inactive or already done.
    public mutating func update(active: Bool, at now: Instant) -> Instant? {
        guard !done, active else {
            since = nil
            return nil
        }
        let start = since ?? now
        since = start
        return start.advanced(by: duration)
    }

    /// complete reports whether the dwell has elapsed at `now`, true exactly once.
    public mutating func complete(at now: Instant) -> Bool {
        guard !done, let since, since.advanced(by: duration) <= now else { return false }
        done = true
        return true
    }
}

/// isViewedOnScreen reports a row at least half visible, or spanning the viewport's
/// midline so a row taller than the screen still qualifies. Mirrors web/src/viewed.ts.
public func isViewedOnScreen(row: CGRect, viewport: CGRect) -> Bool {
    let visible = row.intersection(viewport)
    let ratio = visible.isNull ? 0 : (visible.width * visible.height) / (row.width * row.height)
    return ratio >= 0.5 || (row.minY <= viewport.midY && row.maxY >= viewport.midY)
}
