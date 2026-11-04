import CcPresentKit
import IOSKitUI
import SwiftUI

/// FileTreeBlockView renders a filetree block natively as an indented tree: each
/// implicit directory is a disclosure row, open by default, and each file row shows
/// its name, an optional change badge, and an optional dim note; a removed file's name
/// is struck through. Mirrors web FileTreeView.
struct FileTreeBlockView: View {
    let block: Block.FileTree

    @State private var collapsed: Set<String> = []

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.mono, size: 11)
                    .foregroundStyle(Palette.muted)
                    .lineLimit(1)
            }
            VStack(alignment: .leading, spacing: 2) {
                ForEach(Self.visibleRows(Self.rows(from: block.entries), collapsed: collapsed)) { row in
                    rowView(row)
                        .padding(.leading, CGFloat(row.depth) * Metrics.space4)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 14)
            .padding(.vertical, 10)
            .background(Palette.monoBg)
            .clipShape(RoundedRectangle(cornerRadius: Metrics.radiusMd))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(Palette.line))
        }
    }

    @ViewBuilder
    private func rowView(_ row: FileTreeRow) -> some View {
        if row.isDirectory {
            Button {
                if collapsed.contains(row.path) {
                    collapsed.remove(row.path)
                } else {
                    collapsed.insert(row.path)
                }
            } label: {
                HStack(spacing: Metrics.space1) {
                    Image(systemName: collapsed.contains(row.path) ? "chevron.right" : "chevron.down")
                        .font(.system(size: 10, weight: .semibold))
                        .frame(width: 12)
                    Text(row.name + "/")
                }
                .voice(.mono, size: 13)
                .foregroundStyle(Palette.muted)
                .frame(minHeight: 24)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .environment(\.isEnabled, true)
        } else {
            HStack(alignment: .firstTextBaseline, spacing: Metrics.space2) {
                Text(row.name)
                    .strikethrough(row.badge == "removed")
                    .foregroundStyle(row.badge == "removed" ? Palette.was : Palette.ink)
                    .textSelection(.enabled)
                if let badge = row.badge {
                    Text(badge)
                        .voice(.stamp, size: 10, weight: .semibold)
                        .foregroundStyle(Self.badgeTone(badge))
                        .padding(.horizontal, 5)
                        .padding(.vertical, 1)
                        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusSm).strokeBorder(Self.badgeTone(badge).opacity(0.5)))
                }
                if let note = row.note {
                    Text(note)
                        .foregroundStyle(Palette.muted)
                        .lineLimit(2)
                }
            }
            .voice(.mono, size: 13)
            .padding(.leading, 16)
            .frame(minHeight: 22)
        }
    }

    static func badgeTone(_ badge: String) -> Color {
        switch badge {
        case "added": Palette.approve
        case "modified": Palette.warn
        case "removed": Palette.reject
        default: Palette.muted
        }
    }
}

extension FileTreeBlockView {
    /// FileTreeRow is one flattened line of the tree: a directory or a file leaf, its
    /// path, its depth for indentation, and (for files) a change badge and note.
    struct FileTreeRow: Identifiable, Equatable {
        let id: Int
        let depth: Int
        let path: String
        let name: String
        let isDirectory: Bool
        let badge: String?
        let note: String?
    }

    /// visibleRows drops every row beneath a collapsed directory.
    static func visibleRows(_ rows: [FileTreeRow], collapsed: Set<String>) -> [FileTreeRow] {
        var visible: [FileTreeRow] = []
        var hiddenBelow: Int?
        for row in rows {
            if let depth = hiddenBelow {
                if row.depth > depth {
                    continue
                }
                hiddenBelow = nil
            }
            visible.append(row)
            if row.isDirectory, collapsed.contains(row.path) {
                hiddenBelow = row.depth
            }
        }
        return visible
    }

    /// rows folds path entries into a depth-first flattened tree — directories before
    /// files, lexicographic within each group — mirroring the web `buildTree` order.
    static func rows(from entries: [Block.TreeEntry]) -> [FileTreeRow] {
        var rows: [FileTreeRow] = []
        var index = 0
        func walk(_ nodes: [FileTreeNode], depth: Int, prefix: String) {
            for node in nodes {
                let path = prefix.isEmpty ? node.name : "\(prefix)/\(node.name)"
                rows.append(FileTreeRow(
                    id: index,
                    depth: depth,
                    path: path,
                    name: node.name,
                    isDirectory: node.entry == nil,
                    badge: node.entry?.badge,
                    note: node.entry?.note
                ))
                index += 1
                walk(node.children, depth: depth + 1, prefix: path)
            }
        }
        walk(FileTreeNode.forest(from: entries), depth: 0, prefix: "")
        return rows
    }
}

private final class FileTreeNode {
    let name: String
    var entry: Block.TreeEntry?
    var children: [FileTreeNode] = []

    init(name: String, entry: Block.TreeEntry? = nil) {
        self.name = name
        self.entry = entry
    }

    static func forest(from entries: [Block.TreeEntry]) -> [FileTreeNode] {
        let root = FileTreeNode(name: "")
        for entry in entries {
            let segments = entry.path.split(separator: "/").map(String.init)
            var cursor = root
            for (depth, name) in segments.enumerated() {
                if depth == segments.count - 1 {
                    cursor.children.append(FileTreeNode(name: name, entry: entry))
                    break
                }
                if let dir = cursor.children.first(where: { $0.name == name && $0.entry == nil }) {
                    cursor = dir
                } else {
                    let dir = FileTreeNode(name: name)
                    cursor.children.append(dir)
                    cursor = dir
                }
            }
        }
        root.sortRecursively()
        return root.children
    }

    private func sortRecursively() {
        children.sort { lhs, rhs in
            let lhsDir = lhs.entry == nil
            let rhsDir = rhs.entry == nil
            if lhsDir != rhsDir {
                return lhsDir
            }
            return lhs.name < rhs.name
        }
        for child in children {
            child.sortRecursively()
        }
    }
}
