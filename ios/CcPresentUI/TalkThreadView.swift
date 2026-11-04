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

    private static let bubble = Color(red: 0.169, green: 0.435, blue: 0.902)

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

    private var canSend: Bool {
        !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    private var awaitingReply: Bool {
        if !store.isClosed, case .note = entries.last { return true }
        return false
    }

    private func startsTurn(at index: Int) -> Bool {
        if case .note = entries[index] { return false }
        if index == 0 { return true }
        if case .note = entries[index - 1] { return true }
        return false
    }

    public var body: some View {
        VStack(spacing: 0) {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 18) {
                        if entries.isEmpty {
                            emptyState
                        }
                        ForEach(Array(entries.enumerated()), id: \.offset) { index, entry in
                            VStack(alignment: .leading, spacing: 6) {
                                if startsTurn(at: index) {
                                    Text("Communicator")
                                        .voice(.prose, size: 12)
                                        .foregroundStyle(Palette.muted)
                                }
                                row(entry)
                            }
                            .id(index)
                        }
                        if awaitingReply {
                            Text("Waiting for the communicator")
                                .voice(.prose, size: 13)
                                .foregroundStyle(Palette.muted)
                        }
                    }
                    .padding(.horizontal, Metrics.space4)
                    .padding(.vertical, 20)
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
                    .foregroundStyle(.white)
                    .textSelection(.enabled)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 8)
                    .background(
                        Self.bubble,
                        in: UnevenRoundedRectangle(topLeadingRadius: 16, bottomLeadingRadius: 16, bottomTrailingRadius: 4, topTrailingRadius: 16)
                    )
            }
        case let .reply(reply):
            MarkdownText(reply.md)
                .frame(maxWidth: .infinity, alignment: .leading)
        case let .block(block):
            BlockView(block: block, store: store, client: client, packContext: packContext)
        }
    }

    private var emptyState: some View {
        VStack(spacing: 4) {
            Text("Ask the communicator")
                .voice(.prose, size: 15)
                .fontWeight(.semibold)
                .foregroundStyle(Palette.ink)
            Text("It answers here, and can reply with tables you filter and open.")
                .voice(.prose, size: 14)
                .foregroundStyle(Palette.muted)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 80)
    }

    private var composer: some View {
        HStack(alignment: .bottom, spacing: Metrics.space2) {
            TextField("Message the communicator", text: $draft, axis: .vertical)
                .lineLimit(1 ... 6)
                .voice(.prose, size: 15)
                .padding(.vertical, 5)
            Button {
                let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
                draft = ""
                store.feedback(blockId: talkId, text: text)
            } label: {
                Image(systemName: "arrow.up")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(canSend ? Color.white : Palette.muted)
                    .frame(width: 30, height: 30)
                    .background(canSend ? Self.bubble : Palette.chipBg, in: Circle())
            }
            .disabled(!canSend)
            .accessibilityLabel("Send")
        }
        .padding(.leading, 14)
        .padding(.trailing, 6)
        .padding(.vertical, 6)
        .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: 20))
        .overlay(RoundedRectangle(cornerRadius: 20).stroke(Palette.borderStrong))
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }
}
