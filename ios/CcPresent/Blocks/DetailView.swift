import CcPresentKit
import SwiftUI

/// DetailView renders a Tier-2 drill-down — the pros/cons tradeoffs and full
/// rationale a human needs to decide, hidden until opened. When the detail's mode
/// is `modal` it opens in a sheet overlay; `inline` (the default, also chosen when
/// mode is omitted) expands in place via a DisclosureGroup. It is the shared
/// affordance behind both a Choice option and an Approval prompt.
struct DetailView: View {
    let detail: Block.Detail

    @State private var expanded = false
    @State private var presenting = false

    private var modal: Bool {
        detail.mode == "modal"
    }

    var body: some View {
        content
            // A drill-down is read-only, so it stays live inside a history round's
            // `.disabled(true)` subtree; write the isEnabled environment key
            // directly to override the ancestor, matching MarkdownText's clamp toggle.
            .environment(\.isEnabled, true)
    }

    @ViewBuilder
    private var content: some View {
        if modal {
            Button {
                presenting = true
            } label: {
                affordanceLabel
            }
            .buttonStyle(.plain)
            .sheet(isPresented: $presenting) {
                DetailSheet(detail: detail)
            }
        } else {
            DisclosureGroup(isExpanded: $expanded) {
                DetailBody(detail: detail)
                    .padding(.top, 10)
            } label: {
                affordanceLabel
            }
            .tint(BlockPalette.accentInk)
        }
    }

    private var affordanceLabel: some View {
        Text("Details")
            .voice(.prose, size: 13, weight: .semibold)
            .foregroundStyle(BlockPalette.accentInk)
    }
}

/// DetailBody lays out a drill-down's contents: the pros list (green ✓), the cons
/// list (red ✗), then the full rationale markdown, each shown only when present.
private struct DetailBody: View {
    let detail: Block.Detail

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let pros = detail.pros, !pros.isEmpty {
                TradeoffList(items: pros, glyph: "checkmark", tint: BlockPalette.approve)
            }
            if let cons = detail.cons, !cons.isEmpty {
                TradeoffList(items: cons, glyph: "xmark", tint: BlockPalette.reject)
            }
            if let md = detail.md, !md.isEmpty {
                MarkdownText(md)
                    .font(.subheadline)
                    .foregroundStyle(BlockPalette.ink)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// TradeoffList renders one column of single-line tradeoff entries, each prefixed
/// by a tinted glyph (a check for pros, a cross for cons).
private struct TradeoffList: View {
    let items: [String]
    let glyph: String
    let tint: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Image(systemName: glyph)
                        .voice(.prose, size: 11, weight: .bold)
                        .foregroundStyle(tint)
                    Text(item)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.ink)
                        .fixedSize(horizontal: false, vertical: true)
                    Spacer(minLength: 0)
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}

/// DetailSheet is the modal surface for a `modal`-mode drill-down: the same body
/// contents inside a dismissable, detent-sized sheet.
private struct DetailSheet: View {
    let detail: Block.Detail

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ScrollView {
                DetailBody(detail: detail)
                    .padding(20)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .navigationTitle("Details")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                        .tint(BlockPalette.accentInk)
                }
            }
        }
        .presentationDetents([.medium, .large])
    }
}

#Preview("Detail — inline & modal") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            DetailView(
                detail: Block.Detail(
                    pros: ["Keeps history linear", "Easy to bisect"],
                    cons: ["Rewrites shared commits"],
                    md: "Rebase replays each commit onto the new base, so the branch **disappears** from the graph.",
                    mode: "inline"
                )
            )

            DetailView(
                detail: Block.Detail(
                    pros: ["Preserves the branch shape"],
                    cons: ["Adds a merge node", "Noisier graph"],
                    md: "A merge commit records both parents.",
                    mode: "modal"
                )
            )
        }
        .padding()
    }
}
