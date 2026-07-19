import CcPresentKit
import SwiftUI

/// RecordBlockView renders a record block by hosting the SPA's single-block page. A
/// skeleton fills a default height while it loads; a load failure — or a preview with
/// no board context — falls back to the native record panel, never blank.
struct RecordBlockView: View {
    let block: Block.Record
    var context: PackContext?

    @State private var height: CGFloat = RecordBlockView.skeletonHeight
    @State private var phase: WebViewLoadPhase = .loading

    static let skeletonHeight: CGFloat = 220

    var body: some View {
        content
    }

    @ViewBuilder
    private var content: some View {
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
        RoundedRectangle(cornerRadius: 4)
            .fill(BlockPalette.monoBg)
            .frame(height: Self.skeletonHeight)
            .frame(maxWidth: .infinity)
            .overlay(ProgressView().tint(BlockPalette.muted))
            .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line))
    }

    private var sourcePanel: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(BlockPalette.ink)
            }
            if let chips = block.chips, !chips.isEmpty {
                Text(chips.map { "[\($0.label)]" }.joined(separator: " "))
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(BlockPalette.muted)
            }
            ForEach(block.facts.indices, id: \.self) { index in
                let fact = block.facts[index]
                Text("\(fact.label ?? ""): \(fact.value)")
                    .font(.system(size: 13, design: .monospaced))
                    .textSelection(.enabled)
            }
            if let links = block.links, !links.isEmpty {
                ForEach(links.indices, id: \.self) { index in
                    let link = links[index]
                    Text("\(link.label) — \(link.url)")
                        .font(.system(size: 12))
                        .foregroundStyle(BlockPalette.accentInk)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(12)
        .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(BlockPalette.line, lineWidth: 1))
    }
}
