import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI

/// SectionView renders a section block: an h2-style title over optional markdown
/// prose, mirroring web/src/components/Section.tsx.
public struct SectionView: View {
    let block: Block.Section

    public init(block: Block.Section) {
        self.block = block
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(block.title)
                .font(.title2)
                .fontWeight(.semibold)
                .foregroundStyle(Palette.ink)

            if let md = block.md, !md.isEmpty {
                MarkdownText(md)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

#Preview("Section") {
    ScrollView {
        VStack(alignment: .leading, spacing: 32) {
            SectionView(
                block: Block.Section(
                    id: "sec-intro",
                    title: "Proposed changes",
                    md: """
                    Three files change to land the **native block renderer**. Review each card, then
                    approve or leave feedback.

                    - Verdicts are last-write-wins.
                    - Feedback is append-only.
                    """
                )
            )

            SectionView(block: Block.Section(id: "sec-bare", title: "No prose section"))
        }
        .padding()
    }
}
