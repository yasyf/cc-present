import CcPresentKit
import SwiftUI

/// ChoiceBlockView renders a single- or multi-select option list. Each option
/// shows its label, an optional inline-markdown hint, and an optional clamped
/// markdown body. Selection is the last-write-wins set from BoardState; a tap
/// toggles one option and posts the FULL next `optionIds` array (single-select
/// replaces or clears, multi adds or removes), matching choice.selected semantics.
/// A closed board is read-only. Mirrors web/src/components/Choice.tsx.
struct ChoiceBlockView: View {
    let block: Block.Choice
    let store: BoardStore

    private var multi: Bool {
        block.multi ?? false
    }

    private var locked: Bool {
        store.isClosed
    }

    private var selected: [String] {
        store.state.interactions.choices[block.id]?.optionIds ?? []
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            if let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .fontWeight(.medium)
                    .foregroundStyle(BlockPalette.ink)
            }

            VStack(alignment: .leading, spacing: 8) {
                ForEach(block.options, id: \.id) { option in
                    optionRow(option, isOn: selected.contains(option.id))
                }
            }
        }
    }

    private func optionRow(_ option: Block.Option, isOn: Bool) -> some View {
        HStack(alignment: .top, spacing: 12) {
            OptionIndicator(multi: multi, isOn: isOn)
                .padding(.top, 2)
            VStack(alignment: .leading, spacing: 4) {
                Text(option.label)
                    .font(.body)
                    .fontWeight(.semibold)
                    .foregroundStyle(BlockPalette.ink)
                if let hint = option.hint, !hint.isEmpty {
                    MarkdownText(hint)
                        .font(.caption2)
                        .foregroundStyle(BlockPalette.muted)
                }
                if let body = option.md, !body.isEmpty {
                    MarkdownText(body, style: .clamped)
                        .font(.subheadline)
                        .foregroundStyle(BlockPalette.muted)
                }
            }
            Spacer(minLength: 0)
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            isOn ? BlockPalette.accentInk.opacity(0.08) : Color.clear,
            in: RoundedRectangle(cornerRadius: 8)
        )
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .strokeBorder(isOn ? BlockPalette.accentInk : BlockPalette.line)
        )
        .opacity(locked ? 0.55 : 1)
        .contentShape(RoundedRectangle(cornerRadius: 8))
        .onTapGesture { toggle(option.id) }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(traits(isOn: isOn))
        .accessibilityAction { toggle(option.id) }
    }

    private func traits(isOn: Bool) -> AccessibilityTraits {
        var traits: AccessibilityTraits = multi ? .isToggle : .isButton
        if isOn {
            traits.insert(.isSelected)
        }
        return traits
    }

    private func toggle(_ optionId: String) {
        guard !locked else { return }
        let next: [String] = if multi {
            selected.contains(optionId) ? selected.filter { $0 != optionId } : selected + [optionId]
        } else {
            selected.contains(optionId) ? [] : [optionId]
        }
        store.choose(blockId: block.id, optionIds: next)
    }
}

/// OptionIndicator draws the leading selection marker: a radio dot for single
/// select, a checkbox for multi, filled with the accent tint when selected.
private struct OptionIndicator: View {
    let multi: Bool
    let isOn: Bool

    private let size: CGFloat = 18

    var body: some View {
        marker
            .frame(width: size, height: size)
    }

    @ViewBuilder
    private var marker: some View {
        if multi {
            RoundedRectangle(cornerRadius: 4, style: .continuous)
                .fill(isOn ? BlockPalette.accentInk : Color.clear)
                .overlay(
                    RoundedRectangle(cornerRadius: 4, style: .continuous)
                        .strokeBorder(borderColor, lineWidth: 1.5)
                )
                .overlay {
                    if isOn {
                        Image(systemName: "checkmark")
                            .font(.system(size: 10, weight: .bold))
                            .foregroundStyle(BlockPalette.monoBg)
                    }
                }
        } else {
            Circle()
                .strokeBorder(borderColor, lineWidth: 1.5)
                .overlay {
                    if isOn {
                        Circle()
                            .fill(BlockPalette.accentInk)
                            .frame(width: size * 0.4, height: size * 0.4)
                    }
                }
        }
    }

    private var borderColor: Color {
        isOn ? BlockPalette.accentInk : BlockPalette.borderStrong
    }
}

@MainActor
private func previewStore(blockId: String, selected: [String]) -> BoardStore {
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    if !selected.isEmpty {
        store.choose(blockId: blockId, optionIds: selected)
    }
    return store
}

private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

#Preview("Choice block") {
    let options = [
        Block.Option(id: "opt-a", label: "Rebase onto main", hint: "keeps history linear"),
        Block.Option(
            id: "opt-b",
            label: "Merge commit",
            hint: "preserves the branch shape",
            md: "Adds a merge node so the two lines of work stay **visible** in the graph."
        ),
        Block.Option(id: "opt-c", label: "Squash and merge"),
    ]

    return ScrollView {
        VStack(alignment: .leading, spacing: 32) {
            ChoiceBlockView(
                block: Block.Choice(id: "c-single", prompt: "How should we land this?", options: options),
                store: previewStore(blockId: "c-single", selected: ["opt-b"])
            )

            ChoiceBlockView(
                block: Block.Choice(
                    id: "c-multi",
                    prompt: "Which checks must pass?",
                    multi: true,
                    options: [
                        Block.Option(id: "lint", label: "Lint"),
                        Block.Option(id: "test", label: "Tests"),
                        Block.Option(id: "build", label: "Build"),
                    ]
                ),
                store: previewStore(blockId: "c-multi", selected: ["lint", "build"])
            )
        }
        .padding()
    }
}
