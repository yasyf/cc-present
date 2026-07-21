import CcPresentKit
import SwiftUI

/// TriageBlockView renders a triage block: a list of items each with a compact
/// Approve/Reject pair (re-pressing the active verdict clears it), bulk Accept all /
/// Reject all, and an optional per-item note. Verdicts are the per-item last-write-wins
/// map in BoardState. Mirrors Triage.tsx; a closed board renders read-only.
struct TriageBlockView: View {
    let block: Block.Triage
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.focusHeadlineId) private var focusHeadlineId
    @State private var noteTarget: TriageNoteTarget?

    private var verdicts: [String: Decision] {
        store.state.interactions.triage[block.id] ?? [:]
    }

    private var allowNotes: Bool {
        block.allowNotes ?? true
    }

    private var isClosed: Bool {
        store.isClosed
    }

    private var decidedCount: Int {
        block.items.filter { verdicts[$0.id] != nil }.count
    }

    private var suppressPrompt: Bool {
        focusHeadlineId == block.id
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space3) {
            header
            VStack(spacing: Metrics.space2) {
                ForEach(block.items, id: \.id) { item in
                    itemRow(item)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .sheet(item: $noteTarget) { target in
            TriageNoteSheet(target: target) { text in
                store.triageDecide(
                    blockId: block.id,
                    verdicts: [target.itemId: TriageVerdict(verdict: target.verdict, note: text)]
                )
            }
        }
    }

    // MARK: - Header

    private var header: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if !suppressPrompt, let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .fontWeight(.medium)
                    .foregroundStyle(BlockPalette.ink)
                    .receiptContent()
            }
            HStack(spacing: Metrics.space2) {
                Text("\(decidedCount) of \(block.items.count) decided")
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(BlockPalette.muted)
                Spacer(minLength: 8)
                if !isClosed {
                    Button("Accept all") { bulk("approved") }
                        .buttonStyle(GhostButtonStyle(tint: BlockPalette.approve))
                    Button("Reject all") { bulk("rejected") }
                        .buttonStyle(GhostButtonStyle(tint: BlockPalette.reject))
                }
            }
        }
    }

    // MARK: - Rows

    private func itemRow(_ item: Block.Item) -> some View {
        let verdict = verdicts[item.id]?.verdict
        let note = verdicts[item.id]?.note
        return VStack(alignment: .leading, spacing: Metrics.space2) {
            HStack(alignment: .top, spacing: Metrics.space3) {
                VStack(alignment: .leading, spacing: Metrics.space1) {
                    Text(item.label)
                        .font(.body)
                        .fontWeight(.semibold)
                        .foregroundStyle(BlockPalette.ink)
                    if let hint = item.hint, !hint.isEmpty {
                        MarkdownText(hint)
                            .font(.caption2)
                            .foregroundStyle(BlockPalette.muted)
                    }
                    if let body = item.md, !body.isEmpty {
                        MarkdownText(body, style: .clamped)
                            .font(.subheadline)
                            .foregroundStyle(BlockPalette.muted)
                    }
                }
                Spacer(minLength: 12)
                if let facts = item.facts, !facts.isEmpty {
                    TriageFacts(facts: facts)
                }
            }

            if let detail = item.detail {
                DetailView(detail: detail)
            }

            if let visual = item.visual {
                TriageVisualDisclosure(visual: visual, context: packContext, client: client)
            }

            verdictPair(item, verdict: verdict, note: note)

            if allowNotes, let verdict, verdict != "cleared", !isClosed {
                noteAffordance(item, verdict: verdict, note: note)
            }
        }
        .padding(Metrics.space3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            verdict != nil ? BlockPalette.accentInk.opacity(0.06) : Color.clear,
            in: RoundedRectangle(cornerRadius: Metrics.radiusLg)
        )
        .overlay(
            RoundedRectangle(cornerRadius: Metrics.radiusLg).strokeBorder(BlockPalette.line)
        )
    }

    private func verdictPair(_ item: Block.Item, verdict: String?, note: String?) -> some View {
        HStack(spacing: Metrics.space3) {
            verdictButton(item, target: "approved", label: "Approve", glyph: "checkmark",
                          color: BlockPalette.approve, verdict: verdict, note: note)
            verdictButton(item, target: "rejected", label: "Reject", glyph: "xmark",
                          color: BlockPalette.reject, verdict: verdict, note: note)
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(item.label)
    }

    private func verdictButton(
        _ item: Block.Item,
        target: String,
        label: String,
        glyph: String,
        color: Color,
        verdict: String?,
        note: String?
    ) -> some View {
        let active = verdict == target
        return Button {
            store.triageDecide(
                blockId: block.id,
                verdicts: [item.id: triageFlip(current: verdict, note: note, target: target)]
            )
        } label: {
            VerdictLabel(glyph: glyph, title: label)
        }
        .buttonStyle(VerdictButtonStyle(tint: color, active: active))
        .disabled(isClosed)
        .accessibilityLabel(label)
        .accessibilityAddTraits(active ? [.isSelected] : [])
    }

    private func noteAffordance(_ item: Block.Item, verdict: String, note: String?) -> some View {
        Button((note ?? "").isEmpty ? "Add note" : "Edit note") {
            noteTarget = TriageNoteTarget(itemId: item.id, verdict: verdict, note: note ?? "")
        }
        .buttonStyle(GhostButtonStyle())
    }

    // MARK: - Intent

    private func bulk(_ target: String) {
        store.triageDecide(blockId: block.id, verdicts: triageBulk(items: block.items, verdicts: verdicts, target: target))
    }
}

/// triageFlip is the verdict a compact Approve/Reject press produces for one item:
/// re-pressing the active verdict clears it (dropping the note); the opposite switches
/// while carrying the current note forward. Pure, mirroring the web verdict toggle.
func triageFlip(current: String?, note: String?, target: String) -> TriageVerdict {
    current == target ? TriageVerdict(verdict: "cleared") : TriageVerdict(verdict: target, note: note)
}

/// triageBulk is the full-merge an Accept-all / Reject-all press produces: every item
/// set to `target`, each carrying its current note forward.
func triageBulk(items: [Block.Item], verdicts: [String: Decision], target: String) -> [String: TriageVerdict] {
    var out: [String: TriageVerdict] = [:]
    for item in items {
        out[item.id] = TriageVerdict(verdict: target, note: verdicts[item.id]?.note)
    }
    return out
}

/// TriageFacts renders an item's Tier-1 facts on the row's trailing edge: each value
/// prominent over its optional dim uppercase label. A compact peer of FactsCluster.
private struct TriageFacts: View {
    let facts: [Block.Fact]

    var body: some View {
        VStack(alignment: .trailing, spacing: 6) {
            ForEach(Array(facts.enumerated()), id: \.offset) { _, fact in
                VStack(alignment: .trailing, spacing: 1) {
                    Text(fact.value)
                        .font(.system(size: 14, weight: .semibold, design: .rounded))
                        .foregroundStyle(factTone(fact.tone))
                    if let label = fact.label, !label.isEmpty {
                        Text(label)
                            .voice(.stamp, size: 9, weight: .medium)
                            .foregroundStyle(BlockPalette.muted)
                    }
                }
            }
        }
        .accessibilityElement(children: .combine)
    }

    private func factTone(_ tone: String?) -> Color {
        switch tone {
        case "good": BlockPalette.approve
        case "bad": BlockPalette.reject
        case "warn": BlockPalette.warn
        default: BlockPalette.ink
        }
    }
}

/// TriageVisualDisclosure keeps a row compact: the item's restricted visual sits behind
/// a titled expander that reveals it inline, reusing the shared OptionVisualView.
private struct TriageVisualDisclosure: View {
    let visual: OptionVisual
    var context: PackContext?
    var client: APIClient?

    @State private var expanded = false

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            OptionVisualView(visual: visual, context: context, client: client)
                .padding(.top, Metrics.space2)
        } label: {
            Text(optionVisualTitle(visual))
                .voice(.mono, .caption)
                .foregroundStyle(BlockPalette.accentInk)
                .lineLimit(1)
        }
        .tint(BlockPalette.accentInk)
    }
}

/// TriageNoteTarget is the per-item note the sheet edits: the item it posts under, the
/// verdict the note rides on, and the initial text.
struct TriageNoteTarget: Identifiable {
    let itemId: String
    let verdict: String
    var note: String

    var id: String {
        itemId
    }
}

/// TriageNoteSheet is the modal per-item note editor, committing on Save.
private struct TriageNoteSheet: View {
    let target: TriageNoteTarget
    let onSave: (String) -> Void

    @State private var text: String
    @FocusState private var focused: Bool
    @Environment(\.dismiss) private var dismiss

    init(target: TriageNoteTarget, onSave: @escaping (String) -> Void) {
        self.target = target
        self.onSave = onSave
        _text = State(initialValue: target.note)
    }

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: Metrics.space3) {
                TextField("Add a note for the agent…", text: $text, axis: .vertical)
                    .lineLimit(3 ... 8)
                    .font(.body)
                    .focused($focused)
                    .accessibilityLabel("Item note")
                Spacer()
            }
            .padding(Metrics.space4)
            .navigationTitle("Note")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                        .tint(BlockPalette.muted)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .tint(BlockPalette.accentInk)
                        .disabled(text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
            .onAppear { focused = true }
        }
        .presentationDetents([.medium])
    }

    private func save() {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }
        onSave(trimmed)
        dismiss()
    }
}

// MARK: - Preview

private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

@MainActor
private func previewStore() -> BoardStore {
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    store.triageDecide(blockId: "t1", verdicts: [
        "i-oauth": TriageVerdict(verdict: "approved"),
        "i-flaky": TriageVerdict(verdict: "rejected", note: "flaky on CI, needs a repro first"),
    ])
    return store
}

#Preview("Triage") {
    ScrollView {
        TriageBlockView(
            block: Block.Triage(
                id: "t1",
                prompt: "Which cleanups ship this round?",
                items: [
                    Block.Item(
                        id: "i-oauth",
                        label: "Extract the OAuth refresh helper",
                        hint: "isolated, well-tested",
                        facts: [Block.Fact(label: "Risk", value: "low", tone: "good")]
                    ),
                    Block.Item(id: "i-flaky", label: "Re-enable the flaky integration test"),
                    Block.Item(id: "i-docs", label: "Rewrite the stale README section"),
                ]
            ),
            store: previewStore()
        )
        .padding()
    }
}
