import CcPresentKit
import SwiftUI

/// FileTreeBlockView renders a filetree block by hosting the SPA's single-block page, so
/// the collapsible tree is pixel-identical to web and follows the system appearance. A
/// skeleton fills a default height while it loads; a load failure — or a preview with no
/// board context — falls back to an indented monospaced listing with `+`/`~`/`−` badge
/// markers, never blank.
struct FileTreeBlockView: View {
    let block: Block.FileTree
    var context: PackContext?

    @State private var height: CGFloat = FileTreeBlockView.skeletonHeight
    @State private var phase: WebViewLoadPhase = .loading

    static let skeletonHeight: CGFloat = 220

    private var presentation: WebBlockPresentation {
        WebBlockPresentation.of(hasContext: context != nil, phase: phase)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if presentation.showsNativeTitle, let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            content
        }
    }

    @ViewBuilder
    private var content: some View {
        switch presentation {
        case .rawSource:
            fallbackPanel
        case let .webView(showingSkeleton):
            ZStack {
                if let context {
                    SingleBlockWebView(
                        url: context.singleBlockURL(blockId: block.id),
                        height: $height,
                        phase: $phase
                    )
                    .frame(height: height)
                    .frame(maxWidth: .infinity)
                }
                if showingSkeleton {
                    skeleton
                }
            }
        }
    }

    private var skeleton: some View {
        RoundedRectangle(cornerRadius: 4)
            .fill(BlockPalette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(BlockPalette.muted))
            .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
    }

    private var fallbackPanel: some View {
        VStack(alignment: .leading, spacing: 2) {
            ForEach(Self.rows(from: block.entries)) { row in
                Text(Self.line(for: row))
                    .font(.system(size: 13, design: .monospaced))
                    .foregroundStyle(row.isDirectory ? BlockPalette.muted : BlockPalette.ink)
                    .textSelection(.enabled)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .background(BlockPalette.monoBg)
        .clipShape(RoundedRectangle(cornerRadius: 4))
        .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
    }
}

extension FileTreeBlockView {
    /// FileTreeRow is one flattened line of the native fallback tree: a directory or a
    /// file leaf, its depth for indentation, and (for files) a change badge and note.
    struct FileTreeRow: Identifiable, Equatable {
        let id: Int
        let depth: Int
        let name: String
        let isDirectory: Bool
        let badge: String?
        let note: String?
    }

    /// badgeGlyph is the one-character change marker prefixed to a file line.
    static func badgeGlyph(_ badge: String?) -> String {
        switch badge {
        case "added": "+"
        case "modified": "~"
        case "removed": "\u{2212}"
        default: " "
        }
    }

    /// line renders one flattened row with a two-character badge gutter so sibling
    /// names align: a directory as `<indent>  name/`, a file as `<indent><glyph> name`
    /// with an optional trailing `# note`.
    static func line(for row: FileTreeRow) -> String {
        let indent = String(repeating: "  ", count: row.depth)
        if row.isDirectory {
            return "\(indent)  \(row.name)/"
        }
        let note = row.note.map { "  # \($0)" } ?? ""
        return "\(indent)\(badgeGlyph(row.badge)) \(row.name)\(note)"
    }

    /// rows folds path entries into a depth-first flattened tree — directories before
    /// files, lexicographic within each group — mirroring the web `buildTree` order.
    static func rows(from entries: [Block.TreeEntry]) -> [FileTreeRow] {
        var rows: [FileTreeRow] = []
        var index = 0
        func walk(_ nodes: [FileTreeNode], depth: Int) {
            for node in nodes {
                rows.append(FileTreeRow(
                    id: index,
                    depth: depth,
                    name: node.name,
                    isDirectory: node.entry == nil,
                    badge: node.entry?.badge,
                    note: node.entry?.note
                ))
                index += 1
                walk(node.children, depth: depth + 1)
            }
        }
        walk(FileTreeNode.forest(from: entries), depth: 0)
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
            for (i, name) in segments.enumerated() {
                if i == segments.count - 1 {
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
        children.sort { a, b in
            let aDir = a.entry == nil
            let bDir = b.entry == nil
            if aDir != bDir { return aDir }
            return a.name < b.name
        }
        for child in children { child.sortRecursively() }
    }
}
