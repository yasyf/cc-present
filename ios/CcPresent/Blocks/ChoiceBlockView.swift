import CcPresentKit
import SwiftUI

/// ChoiceBlockView renders a single- or multi-select option list plus two chrome escape
/// hatches — an "Other" write-in row and an "Add note" affordance. Selection is
/// last-write-wins; the write-in posts `other` (single-select replaces the picks, multi
/// coexists) and a note is append-only feedback that never decides. Mirrors web Choice.tsx.
struct ChoiceBlockView: View {
    let block: Block.Choice
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.focusHeadlineId) private var focusHeadlineId
    @Environment(\.focusComposer) private var focusComposer
    @Environment(\.blockReplies) private var blockReplies

    @State private var otherDraft = ""
    @State private var noteDraft = ""
    @State private var noteComposing = false
    @FocusState private var otherFocused: Bool
    @FocusState private var noteFocused: Bool

    private var multi: Bool {
        block.multi ?? false
    }

    private var locked: Bool {
        store.isClosed
    }

    private var selection: Selection? {
        store.state.interactions.choices[block.id]
    }

    private var selectedIds: [String] {
        selection?.optionIds ?? []
    }

    private var otherText: String? {
        selection?.other
    }

    private var otherSelected: Bool {
        !(otherText ?? "").isEmpty
    }

    private var suppressPrompt: Bool {
        focusHeadlineId == block.id
    }

    /// axes is the shared fact-label sequence, non-nil only when every fact-carrying
    /// option matches; `aligned` renders the comparison grid, gated to regular width so
    /// a compact iPhone deck keeps the self-labeled per-option chips.
    private var axes: [String]? {
        factAxes(block.options)
    }

    private var aligned: Bool {
        axes != nil && sizeClass == .regular
    }

    private var feedback: [Feedback] {
        store.state.interactions.feedback[block.id] ?? []
    }

    private var replies: [Reply] {
        blockReplies[block.id] ?? []
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            if !suppressPrompt, let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .fontWeight(.medium)
                    .foregroundStyle(BlockPalette.ink)
                    .receiptContent()
            }

            optionsGroup

            noteAffordance

            if !feedback.isEmpty || !replies.isEmpty {
                Divider().overlay(BlockPalette.line)
                FeedbackThreadView(feedback: feedback, replies: replies)
                    .receiptContent()
            }
        }
        .onAppear { otherDraft = otherText ?? "" }
        .onChange(of: otherText) { _, now in
            if !otherFocused {
                otherDraft = now ?? ""
            }
        }
        .onChange(of: otherFocused) { _, now in
            focusComposer?.set(block.id, composing: now || noteComposing)
        }
        .onChange(of: noteComposing) { _, now in
            if now {
                noteFocused = true
            }
            focusComposer?.set(block.id, composing: now || otherFocused)
        }
        .onDisappear {
            focusComposer?.set(block.id, composing: false)
        }
    }

    @ViewBuilder
    private var optionsGroup: some View {
        let stack = VStack(alignment: .leading, spacing: 8) {
            if aligned, let axes {
                factAxesHeader(axes)
            }
            ForEach(block.options, id: \.id) { option in
                optionRow(option, isOn: selectedIds.contains(option.id))
            }
            otherRow
        }
        if suppressPrompt {
            stack
                .accessibilityElement(children: .contain)
                .accessibilityLabel(block.prompt ?? "Options")
        } else {
            stack
        }
    }

    private func optionRow(_ option: Block.Option, isOn: Bool) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .top, spacing: 12) {
                OptionIndicator(multi: multi, isOn: isOn)
                    .padding(.top, 2)
                VStack(alignment: .leading, spacing: 4) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(option.label)
                            .font(.body)
                            .fontWeight(.semibold)
                            .foregroundStyle(BlockPalette.ink)
                        if option.recommended == true {
                            RecommendedStamp()
                        }
                    }
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
                Spacer(minLength: 12)
                if aligned, let axes {
                    AlignedFactValues(axes: axes, facts: option.facts ?? [])
                } else if let facts = option.facts, !facts.isEmpty {
                    FactsCluster(facts: facts)
                }
            }
            .contentShape(Rectangle())
            .onTapGesture { toggle(option.id) }
            .accessibilityElement(children: .combine)
            .accessibilityAddTraits(traits(isOn: isOn))
            .accessibilityAction { toggle(option.id) }

            if let detail = option.detail {
                DetailView(detail: detail)
                    .padding(.leading, 30)
            }

            if let visual = option.visual {
                OptionVisualDisclosure(visual: visual, context: packContext, client: client)
                    .padding(.leading, 30)
            }
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
    }

    /// otherRow is the chrome write-in: a selection marker and an inline field committing
    /// on submit. Its selected look follows a committed `other`, and the field mirrors it.
    private var otherRow: some View {
        HStack(alignment: .center, spacing: 12) {
            OptionIndicator(multi: multi, isOn: otherSelected)
            TextField("Other…", text: $otherDraft)
                .textFieldStyle(.plain)
                .font(.body)
                .foregroundStyle(BlockPalette.ink)
                .focused($otherFocused)
                .submitLabel(.done)
                .disabled(locked)
                .onSubmit(commitOther)
                .accessibilityLabel("Other, write-in answer")
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            otherSelected ? BlockPalette.accentInk.opacity(0.08) : Color.clear,
            in: RoundedRectangle(cornerRadius: 8)
        )
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .strokeBorder(otherSelected ? BlockPalette.accentInk : BlockPalette.line)
        )
        .opacity(locked ? 0.55 : 1)
    }

    private func factAxesHeader(_ axes: [String]) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Spacer(minLength: 0)
            HStack(spacing: 10) {
                ForEach(Array(axes.enumerated()), id: \.offset) { _, label in
                    Text(label)
                        .font(.system(size: 10, weight: .medium))
                        .textCase(.uppercase)
                        .tracking(0.4)
                        .foregroundStyle(BlockPalette.muted)
                        .frame(width: factColumnWidth, alignment: .trailing)
                        .multilineTextAlignment(.trailing)
                }
            }
        }
        .padding(.horizontal, 12)
        .accessibilityHidden(true)
    }

    @ViewBuilder
    private var noteAffordance: some View {
        if noteComposing {
            VStack(alignment: .leading, spacing: 10) {
                TextField("Add a note for the agent…", text: $noteDraft, axis: .vertical)
                    .lineLimit(2 ... 5)
                    .font(.subheadline)
                    .padding(10)
                    .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 8))
                    .overlay(
                        RoundedRectangle(cornerRadius: 8).strokeBorder(BlockPalette.line, lineWidth: 1)
                    )
                    .focused($noteFocused)
                    .disabled(locked)
                    .accessibilityLabel("Note for the agent")

                HStack(spacing: 12) {
                    Button("Send", action: sendNote)
                        .buttonStyle(.borderedProminent)
                        .tint(BlockPalette.accentInk)
                        .disabled(locked || noteDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                    Button("Cancel") {
                        noteComposing = false
                        noteDraft = ""
                    }
                    .buttonStyle(.bordered)
                    .tint(BlockPalette.muted)
                }
                .font(.system(size: 14, weight: .semibold))
            }
        } else if !locked {
            Button("Add note") {
                noteComposing = true
            }
            .buttonStyle(.plain)
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(BlockPalette.accentInk)
        }
    }

    private func traits(isOn: Bool) -> AccessibilityTraits {
        var traits: AccessibilityTraits = multi ? .isToggle : .isButton
        if isOn {
            traits.formUnion(.isSelected)
        }
        return traits
    }

    private func toggle(_ optionId: String) {
        guard !locked else { return }
        let post = choiceTogglePost(multi: multi, selectedIds: selectedIds, otherText: otherText, optionId: optionId)
        store.choose(blockId: block.id, optionIds: post.optionIds, other: post.other)
    }

    /// commitOther posts the write-in payload and drops focus so a same-step auto-advance
    /// can fire once the compose guard lifts.
    private func commitOther() {
        guard !locked else { return }
        if let post = choiceOtherPost(multi: multi, selectedIds: selectedIds, otherText: otherText, draft: otherDraft) {
            store.choose(blockId: block.id, optionIds: post.optionIds, other: post.other)
        }
        otherFocused = false
    }

    private func sendNote() {
        guard !locked else { return }
        let text = noteDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        store.feedback(blockId: block.id, text: text)
        noteDraft = ""
        noteComposing = false
    }
}

private let factColumnWidth: CGFloat = 60

/// OptionVisualView renders an option's restricted visual leaf: a diagram through the
/// shared single-block webview, code/diff/image through their native views. The switch
/// is exhaustive, so a new visual type is a compile error. Mirrors the web stage dispatch.
struct OptionVisualView: View {
    let visual: OptionVisual
    var context: PackContext?
    var client: APIClient?

    var body: some View {
        switch visual {
        case let .code(code):
            CodeBlockView(block: code)
        case let .diagram(diagram):
            DiagramBlockView(block: diagram, context: context)
        case let .image(image):
            ImageBlockView(block: image, client: client)
        case let .diff(diff):
            DiffBlockView(block: diff)
        case let .chart(chart):
            ChartBlockView(block: chart, context: context)
        case let .term(term):
            TermBlockView(block: term, context: context)
        case let .filetree(filetree):
            FileTreeBlockView(block: filetree, context: context)
        case let .record(record):
            RecordBlockView(block: record, context: context)
        }
    }
}

/// OptionVisualDisclosure keeps the row compact: the visual sits behind a titled
/// expander that reveals it inline, matching the heavy-context treatment in the deck.
private struct OptionVisualDisclosure: View {
    let visual: OptionVisual
    var context: PackContext?
    var client: APIClient?

    @State private var expanded = false

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            OptionVisualView(visual: visual, context: context, client: client)
                .padding(.top, 8)
        } label: {
            Text(optionVisualTitle(visual))
                .font(.system(.caption, design: .monospaced))
                .foregroundStyle(BlockPalette.accentInk)
                .lineLimit(1)
        }
        .tint(BlockPalette.accentInk)
    }
}

/// optionVisualTitle labels an option-visual disclosure by the visual's own title,
/// falling back to a type name. Mirrors focusContextTitle.
func optionVisualTitle(_ visual: OptionVisual) -> String {
    switch visual {
    case let .code(code): code.title ?? code.lang
    case let .diagram(diagram): diagram.title ?? "Diagram"
    case let .image(image): image.caption ?? image.alt
    case let .diff(diff): diff.title ?? "Diff"
    case let .chart(chart): chart.title ?? "Chart"
    case let .term(term): term.title ?? "Terminal"
    case let .filetree(filetree): filetree.title ?? "Files"
    case let .record(record): record.title ?? "Record"
    }
}

/// ChoicePost is the next `choice.selected` payload a choice UI action produces — the
/// full option-id set plus an optional write-in. Pure so the selection logic is testable
/// apart from the view.
struct ChoicePost: Equatable {
    let optionIds: [String]
    let other: String?
}

/// choiceTogglePost is the payload tapping an authored option produces: single-select
/// replaces or clears the pick and drops any write-in (last-write-wins); multi-select
/// adds or removes the id and keeps the coexisting write-in. Mirrors web `choiceToggle`.
func choiceTogglePost(multi: Bool, selectedIds: [String], otherText: String?, optionId: String) -> ChoicePost {
    let nextIds: [String] = if multi {
        selectedIds.contains(optionId) ? selectedIds.filter { $0 != optionId } : selectedIds + [optionId]
    } else {
        selectedIds.contains(optionId) ? [] : [optionId]
    }
    return ChoicePost(optionIds: nextIds, other: multi ? otherText : nil)
}

/// choiceWriteInMaxBytes caps a write-in at the daemon's human-text limit
/// (internal/daemon/rest.go maxHumanTextBytes); an over-cap draft never posts.
let choiceWriteInMaxBytes = 64 << 10

/// choiceWriteInVisuallyEmpty mirrors the daemon's visuallyEmpty: a string is blank when
/// every scalar is Unicode whitespace or a format (Cf) character — a zero-width space or
/// joiner — so it reads as empty to a human even though it is not "".
func choiceWriteInVisuallyEmpty(_ text: String) -> Bool {
    for scalar in text.unicodeScalars where !scalar.properties.isWhitespace && scalar.properties.generalCategory != .format {
        return false
    }
    return true
}

/// choiceOtherPost is the payload committing the write-in produces, or nil when nothing is
/// postable. Single-select replaces the authored picks with the write-in, multi keeps both.
/// It mirrors the daemon — a visually-empty draft clears any prior write-in (else no-op) and
/// an over-cap draft never posts — so an invalid write-in is dropped, not applied then rolled back.
func choiceOtherPost(multi: Bool, selectedIds: [String], otherText: String?, draft: String) -> ChoicePost? {
    let trimmed = draft.trimmingCharacters(in: .whitespacesAndNewlines)
    if choiceWriteInVisuallyEmpty(trimmed) {
        return otherText != nil ? ChoicePost(optionIds: selectedIds, other: nil) : nil
    }
    guard trimmed.utf8.count <= choiceWriteInMaxBytes else { return nil }
    return ChoicePost(optionIds: multi ? selectedIds : [], other: trimmed)
}

/// RecommendedStamp is the small-caps badge marking an author's suggested option,
/// mirroring the web `.option-reco` stamp beside the label.
private struct RecommendedStamp: View {
    var body: some View {
        Text("Recommended")
            .font(.system(size: 9, weight: .semibold, design: .monospaced))
            .textCase(.uppercase)
            .tracking(0.5)
            .foregroundStyle(BlockPalette.accentInk)
            .padding(.vertical, 2)
            .padding(.horizontal, 5)
            .overlay(Capsule().strokeBorder(BlockPalette.accentInk.opacity(0.5)))
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

/// AlignedFactValues renders an option's fact values only, one per axis in fixed-width
/// trailing columns so they line up under the shared axes header. A missing fact leaves
/// its column blank, keeping every option's values in the same columns.
private struct AlignedFactValues: View {
    let axes: [String]
    let facts: [Block.Fact]

    var body: some View {
        HStack(spacing: 10) {
            ForEach(Array(axes.enumerated()), id: \.offset) { index, _ in
                let fact = facts.indices.contains(index) ? facts[index] : nil
                Text(fact?.value ?? "")
                    .font(.system(size: 14, weight: .semibold, design: .rounded))
                    .foregroundStyle(factToneColor(fact?.tone))
                    .frame(width: factColumnWidth, alignment: .trailing)
                    .multilineTextAlignment(.trailing)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

/// FactsCluster renders an option's Tier-1 facts on the row's trailing edge: each
/// fact's value reads prominent and tone-tinted, with its optional label as a dim
/// uppercase eyebrow beneath.
private struct FactsCluster: View {
    let facts: [Block.Fact]

    var body: some View {
        VStack(alignment: .trailing, spacing: 6) {
            ForEach(Array(facts.enumerated()), id: \.offset) { _, fact in
                VStack(alignment: .trailing, spacing: 1) {
                    Text(fact.value)
                        .font(.system(size: 14, weight: .semibold, design: .rounded))
                        .foregroundStyle(factToneColor(fact.tone))
                        .multilineTextAlignment(.trailing)
                    if let label = fact.label, !label.isEmpty {
                        Text(label)
                            .font(.system(size: 10, weight: .medium))
                            .textCase(.uppercase)
                            .tracking(0.4)
                            .foregroundStyle(BlockPalette.muted)
                            .multilineTextAlignment(.trailing)
                    }
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}

private func factToneColor(_ tone: String?) -> Color {
    switch tone {
    case "good": BlockPalette.approve
    case "warn": BlockPalette.warn
    case "bad": BlockPalette.reject
    default: BlockPalette.ink
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
        Block.Option(
            id: "opt-a",
            label: "Rebase onto main",
            hint: "keeps history linear",
            facts: [
                Block.Fact(label: "conflicts", value: "0", tone: "good"),
                Block.Fact(label: "commits", value: "4"),
            ],
            detail: Block.Detail(
                pros: ["Keeps history linear", "Easy to bisect"],
                cons: ["Rewrites shared commits"],
                md: "Rebase replays each commit onto the new base, so the branch **disappears** from the graph.",
                mode: "inline"
            ),
            recommended: true
        ),
        Block.Option(
            id: "opt-b",
            label: "Merge commit",
            hint: "preserves the branch shape",
            md: "Adds a merge node so the two lines of work stay **visible** in the graph.",
            facts: [
                Block.Fact(label: "conflicts", value: "2", tone: "warn"),
                Block.Fact(label: "commits", value: "5", tone: "bad"),
            ],
            detail: Block.Detail(
                pros: ["Preserves the branch shape"],
                cons: ["Adds a merge node", "Noisier graph"],
                mode: "modal"
            )
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
