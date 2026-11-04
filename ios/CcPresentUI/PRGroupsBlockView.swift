import CcPresentKit
import IOSKitUI
import SwiftUI

struct PRGroupsBlockView: View {
    let block: Block.PRGroups
    let store: BoardStore
    var context: PackContext?

    @Environment(\.focusHeadlineId) private var focusHeadlineId
    @State private var open: Set<String> = []

    private var verdicts: [String: Decision] {
        store.state.interactions.triage[block.id] ?? [:]
    }

    private var isClosed: Bool {
        store.isClosed
    }

    private var waitingGroups: [Block.PRGroups.Group] {
        let verdicts = self.verdicts
        return block.groups.compactMap { group in
            var waiting = group
            waiting.prs = group.prs.filter { !Block.PRGroups.stands(verdicts[$0.id], for: $0) }
            return waiting.prs.isEmpty ? nil : waiting
        }
    }

    private var answered: [Block.PRGroups.Row] {
        let verdicts = self.verdicts
        return block.rows.filter { Block.PRGroups.stands(verdicts[$0.id], for: $0) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space4) {
            header
            ForEach(waitingGroups, id: \.id) { group in
                groupSection(group)
            }
            let answered = self.answered
            if !answered.isEmpty {
                answeredList(answered)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if focusHeadlineId != block.id, let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .fontWeight(.medium)
                    .foregroundStyle(Palette.ink)
                    .receiptContent()
            }
            Text(prGroupsTallyLine(Block.PRGroups.tally(block.rows, verdicts)))
                .font(.caption)
                .monospacedDigit()
                .foregroundStyle(Palette.muted)
        }
    }

    private func groupSection(_ group: Block.PRGroups.Group) -> some View {
        VStack(alignment: .leading, spacing: Metrics.space3) {
            VStack(alignment: .leading, spacing: Metrics.space1) {
                Text(group.title)
                    .voice(.prose, size: 15, weight: .semibold)
                    .foregroundStyle(Palette.ink)
                MarkdownText(group.summary)
                    .font(.subheadline)
                    .foregroundStyle(Palette.muted)
            }
            .receiptContent()
            HStack(spacing: Metrics.space2) {
                Text("\(group.prs.count) waiting on you")
                    .font(.caption)
                    .monospacedDigit()
                    .foregroundStyle(Palette.muted)
                Spacer(minLength: Metrics.space2)
                if !isClosed {
                    Button("Approve all in this group") {
                        store.triageDecide(blockId: block.id, verdicts: Block.PRGroups.approveGroup(group, verdicts))
                    }
                    .buttonStyle(GhostButtonStyle(tint: Palette.approve))
                }
            }
            ForEach(group.prs) { row in
                waitingRow(row)
            }
        }
    }

    private func waitingRow(_ row: Block.PRGroups.Row) -> some View {
        let decision = verdicts[row.id]
        let stale: Decision? = if case let .stale(given) = Block.PRGroups.rowVerdict(row, decision) { given } else { nil }
        let expanded = open.contains(row.id)
        return VStack(alignment: .leading, spacing: Metrics.space3) {
            Button {
                open.formSymmetricDifference([row.id])
            } label: {
                VStack(alignment: .leading, spacing: Metrics.space1) {
                    HStack(spacing: Metrics.space2) {
                        Text("#\(row.number)")
                            .voice(.mono, size: 12, weight: .semibold)
                            .foregroundStyle(Palette.accentInk)
                        RowStateLabel(text: stale == nil ? "Waiting on you" : "Needs your tap", tint: stale == nil ? Palette.muted : Palette.warn)
                        Spacer(minLength: Metrics.space2)
                        Image(systemName: expanded ? "chevron.down" : "chevron.right")
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(Palette.muted)
                    }
                    Text(row.title)
                        .font(.body)
                        .fontWeight(.semibold)
                        .foregroundStyle(Palette.ink)
                        .multilineTextAlignment(.leading)
                }
                .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            // A row's drill-down is read-only, so it stays live inside a history round's
            // `.disabled(true)` subtree, as DetailView's does.
            .environment(\.isEnabled, true)

            VStack(alignment: .leading, spacing: Metrics.space2) {
                rowField("What changes", row.change)
                rowField("Against the plan", row.plan)
                rowField("Risk", row.risk)
                rowField("Review and CI", row.review)
                rowField("Blocks landing", row.blocker)
            }
            .receiptContent()

            if let stack = row.stack, !stack.isEmpty {
                Text("One tap also decides \(stack.map { "#\($0.number) at \(Block.PRGroups.shortHead($0.head))" }.joined(separator: ", "))")
                    .font(.subheadline)
                    .fontWeight(.semibold)
                    .foregroundStyle(Palette.ink)
            }

            if let stale {
                Text(Block.PRGroups.staleLine(row, stale))
                    .font(.subheadline)
                    .fontWeight(.semibold)
                    .foregroundStyle(Palette.ink)
                    .padding(.horizontal, Metrics.space3)
                    .padding(.vertical, Metrics.space2)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Palette.warn.opacity(0.14), in: RoundedRectangle(cornerRadius: Metrics.radiusSm))
            }

            HStack(spacing: Metrics.space3) {
                verdictButton(row, decision: decision, target: "approved", label: "Approve", glyph: "checkmark", tint: Palette.approve)
                verdictButton(row, decision: decision, target: "rejected", label: "Hold", glyph: "pause", tint: Palette.warn)
            }
            .accessibilityElement(children: .contain)
            .accessibilityLabel("#\(row.number) \(row.title)")

            if let note = decision?.note, !note.isEmpty {
                RowNote(text: note)
            }

            if expanded {
                VStack(alignment: .leading, spacing: Metrics.space3) {
                    if let md = row.md, !md.isEmpty {
                        MarkdownText(md, style: .clamped)
                    }
                    PRBlockView(
                        block: Block.PR(id: block.id, repo: block.repo, number: row.number, files: row.files),
                        context: context
                    )
                }
            }
        }
        .padding(Metrics.space3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(stale == nil ? Color.clear : Palette.warn.opacity(0.06), in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).strokeBorder(Palette.line))
    }

    private func rowField(_ label: String, _ markdown: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label)
                .voice(.stamp, size: 10, weight: .semibold)
                .foregroundStyle(Palette.muted)
            MarkdownText(markdown)
                .font(.subheadline)
                .foregroundStyle(Palette.ink)
        }
    }

    private func verdictButton(
        _ row: Block.PRGroups.Row,
        decision: Decision?,
        target: String,
        label: String,
        glyph: String,
        tint: Color
    ) -> some View {
        Button {
            store.triageDecide(blockId: block.id, verdicts: [row.id: Block.PRGroups.toggleEntry(row, decision, target: target)])
        } label: {
            VerdictLabel(glyph: glyph, title: label)
        }
        .buttonStyle(VerdictButtonStyle(tint: tint, active: false))
        .disabled(isClosed)
        .accessibilityLabel(label)
    }

    private func answeredList(_ rows: [Block.PRGroups.Row]) -> some View {
        DisclosureGroup {
            VStack(alignment: .leading, spacing: Metrics.space3) {
                ForEach(rows) { row in
                    answeredRow(row)
                }
            }
            .padding(.top, Metrics.space2)
        } label: {
            Text(prGroupsAnsweredLine(Block.PRGroups.tally(rows, verdicts)))
                .font(.subheadline)
                .fontWeight(.semibold)
                .monospacedDigit()
                .foregroundStyle(Palette.muted)
        }
        .tint(Palette.muted)
        .environment(\.isEnabled, true)
    }

    @ViewBuilder
    private func answeredRow(_ row: Block.PRGroups.Row) -> some View {
        if case let .current(decision, carried) = Block.PRGroups.rowVerdict(row, verdicts[row.id]) {
            let approved = decision.verdict == "approved"
            VStack(alignment: .leading, spacing: Metrics.space1) {
                HStack(spacing: Metrics.space2) {
                    Text("#\(row.number)")
                        .voice(.mono, size: 12, weight: .semibold)
                        .foregroundStyle(Palette.accentInk)
                    RowStateLabel(text: approved ? "Approved" : "Held", tint: approved ? Palette.approve : Palette.warn)
                }
                Text(row.title)
                    .font(.subheadline)
                    .fontWeight(.semibold)
                    .foregroundStyle(Palette.ink)
                Text(Block.PRGroups.givenLine(row, decision, carried: carried))
                    .voice(.mono, size: 11)
                    .foregroundStyle(Palette.muted)
                if let note = decision.note, !note.isEmpty {
                    RowNote(text: note)
                }
            }
            .receiptContent()
        }
    }
}

private struct RowStateLabel: View {
    let text: String
    let tint: Color

    var body: some View {
        Text(text)
            .voice(.stamp, size: 10, weight: .semibold)
            .foregroundStyle(tint)
            .padding(.horizontal, 6)
            .padding(.vertical, 1)
            .background(tint.opacity(0.1), in: RoundedRectangle(cornerRadius: Metrics.radiusSm))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusSm).strokeBorder(tint.opacity(0.45)))
            .lineLimit(1)
    }
}

private struct RowNote: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.subheadline)
            .foregroundStyle(Palette.ink)
            .padding(.leading, Metrics.space3)
            .overlay(alignment: .leading) {
                Rectangle()
                    .fill(Palette.borderStrong)
                    .frame(width: 2)
            }
    }
}

func prGroupsTallyLine(_ tally: Block.PRGroups.Tally) -> String {
    var parts = ["\(tally.approved) approved"]
    if tally.held > 0 {
        parts.append("\(tally.held) held")
    }
    if tally.open > 0 {
        parts.append("\(tally.open) waiting on you")
    }
    return parts.joined(separator: " · ")
}

func prGroupsAnsweredLine(_ tally: Block.PRGroups.Tally) -> String {
    tally.held > 0 ? "\(tally.approved) approved, landing · \(tally.held) held" : "\(tally.approved) approved, landing"
}
