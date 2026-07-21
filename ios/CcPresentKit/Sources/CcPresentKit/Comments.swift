// The comments-sheet projection: pure derivations over the reduced BoardState — the
// pinned block, the document-ordered feed, and the counts. Mirrors web threadFeed.ts.

import Foundation

/// CommentFeedEntry is one row of the comments sheet's document-ordered feed: a block
/// carrying human feedback or agent replies, its display title, and the two counts the
/// row badges. Excludes the pinned active block, which the sheet renders in full above
/// the feed.
public struct CommentFeedEntry: Equatable, Sendable, Identifiable {
    public let blockId: String
    public let title: String
    public let feedbackCount: Int
    public let replyCount: Int

    public var id: String {
        blockId
    }

    public var total: Int {
        feedbackCount + replyCount
    }

    public init(blockId: String, title: String, feedbackCount: Int, replyCount: Int) {
        self.blockId = blockId
        self.title = title
        self.feedbackCount = feedbackCount
        self.replyCount = replyCount
    }
}

/// commentDocBlocks flattens a document to its top-level blocks plus one nesting level of
/// card children, in document order — the walk the comments feed orders by. It repeats
/// the app-side `flatten` because the presentation helper isn't visible to the kit.
public func commentDocBlocks(_ doc: Doc) -> [Block] {
    var out: [Block] = []
    for block in doc.blocks {
        out.append(block)
        if case let .card(card) = block {
            out.append(contentsOf: card.children)
        }
    }
    return out
}

/// commentBlockTitle is the feed-row and pinned-header label for a block: its own prompt,
/// title, or label, falling back to a humanized type name.
public func commentBlockTitle(_ block: Block) -> String {
    switch block {
    case let .approval(approval): approval.prompt ?? "Approval"
    case let .choice(choice): choice.prompt ?? "Choice"
    case let .input(input): input.label
    case let .draft(draft): draft.title ?? "Draft"
    case let .triage(triage): triage.prompt ?? "Triage"
    case let .card(card): card.title ?? "Card"
    case let .section(section): section.title
    default: block.type.capitalized
    }
}

/// commentAcceptsFeedback reports whether the sheet shows a composer for a block: an
/// approval honoring its `allowFeedback` flag, or a choice. Every other block is
/// reply-only in the sheet — feedback is authored only on those two decidables.
public func commentAcceptsFeedback(_ doc: Doc, _ blockId: String) -> Bool {
    for block in commentDocBlocks(doc) where block.id == blockId {
        switch block {
        case let .approval(approval): return approval.allowFeedback ?? true
        case .choice: return true
        default: return false
        }
    }
    return false
}

/// commentCount is the feedback-plus-replies tally for one block — the number a
/// CommentChip badges.
public func commentCount(_ interactions: Interactions, _ blockId: String) -> Int {
    (interactions.feedback[blockId]?.count ?? 0) + (interactions.replies[blockId]?.count ?? 0)
}

/// totalCommentCount sums every block's feedback and replies — the number the board
/// header's comments button badges.
public func totalCommentCount(_ interactions: Interactions) -> Int {
    let feedback = interactions.feedback.values.reduce(0) { $0 + $1.count }
    let replies = interactions.replies.values.reduce(0) { $0 + $1.count }
    return feedback + replies
}

/// resolveActiveComment picks the sheet's pinned block: an explicit chip pin wins, then
/// the focus step's decidable, then the last block a human touched. Mirrors the web
/// active-block precedence (a chip pin over the cursor over lastInteracted).
public func resolveActiveComment(pinned: String?, focusActive: String?, lastInteracted: String?) -> String? {
    pinned ?? focusActive ?? lastInteracted
}

/// commentFeed is the document-ordered list of every block carrying comments except the
/// pinned active block — the sheet's jump index below the pinned thread.
public func commentFeed(_ doc: Doc, _ interactions: Interactions, activeId: String?) -> [CommentFeedEntry] {
    commentDocBlocks(doc).compactMap { block -> CommentFeedEntry? in
        guard block.id != activeId else { return nil }
        let feedback = interactions.feedback[block.id]?.count ?? 0
        let replies = interactions.replies[block.id]?.count ?? 0
        guard feedback + replies > 0 else { return nil }
        return CommentFeedEntry(
            blockId: block.id,
            title: commentBlockTitle(block),
            feedbackCount: feedback,
            replyCount: replies
        )
    }
}

/// commentComposerHolds reports whether a sheet composer is actively drafting — focused,
/// or holding non-whitespace text — the latch that keeps the deck's 450ms auto-advance
/// from firing mid-comment. The iOS twin of the web `data-composing` guard.
public func commentComposerHolds(focused: Bool, draft: String) -> Bool {
    focused || !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
}
