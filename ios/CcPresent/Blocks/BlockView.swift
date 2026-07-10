import CcPresentKit
import SwiftUI

/// BlockView dispatches a Block to its renderer. The switch is exhaustive with no
/// default arm, so a new block type is a compile error until it is handled here —
/// the native mirror of web/src/components/BlockRenderer.tsx. The `store` drives the
/// interactive blocks (approval, choice, input, and a card's interactive children);
/// `client` authorizes asset image loads.
struct BlockView: View {
    let block: Block
    let store: BoardStore
    var client: APIClient?

    var body: some View {
        switch block {
        case let .section(section):
            SectionView(block: section)
        case let .card(card):
            CardView(block: card, store: store, client: client)
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
        }
    }
}
