import CcPresentKit
import SwiftUI

/// DraftBlockView renders a draft as numbered monospaced lines a human annotates by
/// content anchor. Annotations resolve against the current text (exact, moved, or
/// detached); a tap-tap on line numbers posts a ranged anchor. Mirrors DraftView.tsx.
struct DraftBlockView: View {
    let block: Block.Draft
    let store: BoardStore

    @State private var selection: DraftSelection = .idle
    @State private var composer: DraftComposerTarget?

    private var lines: [String] {
        block.text.components(separatedBy: "\n")
    }

    private var annotations: [Annotation] {
        store.state.interactions.annotations[block.id] ?? []
    }

    private var isClosed: Bool {
        store.isClosed
    }

    /// resolved pairs each annotation with its resolution against the current lines —
    /// nil when the anchored content is gone (a detached note).
    private var resolved: [(annotation: Annotation, resolution: CcPresentKit.Anchor.Resolution?)] {
        annotations.map { ($0, resolve($0)) }
    }

    /// markersByLine groups annotations by the line their anchor resolves to, so each
    /// resolved note renders beneath its start line.
    private var markersByLine: [Int: [Annotation]] {
        var out: [Int: [Annotation]] = [:]
        for entry in resolved {
            if let resolution = entry.resolution {
                out[resolution.start, default: []].append(entry.annotation)
            }
        }
        return out
    }

    private var detached: [Annotation] {
        resolved.filter { $0.resolution == nil }.map(\.annotation)
    }

    private var gutterWidth: CGFloat {
        CGFloat(max(2, String(max(lines.count, 1)).count)) * 9 + 6
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            header
            if case let .anchored(line) = selection {
                selectionHint(line)
            }
            LazyVStack(alignment: .leading, spacing: 0) {
                ForEach(Array(lines.enumerated()), id: \.offset) { index, text in
                    lineRow(number: index + 1, text: text)
                    ForEach(markersByLine[index + 1] ?? [], id: \.id) { annotation in
                        annotationCard(annotation)
                    }
                }
            }
            .padding(.vertical, 8)
            .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).strokeBorder(BlockPalette.line))

            if !detached.isEmpty {
                detachedSection
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .sheet(item: $composer) { target in
            DraftComposerSheet(target: target, quote: target.quote) { text in
                store.annotate(
                    id: target.id,
                    blockId: block.id,
                    anchor: target.anchor,
                    text: text,
                    quote: target.quote
                )
            }
        }
    }

    // MARK: - Header

    @ViewBuilder
    private var header: some View {
        let count = annotations.count
        if !(block.title ?? "").isEmpty || count > 0 {
            HStack(spacing: 8) {
                if let title = block.title, !title.isEmpty {
                    Text(title)
                        .voice(.mono, size: 13, weight: .semibold)
                        .foregroundStyle(BlockPalette.ink)
                        .lineLimit(1)
                }
                Spacer(minLength: 8)
                if count > 0 {
                    Text("\(count) \(count == 1 ? "note" : "notes")")
                        .font(.caption2)
                        .monospacedDigit()
                        .foregroundStyle(BlockPalette.muted)
                }
            }
            .receiptContent()
        }
    }

    private func selectionHint(_ line: Int) -> some View {
        Text("Tap another line number to annotate from line \(line), or tap \(line) again to cancel.")
            .font(.caption2)
            .foregroundStyle(BlockPalette.accentInk)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    // MARK: - Lines

    private func lineRow(number: Int, text: String) -> some View {
        let anchored = selection == .anchored(number)
        return HStack(alignment: .top, spacing: 8) {
            Button {
                tapLine(number)
            } label: {
                Text("\(number)")
                    .voice(.mono, size: 13)
                    .monospacedDigit()
                    .foregroundStyle(anchored ? BlockPalette.accentFg : BlockPalette.muted)
                    .frame(width: gutterWidth, alignment: .trailing)
                    .padding(.vertical, 1)
                    .background(anchored ? BlockPalette.accentInk : Color.clear)
            }
            .buttonStyle(.plain)
            .disabled(isClosed)
            .accessibilityLabel("Line \(number)")
            .accessibilityHint(isClosed ? "" : "Tap to start or extend an annotation")

            Text(text.isEmpty ? " " : text)
                .voice(.mono, size: 13)
                .foregroundStyle(BlockPalette.ink)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 1)
    }

    private func annotationCard(_ annotation: Annotation) -> some View {
        let resolution = resolve(annotation)
        return VStack(alignment: .leading, spacing: 6) {
            if let resolution, resolution.moved {
                Text("moved · was L\(resolution.from)")
                    .voice(.mono, size: 10, weight: .semibold)
                    .textCase(.uppercase)
                    .foregroundStyle(BlockPalette.warn)
            }
            Text(annotation.text)
                .font(.subheadline)
                .foregroundStyle(BlockPalette.ink)
                .frame(maxWidth: .infinity, alignment: .leading)
            if !isClosed {
                HStack(spacing: 14) {
                    Button("Edit") { edit(annotation) }
                        .foregroundStyle(BlockPalette.accentInk)
                    Button("Remove") { store.removeAnnotation(id: annotation.id, blockId: block.id) }
                        .foregroundStyle(BlockPalette.reject)
                }
                .voice(.prose, size: 12, weight: .semibold)
                .buttonStyle(.plain)
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.accentInk.opacity(0.08), in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .padding(.leading, gutterWidth + 18)
        .padding(.trailing, 10)
        .padding(.vertical, 3)
    }

    private var detachedSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Detached notes")
                .voice(.mono, size: 10, weight: .semibold)
                .textCase(.uppercase)
                .tracking(0.6)
                .foregroundStyle(BlockPalette.muted)
            ForEach(detached, id: \.id) { annotation in
                VStack(alignment: .leading, spacing: 6) {
                    Text(annotation.quote)
                        .voice(.mono, size: 12)
                        .foregroundStyle(BlockPalette.was)
                        .lineLimit(3)
                    Text(annotation.text)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.ink)
                    if !isClosed {
                        Button("Remove") { store.removeAnnotation(id: annotation.id, blockId: block.id) }
                            .voice(.prose, size: 12, weight: .semibold)
                            .buttonStyle(.plain)
                            .foregroundStyle(BlockPalette.reject)
                    }
                }
                .padding(10)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .receiptContent()
    }

    // MARK: - Intent

    private func resolve(_ annotation: Annotation) -> CcPresentKit.Anchor.Resolution? {
        guard let ref = try? CcPresentKit.Anchor.parse(annotation.anchor) else { return nil }
        return try? CcPresentKit.Anchor.resolve(ref, lines: lines)
    }

    private func tapLine(_ number: Int) {
        guard !isClosed else { return }
        switch advanceDraftSelection(selection, tapped: number) {
        case let .selecting(next):
            selection = next
        case let .commit(start, end):
            selection = .idle
            composer = DraftComposerTarget(
                id: UUID().uuidString,
                anchor: rangedAnchor(start: start, end: end),
                quote: quote(start: start, end: end),
                text: "",
                isEdit: false
            )
        }
    }

    private func edit(_ annotation: Annotation) {
        let range = resolve(annotation)
        let quoteText = range.map { quote(start: $0.start, end: $0.end) } ?? annotation.quote
        composer = DraftComposerTarget(
            id: annotation.id,
            anchor: annotation.anchor,
            quote: quoteText,
            text: annotation.text,
            isEdit: true
        )
    }

    /// rangedAnchor formats the ranged anchor a new selection posts: the inclusive span
    /// hashed on the start line only.
    private func rangedAnchor(start: Int, end: Int) -> String {
        CcPresentKit.Anchor.formatRange(start: start, end: end, hash: CcPresentKit.Anchor.of(lines[start - 1]))
    }

    /// quote joins the selected lines into the advisory quote; the daemon re-stamps it.
    private func quote(start: Int, end: Int) -> String {
        lines[(start - 1) ..< min(end, lines.count)].joined(separator: "\n")
    }
}

/// DraftSelection is the draft's line-selection cursor between taps: idle, or anchored
/// on a first tapped line awaiting the second that closes the range.
enum DraftSelection: Equatable {
    case idle
    case anchored(Int)
}

/// DraftSelectionOutcome is what a line-number tap produces: a new cursor state (still
/// selecting or cancelled) or a committed inclusive range to annotate.
enum DraftSelectionOutcome: Equatable {
    case selecting(DraftSelection)
    case commit(start: Int, end: Int)
}

/// advanceDraftSelection is the pure tap-tap normalization: the first tap anchors,
/// re-tapping the same line cancels, and a second distinct line commits the inclusive
/// range (min…max) so a backwards sweep reads like a forwards one.
func advanceDraftSelection(_ state: DraftSelection, tapped line: Int) -> DraftSelectionOutcome {
    switch state {
    case .idle:
        .selecting(.anchored(line))
    case let .anchored(first):
        first == line ? .selecting(.idle) : .commit(start: min(first, line), end: max(first, line))
    }
}

/// DraftComposerTarget is the note the composer sheet edits: the id it posts under, the
/// ranged anchor, the advisory quote, and the initial text.
struct DraftComposerTarget: Identifiable {
    let id: String
    let anchor: String
    let quote: String
    var text: String
    let isEdit: Bool
}

/// DraftComposerSheet is the modal note editor: the anchored quote for context above a
/// text field, committing on Save.
private struct DraftComposerSheet: View {
    let target: DraftComposerTarget
    let quote: String
    let onSave: (String) -> Void

    @State private var text: String
    @FocusState private var focused: Bool
    @Environment(\.dismiss) private var dismiss

    init(target: DraftComposerTarget, quote: String, onSave: @escaping (String) -> Void) {
        self.target = target
        self.quote = quote
        self.onSave = onSave
        _text = State(initialValue: target.text)
    }

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 12) {
                if !quote.isEmpty {
                    Text(quote)
                        .voice(.mono, size: 12)
                        .foregroundStyle(BlockPalette.muted)
                        .lineLimit(4)
                        .padding(10)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
                }
                TextField("Add a note on these lines…", text: $text, axis: .vertical)
                    .lineLimit(3 ... 8)
                    .font(.body)
                    .focused($focused)
                    .accessibilityLabel("Annotation text")
                Spacer()
            }
            .padding(16)
            .navigationTitle(target.isEdit ? "Edit note" : "Add note")
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
    store.annotate(
        id: "an1",
        blockId: "d1",
        anchor: "2-2#\(CcPresentKit.Anchor.of("A claim that needs support."))",
        text: "Needs a citation before this ships.",
        quote: "A claim that needs support."
    )
    return store
}

#Preview("Draft") {
    ScrollView {
        DraftBlockView(
            block: Block.Draft(
                id: "d1",
                lang: "markdown",
                text: "Intro paragraph.\nA claim that needs support.\nClosing remarks.",
                title: "proposal.md"
            ),
            store: previewStore()
        )
        .padding()
    }
}
