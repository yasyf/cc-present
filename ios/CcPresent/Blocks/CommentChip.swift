import SwiftUI

/// CommentChip is the inline comments affordance on an Approval or Choice block in sheet
/// mode: a feedback-plus-replies count that opens the comments sheet pinned to the block,
/// replacing the block's inline thread and add-feedback affordance. Mirrors the web
/// comment-count chip.
struct CommentChip: View {
    let feedbackCount: Int
    let replyCount: Int
    let action: () -> Void

    private var total: Int {
        feedbackCount + replyCount
    }

    var body: some View {
        Button(action: action) {
            HStack(spacing: Metrics.space1) {
                Image(systemName: total == 0 ? "bubble.left" : "bubble.left.fill")
                    .font(.system(size: 12, weight: .semibold))
                Text(total == 0 ? "Comment" : "\(total)")
                    .voice(.prose, size: 13, weight: .semibold)
            }
        }
        .buttonStyle(GhostButtonStyle())
        .accessibilityLabel(total == 0 ? "Add a comment" : "Comments, \(total)")
        .accessibilityHint("Opens the comments sheet")
    }
}

#Preview("CommentChip") {
    VStack(alignment: .leading, spacing: Metrics.space4) {
        CommentChip(feedbackCount: 0, replyCount: 0) {}
        CommentChip(feedbackCount: 2, replyCount: 1) {}
    }
    .padding()
}
