import CcPresentKit
import SwiftUI

/// TermBlockView renders a term block. Phase 0 shows a native fallback with a
/// `$`-prefixed command line above the captured output; Phase 1 wires it to the
/// shared single-block webview (and strips ANSI) via WebBlockPresentation.
struct TermBlockView: View {
    let block: Block.Term
    var context: PackContext?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            VStack(alignment: .leading, spacing: 4) {
                if let command = block.command, !command.isEmpty {
                    Text("$ \(command)")
                        .foregroundStyle(BlockPalette.muted)
                }
                Text(block.output)
                    .foregroundStyle(BlockPalette.ink)
            }
            .font(.system(size: 13, design: .monospaced))
            .textSelection(.enabled)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 14)
            .padding(.vertical, 10)
            .background(BlockPalette.monoBg)
            .clipShape(RoundedRectangle(cornerRadius: 4))
            .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
        }
    }
}
