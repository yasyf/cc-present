import CcPresentKit
import SwiftUI

/// CardView renders a card: a left rail whose color encodes review state, a head
/// (title, status chip, right-aligned chips), an optional summary, a flag callout,
/// then one nesting level of children through BlockView. Mirrors
/// web/src/components/Card.tsx, including the rail cascade
/// (decided reject > decided approve > flagged > has-approval accent > neutral).
struct CardView: View {
    let block: Block.Card
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.inFocusCard) private var inFocusCard

    private var flagged: Bool {
        block.flagged == true
    }

    var body: some View {
        HStack(alignment: .top, spacing: 0) {
            RoundedRectangle(cornerRadius: 1.5)
                .fill(railColor)
                .frame(width: 3)
            VStack(alignment: .leading, spacing: 12) {
                head
                if let summary = block.summary, !summary.isEmpty {
                    MarkdownText(summary)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.muted)
                        .receiptContent()
                }
                if flagged {
                    flagCallout
                }
                VStack(alignment: .leading, spacing: 16) {
                    ForEach(block.children, id: \.id) { child in
                        BlockView(block: child, store: store, client: client, packContext: packContext)
                    }
                }
            }
            .padding(.leading, 16)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private var head: some View {
        // In a focus step the title, status, and chips are hoisted into the deck meta
        // row, so the in-card head would duplicate them.
        if !inFocusCard, block.title != nil || block.status != nil || !(block.chips ?? []).isEmpty {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                if let title = block.title, !title.isEmpty {
                    Text(title)
                        .font(.headline)
                        .foregroundStyle(BlockPalette.ink)
                }
                if let status = block.status, !status.isEmpty {
                    StatusChipView(status: status)
                }
                Spacer(minLength: 8)
                if let chips = block.chips, !chips.isEmpty {
                    HStack(spacing: 4) {
                        ForEach(Array(chips.enumerated()), id: \.offset) { _, chip in
                            ChipView(chip: chip)
                        }
                    }
                }
            }
        }
    }

    private var flagCallout: some View {
        Text("Flagged for review")
            .font(.subheadline)
            .foregroundStyle(BlockPalette.reject)
            .padding(.vertical, 8)
            .padding(.horizontal, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(BlockPalette.reject.opacity(0.12))
            .overlay(alignment: .leading) {
                Rectangle()
                    .fill(BlockPalette.reject)
                    .frame(width: 3)
            }
            .clipShape(RoundedRectangle(cornerRadius: 4))
    }

    /// railColor resolves the left-rail tint from the card's approval children,
    /// following the web `:has()` cascade: a rejected verdict wins over an approved
    /// one, which wins over the flag, which wins over the accent an undecided
    /// approval carries, which wins over the neutral default.
    private var railColor: Color {
        let verdicts = block.children.compactMap { child -> Verdict? in
            guard case let .approval(approval) = child else { return nil }
            return store.state.interactions.decisions[approval.id].flatMap { Verdict(rawValue: $0.verdict) }
        }
        if verdicts.contains(.rejected) {
            return BlockPalette.reject
        }
        if verdicts.contains(.approved) {
            return BlockPalette.approve
        }
        if flagged {
            return BlockPalette.reject
        }
        let hasApproval = block.children.contains {
            if case .approval = $0 {
                true
            } else {
                false
            }
        }
        return hasApproval ? BlockPalette.accentInk : BlockPalette.line
    }
}

/// StatusChipView renders a card's agent-owned status pill (`open`, `resolved`,
/// `redrafted`), tinting resolved green and the rest accent, per the web
/// `.status-*` treatment.
private struct StatusChipView: View {
    let status: String

    var body: some View {
        Text(status)
            .font(.caption2)
            .foregroundStyle(tint)
            .padding(.vertical, 2)
            .padding(.horizontal, 8)
            .overlay(
                Capsule().strokeBorder(tint.opacity(0.45))
            )
    }

    private var tint: Color {
        status == "resolved" ? BlockPalette.approve : BlockPalette.accentInk
    }
}

/// ChipView renders one card chip, toning `flag` red, `demo` accent, and the
/// default muted, per the web `.chip-*` treatment.
private struct ChipView: View {
    let chip: Block.Chip

    var body: some View {
        Text(chip.label)
            .font(.caption2)
            .foregroundStyle(foreground)
            .padding(.vertical, 2)
            .padding(.horizontal, 8)
            .background(background, in: Capsule())
            .overlay(
                Capsule().strokeBorder(border)
            )
            .lineLimit(1)
    }

    private var foreground: Color {
        switch chip.tone {
        case "flag": BlockPalette.reject
        case "demo": BlockPalette.accentInk
        default: BlockPalette.muted
        }
    }

    private var background: Color {
        chip.tone == "flag" ? BlockPalette.reject.opacity(0.12) : BlockPalette.chipBg
    }

    private var border: Color {
        chip.tone == "flag" ? BlockPalette.reject.opacity(0.4) : BlockPalette.line
    }
}

private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

@MainActor
private func previewStore() -> BoardStore {
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    store.decide(blockId: "ap-card", verdict: .approved)
    return store
}

#Preview("Card") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            CardView(
                block: Block.Card(
                    id: "card-1",
                    title: "Rename the reducer helper",
                    summary: "Threads the round number through `applyOrdered` before merging.",
                    chips: [Block.Chip(label: "refactor"), Block.Chip(label: "demo", tone: "demo")],
                    status: "open",
                    children: [
                        .markdown(Block.Markdown(id: "md-card", md: "Two call sites change; both covered.")),
                        .approval(Block.Approval(id: "ap-card", prompt: "Approve the rename?")),
                    ]
                ),
                store: previewStore()
            )

            CardView(
                block: Block.Card(
                    id: "card-2",
                    title: "Risky migration",
                    chips: [Block.Chip(label: "blocking", tone: "flag")],
                    flagged: true,
                    status: "redrafted",
                    children: [
                        .choice(Block.Choice(
                            id: "ch-card",
                            prompt: "How should we roll out?",
                            options: [
                                Block.Option(id: "o1", label: "All at once"),
                                Block.Option(id: "o2", label: "Staged"),
                            ]
                        )),
                    ]
                ),
                store: BoardStore(subject: "preview", transport: PreviewPoster())
            )
        }
        .padding()
    }
}
