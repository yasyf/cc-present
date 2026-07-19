import CcPresentKit
import SwiftUI

/// ChartBlockView renders a chart block. Phase 0 shows a native fallback listing each
/// series and its values; Phase 1 wires it to the shared single-block webview via
/// WebBlockPresentation, like DiagramBlockView.
struct ChartBlockView: View {
    let block: Block.Chart
    var context: PackContext?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(1)
            }
            ForEach(block.series.indices, id: \.self) { index in
                Text(Self.line(for: block.series[index]))
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

    static func line(for series: Block.Series) -> String {
        let values = series.values.map { String($0) }.joined(separator: ", ")
        return "\(series.label): \(values)"
    }
}
