import CcPresentKit
import SwiftUI

/// SubmitBarView is the board's pinned bottom bar: a decided/total tally over the
/// current round's approvals and choices, an optional round marker and submit note,
/// and the submit button. When undecided approvals remain, the button first arms a
/// confirmation dialog before it posts `Interaction.submit`. Mirrors
/// web/src/components/SubmitBar.tsx. The bar renders nothing when there is nothing
/// to tally and no configured submit control.
struct SubmitBarView: View {
    let blocks: [Block]
    let doc: Doc
    let store: BoardStore
    let hasHistory: Bool

    @State private var confirming = false

    private var interactions: Interactions {
        store.state.interactions
    }

    private var items: [SubmitItem] {
        submitItems(blocks, interactions)
    }

    private var total: Int {
        items.count
    }

    private var decided: Int {
        items.filter(\.decided).count
    }

    private var undecidedApprovals: Int {
        items.filter { $0.kind == .approval && !$0.decided }.count
    }

    private var label: String {
        doc.submit?.label ?? "Submit"
    }

    private var hidden: Bool {
        total == 0 && doc.submit == nil
    }

    var body: some View {
        if hidden {
            EmptyView()
        } else {
            bar
        }
    }

    private var bar: some View {
        HStack(spacing: 16) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 8) {
                    if hasHistory {
                        Text("Round \(store.state.rounds.current)")
                            .font(.caption2)
                            .fontWeight(.semibold)
                            .foregroundStyle(BlockPalette.accentInk)
                            .monospacedDigit()
                    }
                    if total > 0 {
                        Text("\(decided)/\(total) decided")
                            .font(.caption2)
                            .fontWeight(.bold)
                            .monospacedDigit()
                            .foregroundStyle(decided == total ? BlockPalette.approve : BlockPalette.muted)
                    }
                }
                if let note = doc.submit?.note, !note.isEmpty {
                    Text(note)
                        .font(.caption2)
                        .foregroundStyle(BlockPalette.muted)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 8)
            Button(label, action: attemptSubmit)
                .buttonStyle(.borderedProminent)
                .tint(BlockPalette.accentInk)
                .disabled(store.isClosed)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .background(.bar)
        .overlay(alignment: .top) {
            Rectangle()
                .fill(BlockPalette.accentInk)
                .frame(height: 2)
        }
        .confirmationDialog(
            "\(undecidedApprovals) \(undecidedApprovals == 1 ? "approval" : "approvals") still undecided",
            isPresented: $confirming,
            titleVisibility: .visible
        ) {
            Button("Submit anyway?") { submit() }
            Button("Cancel", role: .cancel) {}
        }
    }

    private func attemptSubmit() {
        guard !hidden, !store.isClosed else { return }
        if undecidedApprovals > 0 {
            confirming = true
        } else {
            submit()
        }
    }

    /// submit posts against revision 0. BoardStore does not surface the live document
    /// revision (it drops `doc.replaced`'s transport metadata), and the REST plane
    /// accepts any revision in `[0, current]`, so 0 is always valid.
    private func submit() {
        store.submit(revision: 0)
    }
}

private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

#Preview("Submit bar") {
    let blocks: [Block] = [
        .approval(Block.Approval(id: "ap1", prompt: "Approve?")),
        .approval(Block.Approval(id: "ap2", prompt: "And this?")),
        .choice(Block.Choice(id: "ch1", prompt: "Pick", options: [Block.Option(id: "o1", label: "A")])),
    ]
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    store.decide(blockId: "ap1", verdict: .approved)

    return VStack {
        Spacer()
        SubmitBarView(
            blocks: blocks,
            doc: Doc(title: "Review", submit: Doc.Submit(label: "Submit review", note: "2 files"), blocks: blocks),
            store: store,
            hasHistory: true
        )
    }
}
