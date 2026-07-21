import CcPresentKit
import SwiftUI

/// FeedbackThreadView renders an approval block's conversation: human feedback
/// first, then agent replies, mirroring the `.thread` list in
/// web/src/components/Approval.tsx. Feedback is plain text; each reply renders as
/// clamped markdown. The caller owns the emptiness check — an empty thread renders
/// nothing.
struct FeedbackThreadView: View {
    let feedback: [Feedback]
    let replies: [Reply]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(feedback, id: \.id) { item in
                ThreadItem(who: .you) {
                    Text(item.text)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.ink)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            ForEach(replies, id: \.id) { reply in
                ThreadItem(who: .agent) {
                    MarkdownText(reply.md, style: .clamped)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// ThreadItem is one row of the feedback thread: a small author tag beside its
/// content. The `you`/`agent` distinction tints the tag, matching the web
/// `.thread-who` treatment.
private struct ThreadItem<Content: View>: View {
    enum Who {
        case you
        case agent

        var label: String {
            switch self {
            case .you: "you"
            case .agent: "agent"
            }
        }

        var tint: Color {
            switch self {
            case .you: BlockPalette.muted
            case .agent: BlockPalette.accentInk
            }
        }
    }

    let who: Who
    @ViewBuilder let content: Content

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Text(who.label.uppercased())
                .voice(.mono, size: 10, weight: .semibold)
                .tracking(0.8)
                .foregroundStyle(who.tint)
                .frame(width: 42, alignment: .leading)
                .padding(.top, 2)
                .accessibilityLabel(who == .you ? "You" : "Agent")

            content
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}

#Preview("Feedback thread") {
    ScrollView {
        FeedbackThreadView(
            feedback: [
                Feedback(id: "f1", text: "Looks good, but rename the helper before merging."),
                Feedback(id: "f2", text: "Also add a test for the empty case."),
            ],
            replies: [
                Reply(
                    id: "r1",
                    md: """
                    Renamed `apply` to **`applyOrdered`** and threaded the round through. Here's the
                    shape of the change:

                    - `reduce` now sorts by `seq` before folding.
                    - Added a table-driven test covering the empty log, a single decision, and a
                      cleared verdict.
                    - The clamp trims this reply to ten lines, so this final sentence is here only to
                      push the block past the fold and surface the *Show more* toggle in the preview.
                    """
                ),
            ]
        )
        .padding()
    }
}
