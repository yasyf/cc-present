import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI

/// TalkThreadView renders a Talk board natively as one thread, notes and the
/// communicator's replies and blocks in order, with a composer that posts a note.
/// Mirrors web TalkView.
public struct TalkThreadView: View {
    let store: BoardStore
    let talkId: String
    let client: HTTPClient?
    let packContext: PackContext?

    @State private var draft = ""

    public init(store: BoardStore, talkId: String, client: HTTPClient?, packContext: PackContext?) {
        self.store = store
        self.talkId = talkId
        self.client = client
        self.packContext = packContext
    }

    private var entries: [TalkEntry] {
        let interactions = store.state.interactions
        return talkEntries(
            store.state.doc.blocks,
            talkId: talkId,
            feedback: interactions.feedback[talkId] ?? [],
            replies: interactions.replies[talkId] ?? []
        )
    }

    public var body: some View {
        VStack(spacing: 0) {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: Metrics.space3) {
                        ForEach(Array(entries.enumerated()), id: \.offset) { index, entry in
                            row(entry).id(index)
                        }
                    }
                    .padding(Metrics.space4)
                }
                .defaultScrollAnchor(.bottom)
                .onChange(of: entries.count) { _, count in
                    withAnimation { proxy.scrollTo(count - 1, anchor: .bottom) }
                }
            }
            if !store.isClosed {
                composer
            }
        }
        .background(Palette.cardLift)
    }

    @ViewBuilder
    private func row(_ entry: TalkEntry) -> some View {
        switch entry {
        case let .note(note):
            HStack {
                Spacer(minLength: Metrics.space7)
                Text(note.text)
                    .voice(.prose, size: 15)
                    .foregroundStyle(Palette.accentFg)
                    .textSelection(.enabled)
                    .padding(.horizontal, Metrics.space3)
                    .padding(.vertical, Metrics.space2)
                    .background(Palette.accentInk, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
            }
        case let .reply(reply):
            MarkdownText(reply.md)
                .padding(Metrics.space3)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        case let .block(block):
            BlockView(block: block, store: store, client: client, packContext: packContext)
        }
    }

    private var composer: some View {
        HStack(alignment: .bottom, spacing: Metrics.space2) {
            TextField("Message the communicator", text: $draft, axis: .vertical)
                .lineLimit(1 ... 6)
                .voice(.prose, size: 15)
                .padding(.horizontal, Metrics.space3)
                .padding(.vertical, Metrics.space2)
                .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
            Button("Send") {
                let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
                draft = ""
                store.feedback(blockId: talkId, text: text)
            }
            .buttonStyle(PrimaryButtonStyle())
            .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
        .padding(Metrics.space3)
        .background(Palette.cardLift)
        .overlay(alignment: .top) { Palette.line.frame(height: 1) }
    }
}
