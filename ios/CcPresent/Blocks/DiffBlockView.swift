import CcPresentKit
import SwiftUI

/// DiffBlockView renders a unified diff as a two-gutter table over CcPresentKit's
/// `parseDiff` output: an old-side and a new-side line-number gutter, a change mark,
/// and the line text, with add/del/context/meta rows tinted. The table scrolls
/// horizontally as one unit, mirroring web/src/components/DiffView.tsx.
struct DiffBlockView: View {
    let block: Block.Diff

    fileprivate static let gutterTextWidth: CGFloat = 40
    fileprivate static let gutterTrailingPad: CGFloat = 6
    fileprivate static let gutterWidth = gutterTextWidth + gutterTrailingPad
    fileprivate static let markWidth: CGFloat = 18

    private var hunks: [DiffHunk] {
        parseDiff(block.diff)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 10, weight: .semibold, design: .monospaced))
                    .tracking(1)
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            table
        }
    }

    private var table: some View {
        ScrollView(.horizontal, showsIndicators: true) {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Array(hunks.enumerated()), id: \.offset) { _, hunk in
                    hunkHeadRow(hunk.heading)
                    ForEach(Array(hunk.rows.enumerated()), id: \.offset) { _, row in
                        DiffRowView(row: row)
                    }
                }
            }
            .fixedSize(horizontal: true, vertical: false)
        }
        .clipShape(RoundedRectangle(cornerRadius: 4))
        .overlay(
            RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line)
        )
    }

    private func hunkHeadRow(_ heading: String) -> some View {
        HStack(spacing: 0) {
            Color.clear.frame(width: Self.gutterWidth)
            Color.clear.frame(width: Self.gutterWidth)
            Color.clear.frame(width: Self.markWidth)
            Text(heading.isEmpty ? "@@" : "@@ \(heading)")
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(BlockPalette.muted)
                .fixedSize(horizontal: true, vertical: false)
                .padding(.trailing, 8)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.vertical, 1)
        .background(BlockPalette.monoBg)
    }
}

private struct DiffRowView: View {
    let row: DiffRow

    var body: some View {
        HStack(spacing: 0) {
            gutter(row.oldNo)
            gutter(row.newNo)
            Text(mark)
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(markColor)
                .frame(width: DiffBlockView.markWidth)
            Text(row.text)
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(BlockPalette.ink)
                .fixedSize(horizontal: true, vertical: false)
                .padding(.trailing, 8)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(tint)
    }

    private func gutter(_ number: Int?) -> some View {
        Text(number.map(String.init) ?? "")
            .font(.system(size: 10, design: .monospaced))
            .foregroundStyle(BlockPalette.was)
            .frame(width: DiffBlockView.gutterTextWidth, alignment: .trailing)
            .padding(.trailing, DiffBlockView.gutterTrailingPad)
    }

    private var mark: String {
        switch row.kind {
        case .add: "+"
        case .del: "-"
        case .context: " "
        case .meta: ""
        }
    }

    private var markColor: Color {
        switch row.kind {
        case .add: BlockPalette.approve
        case .del: BlockPalette.reject
        case .context, .meta: BlockPalette.muted
        }
    }

    private var tint: Color {
        switch row.kind {
        case .add: BlockPalette.approve.opacity(0.14)
        case .del: BlockPalette.reject.opacity(0.14)
        case .context, .meta: .clear
        }
    }
}

#Preview("Diff block") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            DiffBlockView(
                block: Block.Diff(
                    id: "diff-sample",
                    diff: """
                    diff --git a/internal/state/reduce.go b/internal/state/reduce.go
                    index 1a2b3c4..5d6e7f8 100644
                    --- a/internal/state/reduce.go
                    +++ b/internal/state/reduce.go
                    @@ -12,7 +12,8 @@ func Reduce(events []Event) BoardState {
                     	state := BoardState{}
                     	for _, ev := range events {
                    -		state.apply(ev)
                    +		// apply each event in arrival order
                    +		state.applyOrdered(ev)
                     	}
                     	return state
                     }
                    """,
                    title: "reduce.go"
                )
            )
        }
        .padding()
    }
}
