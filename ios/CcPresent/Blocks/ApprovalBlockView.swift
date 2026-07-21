import CcPresentKit
import SwiftUI

/// ApprovalBlockView renders an approval block: an approve/reject verdict pair
/// where re-pressing the active verdict clears it, an expandable feedback composer,
/// and the reply thread. It mirrors web/src/components/Approval.tsx over the native
/// optimistic engine — verdict state is the last-write-wins decision in
/// BoardState, the feedback list is the append-only map, and both post through the
/// injected BoardStore. A closed board disables every control.
struct ApprovalBlockView: View {
    let block: Block.Approval
    let store: BoardStore

    @State private var composing = false
    @State private var draft = ""
    @FocusState private var composerFocused: Bool
    @Environment(\.blockReplies) private var blockReplies
    @Environment(\.focusComposer) private var focusComposer
    @Environment(\.focusHeadlineId) private var focusHeadlineId
    @Environment(\.commentsHost) private var commentsHost

    private var suppressPrompt: Bool {
        focusHeadlineId == block.id
    }

    private var verdict: Verdict? {
        store.state.interactions.decisions[block.id].flatMap { Verdict(rawValue: $0.verdict) }
    }

    private var feedback: [Feedback] {
        store.state.interactions.feedback[block.id] ?? []
    }

    private var replies: [Reply] {
        blockReplies[block.id] ?? []
    }

    private var allowFeedback: Bool {
        block.allowFeedback ?? true
    }

    private var isClosed: Bool {
        store.isClosed
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space4) {
            if !suppressPrompt, let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .foregroundStyle(BlockPalette.ink)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .receiptContent()
            }

            if let detail = block.detail {
                DetailView(detail: detail)
            }

            verdictPair

            if let commentsHost {
                CommentChip(feedbackCount: feedback.count, replyCount: replies.count) {
                    commentsHost.present(pin: block.id)
                }
            } else {
                if allowFeedback, !isClosed {
                    feedbackAffordance
                }

                if !feedback.isEmpty || !replies.isEmpty {
                    Divider().overlay(BlockPalette.line)
                    FeedbackThreadView(feedback: feedback, replies: replies)
                        .receiptContent()
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .onChange(of: composing) { _, now in
            if now {
                composerFocused = true
            }
            focusComposer?.set(block.id, composing: now)
        }
        .onDisappear {
            focusComposer?.set(block.id, composing: false)
        }
    }

    // MARK: - Verdict

    private var verdictPair: some View {
        HStack(spacing: Metrics.space3) {
            verdictButton(.approved, label: "Approve", glyph: "checkmark", color: BlockPalette.approve)
            verdictButton(.rejected, label: "Reject", glyph: "xmark", color: BlockPalette.reject)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(suppressPrompt ? (block.prompt ?? "Verdict") : "Verdict")
    }

    private func verdictButton(_ target: Verdict, label: String, glyph: String, color: Color) -> some View {
        let active = verdict == target
        return Button {
            choose(target)
        } label: {
            VerdictLabel(glyph: glyph, title: label)
        }
        .buttonStyle(VerdictButtonStyle(tint: color, active: active))
        .disabled(isClosed)
        .accessibilityLabel(label)
        .accessibilityAddTraits(active ? [.isSelected] : [])
    }

    /// choose applies the verdict a press produces: re-pressing the active verdict
    /// clears it, the opposite switches — parity with web `verdictToggle`.
    private func choose(_ target: Verdict) {
        store.decide(blockId: block.id, verdict: verdict == target ? .cleared : target)
    }

    // MARK: - Feedback composer

    @ViewBuilder
    private var feedbackAffordance: some View {
        if composing {
            VStack(alignment: .leading, spacing: Metrics.space2) {
                TextField("Add feedback for the agent…", text: $draft, axis: .vertical)
                    .lineLimit(2 ... 5)
                    .font(.subheadline)
                    .padding(Metrics.space3)
                    .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
                    .overlay(
                        RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(BlockPalette.line, lineWidth: 1)
                    )
                    .focused($composerFocused)
                    .accessibilityLabel("Feedback for the agent")

                HStack(spacing: Metrics.space3) {
                    Button("Send", action: sendFeedback)
                        .buttonStyle(PrimaryButtonStyle())
                        .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                    Button("Cancel") {
                        composing = false
                        draft = ""
                    }
                    .buttonStyle(GhostButtonStyle(tint: BlockPalette.muted))
                }
            }
        } else {
            Button("Add feedback") {
                composing = true
            }
            .buttonStyle(GhostButtonStyle())
        }
    }

    private func sendFeedback() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        store.feedback(blockId: block.id, text: text)
        draft = ""
        composing = false
    }
}

// MARK: - Preview

/// PreviewPoster is a no-op InteractionPoster for previews: it accepts every post
/// and hands back a fixed seq, so optimistically-applied seed interactions stay
/// visible without a network.
private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

@MainActor
private func previewStore() -> BoardStore {
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    store.decide(blockId: "ap1", verdict: .approved)
    store.feedback(blockId: "ap1", text: "Rename the helper before merging.")
    store.feedback(blockId: "ap1", text: "Add a test for the empty log.")
    return store
}

#Preview("Approval") {
    ScrollView {
        ApprovalBlockView(
            block: Block.Approval(
                id: "ap1",
                prompt: "Approve the reduce-order refactor?",
                allowFeedback: true,
                detail: Block.Detail(
                    pros: ["Deterministic replay", "Smaller event log"],
                    cons: ["Touches every reducer call site"],
                    md: "The reducer now folds events in receipt order, so a fresh tab replays identically.",
                    mode: "inline"
                )
            ),
            store: previewStore()
        )
        .padding()
    }
}

#Preview("Approval — no feedback, undecided") {
    ScrollView {
        ApprovalBlockView(
            block: Block.Approval(id: "ap2", prompt: "Ship it?", allowFeedback: false),
            store: BoardStore(subject: "preview", transport: PreviewPoster())
        )
        .padding()
    }
}
