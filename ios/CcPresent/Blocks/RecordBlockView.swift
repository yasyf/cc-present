import CcPresentKit
import SwiftUI

/// RecordBlockView renders a record block. Phase 0 shows a native fallback — title, a
/// bracketed chip row, monospaced `label: value` fact lines, and `label — url` links;
/// Phase 1 wires it to the shared single-block webview via WebBlockPresentation.
struct RecordBlockView: View {
    let block: Block.Record
    var context: PackContext?

    var body: some View {
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
