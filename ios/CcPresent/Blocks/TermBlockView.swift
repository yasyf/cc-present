import CcPresentKit
import SwiftUI

/// TermBlockView renders a term block by hosting the SPA's single-block page, so the
/// ANSI-colored output is pixel-identical to web and follows the system appearance. A
/// skeleton fills a default height while it loads; a load failure — or a preview with no
/// board context — falls back to a native panel: the `$`-prefixed command over the
/// ANSI-stripped output, never blank.
struct TermBlockView: View {
    let block: Block.Term
    var context: PackContext?

    @State private var height: CGFloat = TermBlockView.skeletonHeight
    @State private var phase: WebViewLoadPhase = .loading

    static let skeletonHeight: CGFloat = 160

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
        switch WebBlockPresentation.of(hasContext: context != nil, phase: phase) {
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
            if let command = block.command, !command.isEmpty {
                Text("$ \(command)")
                    .foregroundStyle(BlockPalette.muted)
            }
            Text(Self.stripAnsi(block.output))
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

    private static let ansiEscape = /\u{001B}\[[0-9;?]*[A-Za-z]/

    /// stripAnsi drops the CSI escape sequences (SGR color runs, cursor moves, erases) a
    /// captured stream carries, leaving the plain text a terminal would render.
    static func stripAnsi(_ text: String) -> String {
        text.replacing(ansiEscape, with: "")
    }
}
