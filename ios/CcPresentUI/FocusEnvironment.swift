import SwiftUI

/// FocusComposer is the deck-scoped signal the auto-advance timer consults: an
/// approval's feedback composer registers itself while open so a verdict's 450ms
/// advance is cancelled mid-compose, the native mirror of the web `data-composing`
/// attribute. FocusDeckView owns one and injects it; board mode leaves it nil.
@MainActor
@Observable
public final class FocusComposer {
    public init() {}

    private var open: Set<String> = []

    public var isComposing: Bool {
        !open.isEmpty
    }

    public func set(_ id: String, composing: Bool) {
        if composing {
            open.insert(id)
        } else {
            open.remove(id)
        }
    }
}

public extension EnvironmentValues {
    /// focusComposer is the open-composer signal ApprovalBlockView writes to while
    /// its feedback composer is up, so the focus deck can hold its auto-advance.
    @Entry var focusComposer: FocusComposer? = nil

    /// focusHeadlineId names the decidable whose prompt the focus card hoisted into the
    /// step headline, so ChoiceBlockView/ApprovalBlockView suppress the now-duplicate
    /// inline prompt (keeping it as an accessibility label). nil in board mode.
    @Entry var focusHeadlineId: String? = nil

    /// inFocusCard is true for the focal block of a focus step, so CardView drops its
    /// head — the card's title, status, and chips are hoisted into the deck's meta row.
    /// false in board mode, where the card renders its head as authored.
    @Entry var inFocusCard: Bool = false
}
