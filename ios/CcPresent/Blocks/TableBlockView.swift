import CcPresentKit
import SwiftUI

/// TableBlockView renders a columnar block as a SwiftUI Grid. Column headers show
/// as uppercase mono labels, each column honors its `align` (`left` or `right`),
/// and every cell renders its value through inline markdown. The grid scrolls
/// horizontally so a wide table never forces the page to.
struct TableBlockView: View {
    let block: Block.Table

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Grid(alignment: .topLeading, horizontalSpacing: 16, verticalSpacing: 8) {
                GridRow {
                    ForEach(block.columns, id: \.key) { column in
                        Text(column.label.uppercased())
                            .font(.system(.caption2, design: .monospaced))
                            .fontWeight(.semibold)
                            .kerning(0.6)
                            .foregroundStyle(.secondary)
                            .gridColumnAlignment(alignment(for: column))
                    }
                }
                Divider()
                ForEach(Array(block.rows.enumerated()), id: \.offset) { index, row in
                    GridRow {
                        ForEach(block.columns, id: \.key) { column in
                            MarkdownText(row[column.key] ?? "")
                                .font(.subheadline)
                        }
                    }
                    if index < block.rows.count - 1 {
                        Divider()
                    }
                }
            }
            .padding(.vertical, 4)
        }
    }

    private func alignment(for column: Block.Column) -> HorizontalAlignment {
        column.align == "right" ? .trailing : .leading
    }
}

#Preview {
    TableBlockView(
        block: Block.Table(
            id: "tbl1",
            columns: [
                Block.Column(key: "name", label: "Name", align: "left"),
                Block.Column(key: "status", label: "Status"),
                Block.Column(key: "count", label: "Count", align: "right"),
            ],
            rows: [
                ["name": "**Renderer**", "status": "`shipping`", "count": "12"],
                ["name": "Diff parser", "status": "_in review_", "count": "3"],
                ["name": "Pairing", "status": "blocked", "count": "128"],
            ]
        )
    )
    .padding()
}
