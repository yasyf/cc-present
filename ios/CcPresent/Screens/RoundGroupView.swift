import CcPresentKit
import SwiftUI

/// RoundGroupView renders one closed round as a collapsible, read-only history
/// entry: a one-line header carrying the round number, title, block count, and a
/// tally of chips (approved, rejected, picks, notes), disclosing the frozen blocks
/// beneath. Mirrors web/src/components/RoundGroup.tsx.
///
/// The frozen blocks render through the same interactive block views the live board
/// uses, driven by a private store seeded from the record's snapshot via the public
/// optimistic methods (the pattern the block previews already use). The whole
/// disclosed subtree is `.disabled` and dimmed, the native analogue of the web
/// read-only group — the seed is display state, never a live interaction.
struct RoundGroupView: View {
    let record: RoundRecord
    var client: APIClient?
    var packContext: PackContext?

    @State private var store: BoardStore
    @State private var expanded = false

    init(record: RoundRecord, client: APIClient? = nil, packContext: PackContext? = nil) {
        self.record = record
        self.client = client
        self.packContext = packContext
        _store = State(initialValue: RoundGroupView.seed(record))
    }

    private var tally: RoundTally {
        roundTally(record)
    }

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            VStack(alignment: .leading, spacing: 16) {
                ForEach(record.blocks, id: \.id) { block in
                    BlockView(block: block, store: store, client: client, packContext: packContext)
                }
            }
            .padding(.top, 12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .disabled(true)
            .opacity(0.55)
        } label: {
            header
        }
        .tint(BlockPalette.muted)
    }

    private var header: some View {
        HStack(spacing: 12) {
            Text(headerTitle)
                .font(.caption)
                .fontWeight(.semibold)
                .foregroundStyle(BlockPalette.ink)
                .monospacedDigit()
                .lineLimit(1)
            Spacer(minLength: 8)
            HStack(spacing: 4) {
                if tally.approved > 0 {
                    RoundChip(text: "✓ \(tally.approved)")
                }
                if tally.rejected > 0 {
                    RoundChip(text: "✗ \(tally.rejected)")
                }
                if tally.picks > 0 {
                    RoundChip(text: "\(tally.picks) \(tally.picks == 1 ? "pick" : "picks")")
                }
                if tally.notes > 0 {
                    RoundChip(text: "\(tally.notes) \(tally.notes == 1 ? "note" : "notes")")
                }
            }
        }
    }

    private var headerTitle: String {
        var title = "Round \(record.number)"
        if !record.title.isEmpty {
            title += " · \(record.title)"
        }
        title += " · \(record.blocks.count) blocks"
        if record.submittedRevision != nil {
            title += " · submitted"
        }
        return title
    }

    /// seed builds the read-only store backing a history round by replaying the
    /// record's frozen interactions through the optimistic engine. Replies are not
    /// part of a RoundRecord and are never seeded here; a late agent reply threads
    /// back into a history block through the blockReplies environment BoardScreen
    /// injects at the board root, live-spliced beneath the frozen block.
    @MainActor
    private static func seed(_ record: RoundRecord) -> BoardStore {
        let store = BoardStore(subject: "history", transport: ReadOnlyPoster())
        for block in flatten(record.blocks) {
            if let decision = record.decisions[block.id], let verdict = Verdict(rawValue: decision.verdict) {
                store.decide(blockId: block.id, verdict: verdict)
            }
            if let selection = record.choices[block.id], !selection.optionIds.isEmpty {
                store.choose(blockId: block.id, optionIds: selection.optionIds)
            }
            if let input = record.inputs[block.id] {
                store.submitInput(blockId: block.id, text: input.text)
            }
            for entry in record.feedback[block.id] ?? [] {
                store.feedback(blockId: block.id, text: entry.text)
            }
        }
        return store
    }
}

/// RoundChip is one pill in a round header's tally row.
private struct RoundChip: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.caption2)
            .monospacedDigit()
            .foregroundStyle(BlockPalette.muted)
            .padding(.vertical, 2)
            .padding(.horizontal, 8)
            .overlay(
                Capsule().strokeBorder(BlockPalette.line)
            )
            .lineLimit(1)
    }
}

/// ReadOnlyPoster is the inert poster the history store seeds through: the disclosed
/// subtree is disabled, so no interaction ever reaches it, and the seed itself only
/// needs the optimistic overlay the store applies before the POST.
private struct ReadOnlyPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        0
    }
}

#Preview("Round group") {
    ScrollView {
        VStack(alignment: .leading, spacing: 16) {
            RoundGroupView(
                record: RoundRecord(
                    number: 1,
                    title: "First pass",
                    blocks: [
                        .approval(Block.Approval(id: "ap1", prompt: "Approve the rename?")),
                        .choice(Block.Choice(
                            id: "ch1",
                            prompt: "Merge strategy?",
                            options: [Block.Option(id: "o1", label: "Rebase"), Block.Option(id: "o2", label: "Squash")]
                        )),
                    ],
                    decisions: ["ap1": Decision(verdict: "approved")],
                    choices: ["ch1": Selection(optionIds: ["o1"])],
                    feedback: ["ap1": [Feedback(id: "f1", text: "Ship it.")]],
                    submittedRevision: 2
                )
            )
        }
        .padding()
    }
}
