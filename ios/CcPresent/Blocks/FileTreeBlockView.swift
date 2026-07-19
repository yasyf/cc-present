import CcPresentKit
import SwiftUI

/// FileTreeBlockView renders a filetree block. Phase 0 shows a native fallback with one
/// monospaced line per entry, badge-prefixed (`+`/`~`/`−`); Phase 1 wires it to the
/// shared single-block webview via WebBlockPresentation, like DiagramBlockView.
struct FileTreeBlockView: View {
    let block: Block.FileTree
    var context: PackContext?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            VStack(alignment: .leading, spacing: 2) {
                ForEach(block.entries.indices, id: \.self) { index in
                    let entry = block.entries[index]
                    Text("\(Self.badgeGlyph(entry.badge)) \(entry.path)")
                        .font(.system(size: 13, design: .monospaced))
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

    static func badgeGlyph(_ badge: String?) -> String {
        switch badge {
        case "added": "+"
        case "modified": "~"
        case "removed": "\u{2212}"
        default: " "
        }
    }
}
