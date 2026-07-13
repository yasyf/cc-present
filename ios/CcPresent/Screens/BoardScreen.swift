import CcPresentKit
import SwiftUI

/// BoardScreen renders one subject's live board. It owns a BoardStore for the
/// (machine, subject) pair, attaches an SSEClient stream that drives the reduction,
/// and lays the board out: the document header, closed-round history through
/// RoundGroupView, the current round's blocks through BlockView, and a pinned
/// SubmitBarView. A connection banner reflects the transport, the board turns
/// read-only once closed, and a skeleton stands in until the replay catches up.
/// The `(machine:subject:)` signature is the contract the sessions list navigates
/// into and stays exactly this shape.
struct BoardScreen: View {
    let machine: Machine
    let subject: String

    @State private var store: BoardStore
    @State private var viewOverride: ViewMode?
    @State private var declaredInteractive: Set<String>?
    private let client: APIClient
    private let bearerToken: String?

    init(machine: Machine, subject: String) {
        self.machine = machine
        self.subject = subject
        let token = (try? TokenStore.token(machineID: machine.id)) ?? nil
        let client = APIClient(baseURL: machine.baseURL, bearerToken: token)
        self.client = client
        bearerToken = token
        _store = State(initialValue: BoardStore(subject: subject, transport: client))
        _viewOverride = State(initialValue: loadViewOverride(subject: subject))
    }

    private var state: BoardState {
        store.state
    }

    private var packContext: PackContext {
        PackContext(baseURL: machine.baseURL, token: bearerToken, subject: store.subject)
    }

    private var currentBlocks: [Block] {
        state.doc.blocks.filter { state.rounds.blockRounds[$0.id] == state.rounds.current }
    }

    private var packInteractive: Set<String> {
        interactivePackTypes(declared: declaredInteractive, blocks: currentBlocks)
    }

    private var deckSteps: [FocusStep] {
        focusSteps(currentBlocks, packInteractive)
    }

    private var mode: ViewMode {
        resolveViewMode(presentation: state.doc.presentation, override: viewOverride, steps: deckSteps)
    }

    private var hasHistory: Bool {
        !state.rounds.history.isEmpty
    }

    private var isWaiting: Bool {
        !store.isClosed && currentBlocks.isEmpty
    }

    private var hasContent: Bool {
        !state.doc.blocks.isEmpty || hasHistory || store.isClosed
    }

    var body: some View {
        content
            .navigationTitle(subject)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) { viewToggle }
            }
            .safeAreaInset(edge: .top, spacing: 0) { connectionBanner }
            .safeAreaInset(edge: .bottom, spacing: 0) { submitBar }
            .task {
                let sse = SSEClient(baseURL: machine.baseURL, session: subject, bearerToken: bearerToken)
                let connection = await sse.connect()
                store.connect(connection)
            }
            .task {
                if let response = try? await client.packs() {
                    declaredInteractive = response.interactiveTypes
                }
            }
    }

    @ViewBuilder
    private var content: some View {
        if store.isLoading, !hasContent {
            BoardSkeletonView()
        } else {
            board
        }
    }

    private var board: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                docHeader
                if store.isClosed {
                    ClosedBannerView(summary: state.interactions.closed.summary)
                }
                ForEach(state.rounds.history, id: \.number) { record in
                    RoundGroupView(record: record, client: client, packContext: packContext)
                }
                if !currentBlocks.isEmpty, hasHistory || !state.rounds.currentTitle.isEmpty {
                    currentRoundHeader
                }
                if mode == .focus, !currentBlocks.isEmpty {
                    FocusDeckView(
                        steps: deckSteps,
                        store: store,
                        packInteractive: packInteractive,
                        client: client,
                        packContext: packContext
                    )
                } else {
                    ForEach(currentBlocks, id: \.id) { block in
                        BlockView(block: block, store: store, client: client, packContext: packContext)
                            .environment(\.receiptReceded, blockDecided(block, state.interactions, packInteractive))
                    }
                }
                if isWaiting {
                    WaitingPanelView(round: state.rounds.current, lastRound: state.rounds.history.last)
                }
            }
            .padding()
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .environment(\.blockReplies, state.interactions.replies)
    }

    @ViewBuilder
    private var docHeader: some View {
        if !state.doc.title.isEmpty || state.doc.intro != nil || !(state.doc.stats ?? []).isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                if !state.doc.title.isEmpty {
                    Text(state.doc.title)
                        .font(.title.weight(.semibold))
                        .foregroundStyle(BlockPalette.ink)
                }
                if let intro = state.doc.intro, !intro.isEmpty {
                    Text(intro)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.muted)
                }
                if let stats = state.doc.stats, !stats.isEmpty {
                    HStack(spacing: 16) {
                        ForEach(Array(stats.enumerated()), id: \.offset) { _, stat in
                            HStack(spacing: 4) {
                                Text(stat.value)
                                    .fontWeight(.bold)
                                    .foregroundStyle(BlockPalette.ink)
                                Text(stat.label)
                                    .foregroundStyle(BlockPalette.muted)
                            }
                            .font(.caption)
                        }
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private var currentRoundHeader: some View {
        Text(currentRoundTitle)
            .font(.caption)
            .foregroundStyle(BlockPalette.accentInk)
            .monospacedDigit()
    }

    private var currentRoundTitle: String {
        var title = "Round \(state.rounds.current)"
        if !state.rounds.currentTitle.isEmpty {
            title += " · \(state.rounds.currentTitle)"
        }
        return title
    }

    @ViewBuilder
    private var viewToggle: some View {
        if !currentBlocks.isEmpty {
            Button {
                let next: ViewMode = mode == .focus ? .board : .focus
                viewOverride = next
                saveViewOverride(subject: subject, mode: next)
            } label: {
                Image(systemName: mode == .focus ? "list.bullet.rectangle" : "square.stack")
            }
            .accessibilityLabel(mode == .focus ? "Switch to board view" : "Switch to focus view")
        }
    }

    @ViewBuilder
    private var connectionBanner: some View {
        switch store.connectionState {
        case .connecting:
            BannerBar(text: "Connecting…", systemImage: "wifi", tint: BlockPalette.muted)
        case .reconnecting:
            BannerBar(text: "Reconnecting…", systemImage: "wifi.exclamationmark", tint: BlockPalette.reject)
        case .live:
            EmptyView()
        }
    }

    @ViewBuilder
    private var submitBar: some View {
        if !store.isClosed, !currentBlocks.isEmpty {
            SubmitBarView(
                blocks: currentBlocks,
                doc: state.doc,
                store: store,
                packInteractive: packInteractive,
                hasHistory: hasHistory
            )
        }
    }
}

/// BannerBar is the thin connectivity strip the board pins under the navigation bar
/// while the stream is not live.
private struct BannerBar: View {
    let text: String
    let systemImage: String
    let tint: Color

    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: systemImage)
            Text(text)
            Spacer(minLength: 0)
        }
        .font(.caption.weight(.medium))
        .foregroundStyle(tint)
        .padding(.horizontal, 16)
        .padding(.vertical, 6)
        .frame(maxWidth: .infinity)
        .background(.bar)
    }
}

/// ClosedBannerView marks a terminated presentation and renders its closing
/// summary. Mirrors web/src/components/ClosedBanner.tsx.
private struct ClosedBannerView: View {
    let summary: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Closed")
                .font(.caption.weight(.semibold))
                .foregroundStyle(BlockPalette.monoBg)
                .padding(.vertical, 4)
                .padding(.horizontal, 8)
                .background(BlockPalette.accentInk, in: Capsule())
            if let summary, !summary.isEmpty {
                MarkdownText(summary)
                    .font(.subheadline)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10).strokeBorder(BlockPalette.line)
        )
    }
}

/// WaitingPanelView fills the gap between rounds, recalling how the last round
/// closed. Mirrors web/src/components/WaitingPanel.tsx.
private struct WaitingPanelView: View {
    let round: Int
    let lastRound: RoundRecord?

    var body: some View {
        VStack(spacing: 8) {
            ProgressView()
            Text("Waiting for round \(round)")
                .font(.headline)
                .foregroundStyle(BlockPalette.ink)
            Text(subline)
                .font(.subheadline)
                .foregroundStyle(BlockPalette.muted)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 40)
    }

    private var subline: String {
        guard let lastRound else {
            return "Waiting for the agent to add content"
        }
        if let revision = lastRound.submittedRevision {
            return "Round \(lastRound.number) submitted · rev \(revision)"
        }
        return "Round \(lastRound.number) wrapped up"
    }
}

/// BoardSkeletonView is the pre-caught-up placeholder shown until the replay flushes
/// so live content never flickers over it. Mirrors web/src/components/BoardSkeleton.tsx.
private struct BoardSkeletonView: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            RoundedRectangle(cornerRadius: 8)
                .fill(BlockPalette.chipBg)
                .frame(height: 32)
                .frame(maxWidth: 220)
            ForEach(0 ..< 3, id: \.self) { _ in
                RoundedRectangle(cornerRadius: 10)
                    .fill(BlockPalette.chipBg)
                    .frame(height: 96)
            }
            Spacer()
        }
        .padding()
        .redacted(reason: .placeholder)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
    }
}
