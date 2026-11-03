import CcPresentKit
import IOSKit
import IOSKitUI
import IOSKitWeb
import SwiftUI

/// PRGroupsBlockView renders a pr-groups block by hosting the SPA's single-block page,
/// which draws the groups, reads each PR live through the daemon, and posts verdicts.
/// A load failure, or a preview with no board context, falls back to a native panel
/// naming the block.
struct PRGroupsBlockView: View {
    let block: Block.PRGroups
    var context: PackContext?

    @State private var height: CGFloat = PRGroupsBlockView.skeletonHeight
    @State private var phase: WebLoadPhase = .loading

    static let skeletonHeight: CGFloat = 320

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
        VStack(alignment: .leading, spacing: 4) {
            Text("pull request groups")
                .voice(.mono, size: 11)
                .foregroundStyle(Palette.muted)
            Text(block.id)
                .voice(.prose, size: 13, weight: .semibold)
                .foregroundStyle(Palette.ink)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).stroke(Palette.line, lineWidth: 1))
    }
}
