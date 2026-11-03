import CcPresentKit
import IOSKit
import IOSKitUI
import IOSKitWeb
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
    @State private var phase: WebLoadPhase = .loading

    static let skeletonHeight: CGFloat = 160

    private var presentation: WebBlockPresentation {
        WebBlockPresentation.of(hasContext: context != nil, phase: phase)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if presentation.showsNativeTitle, let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.mono, size: 11)
                    .foregroundStyle(Palette.muted)
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
        RoundedRectangle(cornerRadius: Metrics.radiusMd)
            .fill(Palette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(Palette.muted))
            .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(Palette.line))
    }

    private var fallbackPanel: some View {
        VStack(alignment: .leading, spacing: 4) {
            if let command = block.command, !command.isEmpty {
                Text("$ \(command)")
                    .foregroundStyle(Palette.muted)
            }
            Text(Self.stripAnsi(block.output))
                .foregroundStyle(Palette.ink)
        }
        .voice(.mono, size: 13)
        .textSelection(.enabled)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .background(Palette.monoBg)
        .clipShape(RoundedRectangle(cornerRadius: Metrics.radiusMd))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(Palette.line))
    }

    private static let ansiEscape =
        /\u{001B}\][^\u{0007}\u{001B}]*(?:\u{0007}|\u{001B}\\)|\u{001B}\[[\u{0030}-\u{003F}]*[\u{0020}-\u{002F}]*[\u{0040}-\u{007E}]|\u{001B}[\u{0020}-\u{002F}]*[\u{0030}-\u{007E}]/

    /// stripAnsi drops every ANSI escape a captured stream carries — SGR color runs
    /// (semicolon or colon form), cursor moves, erases, and OSC strings (an OSC-8
    /// hyperlink's visible text is kept) — leaving the plain text a terminal renders.
    static func stripAnsi(_ text: String) -> String {
        text.replacing(ansiEscape, with: "")
    }
}
