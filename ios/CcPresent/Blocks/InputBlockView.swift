import CcPresentKit
import SwiftUI

/// InputBlockView renders a free-text input block. The committed value is the
/// source of truth, drawn from BoardState's last-write-wins inputs map: in a live
/// round it seeds the field only while the entry belongs to the current round, and
/// clears once the round advances past it (a "last round: …" hint then carries the
/// prior text forward); a closed board always shows its frozen text. Editing is
/// uncontrolled and commits only on blur — or on Return for a single-line field —
/// and only when the draft differs from the committed value, posting
/// `input.submitted` through the store. Mirrors web/src/components/Input.tsx.
struct InputBlockView: View {
    let block: Block.Input
    let store: BoardStore

    private var value: InputValue? {
        store.state.interactions.inputs[block.id]
    }

    private var locked: Bool {
        store.isClosed
    }

    private var committed: String {
        guard let value else { return "" }
        return locked || value.round == store.state.rounds.current ? value.text : ""
    }

    private var lastRound: String? {
        guard !locked, let value, value.round < store.state.rounds.current else { return nil }
        return value.text
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(block.label)
                .font(.subheadline)
                .foregroundStyle(BlockPalette.muted)

            if let lastRound {
                Text("last round: \(lastRound)")
                    .font(.caption)
                    .italic()
                    .foregroundStyle(BlockPalette.was)
            }

            UncontrolledField(
                committed: committed,
                placeholder: block.placeholder ?? "",
                multiline: block.multiline ?? false,
                disabled: locked
            ) { text in
                store.submitInput(blockId: block.id, text: text)
            }
            .id(committed)
        }
    }
}

/// UncontrolledField is a text field whose visible draft is edited freely and
/// committed only on blur (or Return, single-line). It seeds its draft from
/// `committed` at construction — the parent re-mounts it (via `.id(committed)`)
/// whenever the committed value changes, re-seeding the draft, the SwiftUI analogue
/// of the web field's `key={committed}` uncontrolled remount. A local mirror of the
/// committed value makes `commit` idempotent, so a Return that both submits and
/// blurs posts only once.
private struct UncontrolledField: View {
    let placeholder: String
    let multiline: Bool
    let disabled: Bool
    let onCommit: (String) -> Void

    @State private var draft: String
    @State private var committed: String
    @FocusState private var focused: Bool

    init(
        committed: String,
        placeholder: String,
        multiline: Bool,
        disabled: Bool,
        onCommit: @escaping (String) -> Void
    ) {
        self.placeholder = placeholder
        self.multiline = multiline
        self.disabled = disabled
        self.onCommit = onCommit
        _draft = State(initialValue: committed)
        _committed = State(initialValue: committed)
    }

    var body: some View {
        field
            .font(.body)
            .foregroundStyle(BlockPalette.ink)
            .focused($focused)
            .disabled(disabled)
            .padding(10)
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .strokeBorder(focused ? BlockPalette.accentInk : BlockPalette.line)
            )
            .onChange(of: focused) { _, isFocused in
                if !isFocused {
                    commit()
                }
            }
    }

    @ViewBuilder
    private var field: some View {
        if multiline {
            TextField(placeholder, text: $draft, axis: .vertical)
                .lineLimit(3...)
        } else {
            TextField(placeholder, text: $draft)
                .onSubmit(commit)
        }
    }

    private func commit() {
        guard draft != committed else { return }
        committed = draft
        onCommit(draft)
    }
}

@MainActor
private func previewStore(seedInput: (blockId: String, text: String)? = nil) -> BoardStore {
    let store = BoardStore(subject: "preview", transport: PreviewPoster())
    if let seedInput {
        store.submitInput(blockId: seedInput.blockId, text: seedInput.text)
    }
    return store
}

private struct PreviewPoster: InteractionPoster {
    func postInteraction(subject _: String, interaction _: Interaction) async throws -> Int64 {
        1
    }
}

#Preview("Input block") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            InputBlockView(
                block: Block.Input(
                    id: "in-empty",
                    label: "What should we name the release?",
                    placeholder: "e.g. Aurora"
                ),
                store: previewStore()
            )

            InputBlockView(
                block: Block.Input(
                    id: "in-filled",
                    label: "Release notes",
                    placeholder: "Summarize the changes",
                    multiline: true
                ),
                store: previewStore(seedInput: (blockId: "in-filled", text: "Ships the native block renderer."))
            )
        }
        .padding()
    }
}
