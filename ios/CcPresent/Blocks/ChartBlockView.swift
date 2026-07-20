import CcPresentKit
import SwiftUI

/// ChartBlockView renders a chart block by hosting the SPA's single-block page, so the
/// themed SVG is pixel-identical to web and follows the system appearance. A skeleton
/// fills a default height while it loads; a load failure — or a preview with no board
/// context — falls back to a native panel listing each series and its values, never blank.
struct ChartBlockView: View {
    let block: Block.Chart
    var context: PackContext?

    @State private var height: CGFloat = ChartBlockView.skeletonHeight
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
        VStack(alignment: .leading, spacing: 4) {
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
