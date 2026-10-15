import CcPresentKit
import SwiftUI

/// PRBlockView renders a pr block by hosting the SPA's single-block page, which reads
/// the PR live through the daemon. A load failure, or a preview with no board context,
/// falls back to a native panel naming the PR and its GitHub URL.
struct PRBlockView: View {
    let block: Block.PR
    var context: PackContext?

    @State private var height: CGFloat = PRBlockView.skeletonHeight
    @State private var phase: WebViewLoadPhase = .loading

    static let skeletonHeight: CGFloat = 180

    var body: some View {
        switch WebBlockPresentation.of(hasContext: context != nil, phase: phase) {
        case .rawSource:
            sourcePanel
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
        RoundedRectangle(cornerRadius: Metrics.radiusMd)
            .fill(BlockPalette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(BlockPalette.muted))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(BlockPalette.line))
    }

    private var sourcePanel: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("\(block.repo)#\(block.number)")
                .voice(.mono, size: 11)
                .foregroundStyle(BlockPalette.muted)
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.prose, size: 13, weight: .semibold)
                    .foregroundStyle(BlockPalette.ink)
            }
            if let summary = block.summary, !summary.isEmpty {
                MarkdownText(summary, style: .clamped)
            }
            Text("https://github.com/\(block.repo)/pull/\(block.number)")
                .voice(.prose, size: 12)
                .foregroundStyle(BlockPalette.accentInk)
                .textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).stroke(BlockPalette.line, lineWidth: 1))
    }
}
