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
        VStack(alignment: .leading, spacing: 14) {
            if let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .foregroundStyle(BlockPalette.ink)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            verdictPair

            if allowFeedback, !isClosed {
                feedbackAffordance
            }

            if !feedback.isEmpty || !replies.isEmpty {
                Divider().overlay(BlockPalette.line)
                FeedbackThreadView(feedback: feedback, replies: replies)
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
        HStack(spacing: 10) {
            verdictButton(.approved, label: "Approve", glyph: "checkmark", color: BlockPalette.approve)
            verdictButton(.rejected, label: "Reject", glyph: "xmark", color: BlockPalette.reject)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("Verdict")
    }

    private func verdictButton(_ target: Verdict, label: String, glyph: String, color: Color) -> some View {
        let active = verdict == target
        return Button {
            choose(target)
        } label: {
            HStack(spacing: 6) {
                Image(systemName: glyph)
                    .font(.system(size: 13, weight: .bold))
                Text(label)
                    .font(.system(size: 15, weight: .semibold))
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 11)
            .foregroundStyle(active ? Self.activeInk : color)
            .background(active ? color : Color.clear)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .strokeBorder(color.opacity(active ? 0 : 0.55), lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
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
            VStack(alignment: .leading, spacing: 10) {
                TextField("Add feedback for the agent…", text: $draft, axis: .vertical)
                    .lineLimit(2 ... 5)
                    .font(.subheadline)
                    .padding(10)
                    .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 8))
                    .overlay(
                        RoundedRectangle(cornerRadius: 8).strokeBorder(BlockPalette.line, lineWidth: 1)
                    )
                    .focused($composerFocused)
                    .accessibilityLabel("Feedback for the agent")

                HStack(spacing: 12) {
                    Button("Send", action: sendFeedback)
                        .buttonStyle(.borderedProminent)
                        .tint(BlockPalette.accentInk)
                        .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                    Button("Cancel") {
                        composing = false
                        draft = ""
                    }
                    .buttonStyle(.bordered)
                    .tint(BlockPalette.muted)
                }
                .font(.system(size: 14, weight: .semibold))
            }
        } else {
            Button("Add feedback") {
                composing = true
            }
            .buttonStyle(.plain)
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(BlockPalette.accentInk)
        }
    }

    private func sendFeedback() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        store.feedback(blockId: block.id, text: text)
        draft = ""
        composing = false
    }

    /// activeInk is the label color on a filled verdict button: near-white in light
    /// mode, near-black in dark, so text stays legible on the tint in both.
    private static let activeInk = Color(hexLight: 0xFFFFFF, hexDark: 0x0C110F)
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
                allowFeedback: true
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
