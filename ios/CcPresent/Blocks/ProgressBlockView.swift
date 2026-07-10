import CcPresentKit
import SwiftUI

/// ProgressBlockView renders a progress bar: a label and a `value/max` readout
/// above a fill bar. `state` (`active`, `done`, `error`) tints the fill — accent
/// while active, green when done, red on error — while the fill fraction always
/// tracks value over max.
struct ProgressBlockView: View {
    let block: Block.Progress

    private var state: String {
        block.state ?? "active"
    }

    private var fillColor: Color {
        switch state {
        case "done": .green
        case "error": .red
        default: .accentColor
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(block.label)
                Spacer(minLength: 12)
                Text("\(block.value)/\(block.max)")
                    .monospacedDigit()
            }
            .font(.subheadline)
            .foregroundStyle(.secondary)

            ProgressView(value: Double(block.value), total: Double(max(block.max, 1)))
                .tint(fillColor)
        }
    }
}

#Preview {
    VStack(spacing: 24) {
        ProgressBlockView(
            block: Block.Progress(id: "p1", label: "Indexing files", value: 4, max: 10, state: "active")
        )
        ProgressBlockView(
            block: Block.Progress(id: "p2", label: "Migration complete", value: 8, max: 8, state: "done")
        )
        ProgressBlockView(
            block: Block.Progress(id: "p3", label: "Upload failed", value: 3, max: 10, state: "error")
        )
    }
    .padding()
}
