import CcPresentKit
import IOSKit
import IOSKitUI
import IOSKitWeb
import SwiftUI

/// PRBlockView renders a pr block by hosting the SPA's single-block page, which reads
/// the PR live through the daemon. A load failure, or a preview with no board context,
/// falls back to a native panel naming the PR and its GitHub URL.
struct PRBlockView: View {
    let block: Block.PR
    var context: PackContext?

    @State private var height: CGFloat = PRBlockView.skeletonHeight
    @State private var phase: WebLoadPhase = .loading

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
            .fill(Palette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(Palette.muted))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(Palette.line))
    }

    private var sourcePanel: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("\(block.repo)#\(block.number)")
                .voice(.mono, size: 11)
                .foregroundStyle(Palette.muted)
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.prose, size: 13, weight: .semibold)
                    .foregroundStyle(Palette.ink)
            }
            if let summary = block.summary, !summary.isEmpty {
                MarkdownText(summary, style: .clamped)
            }
            Text("https://github.com/\(block.repo)/pull/\(block.number)")
                .voice(.prose, size: 12)
                .foregroundStyle(Palette.accentInk)
                .textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).stroke(Palette.line, lineWidth: 1))
    }
}
