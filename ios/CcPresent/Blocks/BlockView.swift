import CcPresentKit
import SwiftUI

/// BlockView dispatches a Block to its renderer and splices the block's agent reply
/// thread beneath it — the native mirror of web/src/components/BlockRenderer.tsx.
/// The `blockContent` switch is exhaustive with no default arm, so a new block type
/// is a compile error until it is handled there. The reply thread reads
/// `blockReplies` from the environment and renders for every block whose
/// `showsNativeReplyThread` is true (all but approval, which owns its integrated
/// thread, and pack, which threads inside its WKWebView). The `store` drives the
/// interactive blocks (approval, choice, input, and a card's interactive children);
/// `client` authorizes asset image loads.
struct BlockView: View {
    let block: Block
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.blockReplies) private var blockReplies

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            blockContent
            let thread = blockReplies[block.id] ?? []
            if showsNativeReplyThread(block), !thread.isEmpty {
                FeedbackThreadView(feedback: [], replies: thread)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder
    private var blockContent: some View {
        switch block {
        case let .section(section):
            SectionView(block: section)
        case let .card(card):
            CardView(block: card, store: store, client: client, packContext: packContext)
        case let .approval(approval):
            ApprovalBlockView(block: approval, store: store)
        case let .choice(choice):
            ChoiceBlockView(block: choice, store: store)
        case let .input(input):
            InputBlockView(block: input, store: store)
        case let .markdown(markdown):
            MarkdownText(markdown.md, style: markdown.struck == true ? .struck : .clamped)
                .frame(maxWidth: .infinity, alignment: .leading)
        case let .code(code):
            CodeBlockView(block: code)
        case let .diff(diff):
            DiffBlockView(block: diff)
        case let .image(image):
            ImageBlockView(block: image, client: client)
        case let .table(table):
            TableBlockView(block: table)
        case let .progress(progress):
            ProgressBlockView(block: progress)
        case let .pack(pack):
            if let packContext {
                PackBlockWebView(block: pack, context: packContext)
            } else {
                PackPlaceholderView(pack: pack)
            }
        }
    }
}

extension EnvironmentValues {
    /// blockReplies is the live agent reply thread per block id, injected once at the
    /// board root so every BlockView — live or history — splices in the same replies.
    /// The native mirror of the web SingleBlockView reply thread.
    @Entry var blockReplies: [String: [Reply]] = [:]
}

/// PackPlaceholderView is the native stand-in for a plugin-supplied pack block when
/// no PackContext is available (e.g. a preview): a labeled card naming the pack type
/// and block id. On a live board BlockView renders the block through PackBlockWebView.
private struct PackPlaceholderView: View {
    let pack: Block.Pack

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(pack.packType)
                .font(.footnote.monospaced())
                .foregroundStyle(BlockPalette.accentInk)
            Text(pack.id)
                .font(.footnote)
                .foregroundStyle(BlockPalette.muted)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: 8))
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .stroke(BlockPalette.line, lineWidth: 1)
        )
    }
}
