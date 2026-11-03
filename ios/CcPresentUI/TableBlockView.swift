import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI

/// TableBlockView renders a columnar block as a SwiftUI Grid. Column headers show
/// as uppercase mono labels, each column honors its `align` (`left` or `right`),
/// and every cell renders its value through inline markdown. The grid scrolls
/// horizontally so a wide table never forces the page to. Filter columns add a
/// text field and value chips, sortable headers sort on a tap, and with a `rowKey`
/// a row opens to its details and actions; an action posts a note naming the row.
/// Mirrors web TableView.
struct TableBlockView: View {
    let block: Block.Table
    var store: BoardStore?

    @Environment(\.isEnabled) private var enabled
    @State private var query = TableQuery()
    @State private var open: String?

    private var filterColumns: [Block.Column] {
        block.columns.filter { $0.filter == true }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if !filterColumns.isEmpty {
                tools
            }
            ScrollView(.horizontal, showsIndicators: false) {
                Grid(alignment: .topLeading, horizontalSpacing: 16, verticalSpacing: 8) {
                    GridRow {
                        ForEach(block.columns, id: \.key) { column in
                            header(column)
                                .gridColumnAlignment(alignment(for: column))
                        }
                    }
                    Divider()
                    let rows = block.visibleRows(query)
                    ForEach(Array(rows.enumerated()), id: \.offset) { index, row in
                        let key = block.rowKey.flatMap { row[$0] }
                        GridRow {
                            ForEach(block.columns, id: \.key) { column in
                                MarkdownText(row[column.key] ?? "")
                                    .font(.subheadline)
                                    .contentShape(Rectangle())
                                    .onTapGesture { toggle(key) }
                            }
                        }
                        if let key, open == key {
                            detail(key)
                        }
                        if index < rows.count - 1 {
                            Divider()
                        }
                    }
                }
                .padding(.vertical, 4)
            }
        }
        .environment(\.isEnabled, true)
    }

    private var tools: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            TextField("Filter", text: $query.text)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .voice(.prose, size: 14)
                .padding(.horizontal, Metrics.space3)
                .padding(.vertical, 6)
                .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: Metrics.space1) {
                    ForEach(filterColumns, id: \.key) { column in
                        ForEach(block.filterChoices(column), id: \.self) { value in
                            let on = query.chips[column.key] == value
                            Button(value) { query = query.toggling(chip: value, for: column.key) }
                                .voice(.prose, size: 13, weight: on ? .semibold : .regular)
                                .foregroundStyle(on ? Palette.accentFg : Palette.ink)
                                .padding(.horizontal, Metrics.space3)
                                .padding(.vertical, 4)
                                .background(on ? Palette.accentInk : Palette.chipBg, in: Capsule())
                                .buttonStyle(.plain)
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private func header(_ column: Block.Column) -> some View {
        let label = Text(column.label.uppercased())
            .font(.system(.caption2, design: .monospaced))
            .fontWeight(.semibold)
            .kerning(0.6)
            .foregroundStyle(.secondary)
        if column.sort == true {
            Button {
                query = query.pressing(column.key)
            } label: {
                HStack(spacing: 2) {
                    label
                    if query.sort?.key == column.key {
                        Image(systemName: query.sort?.descending == true ? "chevron.down" : "chevron.up")
                            .font(.system(size: 9, weight: .bold))
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .buttonStyle(.plain)
        } else {
            label
        }
    }

    private func toggle(_ key: String?) {
        guard let key, opens(key) else { return }
        open = open == key ? nil : key
    }

    private func opens(_ key: String) -> Bool {
        block.details?[key] != nil || !(block.actions ?? []).isEmpty
    }

    private func detail(_ key: String) -> some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if let detail = block.details?[key] {
                MarkdownText(detail)
            }
            if let actions = block.actions, !actions.isEmpty, let store {
                HStack(spacing: Metrics.space2) {
                    ForEach(actions, id: \.id) { action in
                        Button(action.label) {
                            store.feedback(blockId: action.target ?? block.id, text: "\(action.label): \(key)")
                        }
                        .buttonStyle(GhostButtonStyle())
                        .disabled(!enabled || store.isClosed)
                    }
                }
            }
        }
        .padding(Metrics.space2)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
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
