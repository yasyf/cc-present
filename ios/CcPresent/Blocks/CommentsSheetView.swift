import CcPresentKit
import SwiftUI

/// CommentsModel coordinates the board's native comments sheet: whether it is shown, the
/// explicitly pinned block, the focus deck's current decidable and jump handler (attached
/// while the deck is mounted), and the board's scroll-to fallback. It is the iOS margin —
/// the platform sheet standing in for the web margin rail.
@MainActor
@Observable
final class CommentsModel {
    var isPresented = false
    var pinnedId: String?
    var focusActiveId: String?

    @ObservationIgnored var composer: FocusComposer?
    @ObservationIgnored private var deckJump: ((String) -> Void)?
    @ObservationIgnored var boardScrollTo: ((String) -> Void)?

    /// present opens the sheet, optionally pinning a specific block; a nil pin lets the
    /// active block resolve to the focus decidable, then last-interacted.
    func present(pin: String?) {
        pinnedId = pin
        isPresented = true
    }

    /// attachDeck wires the deck's composer (so a sheet composer holds the auto-advance)
    /// and its jump handler while the focus deck is mounted.
    func attachDeck(composer: FocusComposer, jump: @escaping (String) -> Void) {
        self.composer = composer
        deckJump = jump
    }

    /// detachDeck drops the deck wiring when the deck unmounts, so board mode falls back
    /// to last-interacted and the board scroll-to jump.
    func detachDeck() {
        composer = nil
        deckJump = nil
        focusActiveId = nil
    }

    /// jump dismisses the sheet and routes to the block: the deck's jump in focus mode,
    /// the board's scroll-to otherwise.
    func jump(to id: String) {
        isPresented = false
        (deckJump ?? boardScrollTo)?(id)
    }
}

extension EnvironmentValues {
    /// commentsHost is the board's comments coordinator. When present, Approval and Choice
    /// render a CommentChip that opens the sheet instead of an inline thread and composer;
    /// nil (previews, single-block embeds) keeps the inline rendering.
    @Entry var commentsHost: CommentsModel? = nil
}

/// CommentsSheetView is the native margin: a pinned active-block thread and composer on
/// top, then a document-ordered feed of every other block's comments with a jump action.
/// Presented with `.medium`/`.large` detents over the cardLift ground. Mirrors the web
/// margin rail / CommentsSheet.
struct CommentsSheetView: View {
    let store: BoardStore
    let comments: CommentsModel

    private var interactions: Interactions {
        store.state.interactions
    }

    private var doc: Doc {
        store.state.doc
    }

    private var activeId: String? {
        resolveActiveComment(
            pinned: comments.pinnedId,
            focusActive: comments.focusActiveId,
            lastInteracted: store.lastInteracted
        )
    }

    private var feed: [CommentFeedEntry] {
        commentFeed(doc, interactions, activeId: activeId)
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider().overlay(BlockPalette.line)
            ScrollView {
                VStack(alignment: .leading, spacing: Metrics.space5) {
                    pinnedSection
                    if !feed.isEmpty {
                        feedSection
                    }
                    if activeId == nil, feed.isEmpty {
                        Text("No comments yet")
                            .font(.subheadline)
                            .foregroundStyle(BlockPalette.muted)
                            .frame(maxWidth: .infinity, alignment: .center)
                            .padding(.top, Metrics.space6)
                    }
                }
                .padding(Metrics.space4)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
        .presentationBackground(alignment: .top) {
            BlockPalette.cardLift
                .overlay(alignment: .top) {
                    Rectangle().fill(BlockPalette.edgeLift).frame(height: 1)
                }
                .ignoresSafeArea()
        }
    }

    private var header: some View {
        HStack(spacing: Metrics.space2) {
            Text("Comments")
                .voice(.stamp, size: 16, weight: .semibold)
                .foregroundStyle(BlockPalette.ink)
            let count = totalCommentCount(interactions)
            if count > 0 {
                Text("\(count)")
                    .voice(.mono, size: 12)
                    .foregroundStyle(BlockPalette.muted)
            }
            Spacer(minLength: 0)
            Button("Done") { comments.isPresented = false }
                .buttonStyle(GhostButtonStyle())
        }
        .padding(.horizontal, Metrics.space4)
        .padding(.vertical, Metrics.space3)
    }

    @ViewBuilder
    private var pinnedSection: some View {
        if let activeId {
            let feedback = interactions.feedback[activeId] ?? []
            let replies = interactions.replies[activeId] ?? []
            let block = commentDocBlocks(doc).first { $0.id == activeId }
            VStack(alignment: .leading, spacing: Metrics.space3) {
                if let block {
                    Text(commentBlockTitle(block))
                        .voice(.stamp, size: 11, weight: .semibold)
                        .foregroundStyle(BlockPalette.muted)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                if !store.isClosed, commentAcceptsFeedback(doc, activeId) {
                    CommentComposer(blockId: activeId, store: store, composer: comments.composer)
                }
                if feedback.isEmpty, replies.isEmpty {
                    Text("No comments on this block yet")
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.muted)
                } else {
                    FeedbackThreadView(feedback: feedback, replies: replies)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .id(activeId)
        }
    }

    private var feedSection: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            Text("Other blocks")
                .voice(.stamp, size: 11, weight: .semibold)
                .foregroundStyle(BlockPalette.muted)
            ForEach(feed) { entry in
                CommentFeedRow(entry: entry) { comments.jump(to: entry.blockId) }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// CommentComposer is the one unified sheet composer replacing the duplicated inline
/// feedback and note affordances. It registers with the deck's FocusComposer while it
/// holds a draft, so an open composer holds the 450ms auto-advance.
private struct CommentComposer: View {
    let blockId: String
    let store: BoardStore
    let composer: FocusComposer?

    @State private var draft = ""
    @FocusState private var focused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            TextField("Add a comment for the agent…", text: $draft, axis: .vertical)
                .lineLimit(2 ... 5)
                .font(.subheadline)
                .padding(Metrics.space3)
                .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
                .overlay(
                    RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(BlockPalette.line, lineWidth: 1)
                )
                .focused($focused)
                .accessibilityLabel("Comment for the agent")

            Button("Send", action: send)
                .buttonStyle(PrimaryButtonStyle())
                .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
        .onChange(of: focused) { _, _ in updateHold() }
        .onChange(of: draft) { _, _ in updateHold() }
        .onDisappear { composer?.set(blockId, composing: false) }
    }

    private func updateHold() {
        composer?.set(blockId, composing: commentComposerHolds(focused: focused, draft: draft))
    }

    private func send() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        store.feedback(blockId: blockId, text: text)
        draft = ""
        focused = false
        composer?.set(blockId, composing: false)
    }
}

/// CommentFeedRow is one document-ordered feed entry: the block's title, its feedback and
/// reply counts, and a chevron; tapping dismisses the sheet and jumps to the block.
private struct CommentFeedRow: View {
    let entry: CommentFeedEntry
    let onJump: () -> Void

    var body: some View {
        Button(action: onJump) {
            HStack(spacing: Metrics.space3) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(entry.title)
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(BlockPalette.ink)
                        .lineLimit(1)
                    HStack(spacing: Metrics.space3) {
                        if entry.feedbackCount > 0 {
                            Label("\(entry.feedbackCount)", systemImage: "person.crop.circle")
                        }
                        if entry.replyCount > 0 {
                            Label("\(entry.replyCount)", systemImage: "bubble.left")
                        }
                    }
                    .voice(.mono, size: 10)
                    .foregroundStyle(BlockPalette.muted)
                }
                Spacer(minLength: Metrics.space2)
                Image(systemName: "chevron.right")
                    .font(.caption)
                    .foregroundStyle(BlockPalette.muted)
            }
            .padding(Metrics.space3)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(entry.title), \(entry.total) comments")
        .accessibilityHint("Jumps to the block")
    }
}
