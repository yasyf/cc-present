import CcPresentKit
import SwiftUI

/// DiagramBlockView renders a diagram block by hosting the SPA's single-block page, so
/// the mermaid output is pixel-identical to web and follows the system appearance. A
/// skeleton fills a default height while it loads; a load failure — or a preview with
/// no board context — falls back to the raw source in a monospaced panel, never blank.
struct DiagramBlockView: View {
    let block: Block.Diagram
    var context: PackContext?

    @State private var height: CGFloat = DiagramBlockView.skeletonHeight
    @State private var phase: WebViewLoadPhase = .loading

    static let skeletonHeight: CGFloat = 220

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
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
        switch Self.presentation(hasContext: context != nil, phase: phase) {
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
        RoundedRectangle(cornerRadius: 4)
            .fill(BlockPalette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(BlockPalette.muted))
            .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
    }

    private var sourcePanel: some View {
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
        .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
    }

    /// DiagramPresentation is the content a diagram shows for a given context
    /// availability and load phase — the pure mapping the fallback tests pin.
    enum DiagramPresentation: Equatable {
        case webView(showingSkeleton: Bool)
        case rawSource
    }

    static func presentation(hasContext: Bool, phase: WebViewLoadPhase) -> DiagramPresentation {
        guard hasContext else { return .rawSource }
        switch phase {
        case .loading: return .webView(showingSkeleton: true)
        case .loaded: return .webView(showingSkeleton: false)
        case .failed: return .rawSource
        }
    }
}
