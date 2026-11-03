import CcPresentKit
import IOSKitUI
import SwiftUI

/// RecordBlockView renders a record block natively: one entity's title, its tone
/// chips, its labelled facts with each fact's tone, and its links. Mirrors web
/// RecordView.
struct RecordBlockView: View {
    let block: Block.Record

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.prose, size: 15, weight: .semibold)
                    .foregroundStyle(Palette.ink)
            }
            if let chips = block.chips, !chips.isEmpty {
                FlowChips(chips: chips)
            }
            VStack(alignment: .leading, spacing: Metrics.space1) {
                ForEach(Array(block.facts.enumerated()), id: \.offset) { _, fact in
                    HStack(alignment: .firstTextBaseline, spacing: Metrics.space3) {
                        if let label = fact.label {
                            Text(label)
                                .voice(.stamp, size: 11, weight: .semibold)
                                .foregroundStyle(Palette.muted)
                                .frame(width: 110, alignment: .leading)
                        }
                        Text(fact.value)
                            .voice(.prose, size: 14)
                            .foregroundStyle(Palette.factTone(fact.tone))
                            .textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
            if let links = block.links, !links.isEmpty {
                HStack(spacing: Metrics.space3) {
                    ForEach(Array(links.enumerated()), id: \.offset) { _, link in
                        if let url = URL(string: link.url) {
                            Link(link.label, destination: url)
                                .voice(.prose, size: 14, weight: .semibold)
                                .foregroundStyle(Palette.accentInk)
                        }
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(Metrics.space3)
        .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).stroke(Palette.line, lineWidth: 1))
        .receiptContent()
    }
}

private struct FlowChips: View {
    let chips: [Block.Chip]

    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: Metrics.space1) {
                ForEach(Array(chips.enumerated()), id: \.offset) { _, chip in
                    ChipView(chip: chip)
                }
            }
        }
        .scrollIndicators(.hidden)
    }
}
