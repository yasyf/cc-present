import CcPresentKit
import SwiftUI

/// DiagramBlockView renders a diagram block. The Phase 1 stub shows the optional
/// title over the raw mermaid source in a scrollable monospaced panel; the live
/// webview-hosted renderer (the SingleBlockWebView path, pixel-identical to web)
/// lands in Phase 4.
struct DiagramBlockView: View {
    let block: Block.Diagram

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            ScrollView(.horizontal, showsIndicators: true) {
                Text(block.source)
                    .font(.system(size: 13, design: .monospaced))
                    .textSelection(.enabled)
                    .fixedSize(horizontal: true, vertical: false)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 10)
            }
            .background(BlockPalette.monoBg)
            .clipShape(RoundedRectangle(cornerRadius: 4))
            .overlay(
                RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line)
            )
        }
    }
}
