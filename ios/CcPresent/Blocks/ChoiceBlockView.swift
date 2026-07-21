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
    @Environment(\.commentsHost) private var commentsHost

    @State private var otherDraft = ""
    @State private var noteDraft = ""
    @State private var noteComposing = false
    @State private var activeCardId: String?
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

    /// strip opts the options into the horizontal snap carousel at three or more, mirroring
    /// the web `data-strip` threshold; two or fewer keep the vertical stack.
    private var strip: Bool {
        block.options.count >= 3
    }

    /// cardsVisible is how many cards the carousel shows at once — ~2.2 in a regular-width
    /// deck, ~1.15 plus an edge peek on a compact iPhone.
    private var cardsVisible: CGFloat {
        sizeClass == .regular ? 2.2 : 1.15
    }

    /// otherCardId names the trailing write-in card in the scroll-position and dots id space,
    /// namespaced by the block id so it never collides with an authored option id.
    private var otherCardId: String {
        "\(block.id)::__other"
    }

    /// cardIds is the carousel's ordered id list — every authored option then the write-in —
    /// backing both the dot indicators and the `scrollPosition` binding.
    private var cardIds: [String] {
        block.options.map(\.id) + [otherCardId]
    }

    private var feedback: [Feedback] {
        store.state.interactions.feedback[block.id] ?? []
    }

    private var replies: [Reply] {
        blockReplies[block.id] ?? []
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if !suppressPrompt, let prompt = block.prompt, !prompt.isEmpty {
                Text(prompt)
                    .font(.body)
                    .fontWeight(.medium)
                    .foregroundStyle(BlockPalette.ink)
                    .receiptContent()
            }

            optionsGroup

            if let commentsHost {
                CommentChip(feedbackCount: feedback.count, replyCount: replies.count) {
                    commentsHost.present(pin: block.id)
                }
            } else {
                noteAffordance

                if !feedback.isEmpty || !replies.isEmpty {
                    Divider().overlay(BlockPalette.line)
                    FeedbackThreadView(feedback: feedback, replies: replies)
                        .receiptContent()
                }
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
        let group = VStack(alignment: .leading, spacing: Metrics.space3) {
            if strip {
                carousel
            } else {
                ForEach(Array(block.options.enumerated()), id: \.element.id) { index, option in
                    optionCard(option, isOn: selectedIds.contains(option.id), position: index + 1)
                }
                otherCard(position: cardIds.count)
            }
        }
        if suppressPrompt {
            group
                .accessibilityElement(children: .contain)
                .accessibilityLabel(block.prompt ?? "Options")
        } else {
            group
        }
    }

    /// carousel is the horizontal snap strip: each card is width-bounded to show ~2.2
    /// (regular) or ~1.15 + peek (compact) at once, view-aligned snapping, with the leading
    /// card's id driving the dot indicators below the strip.
    private var carousel: some View {
        VStack(alignment: .leading, spacing: Metrics.space3) {
            ScrollView(.horizontal) {
                HStack(alignment: .top, spacing: Metrics.space3) {
                    ForEach(Array(block.options.enumerated()), id: \.element.id) { index, option in
                        optionCard(option, isOn: selectedIds.contains(option.id), position: index + 1)
                            .containerRelativeFrame(.horizontal, alignment: .leading) { width, _ in
                                width / cardsVisible
                            }
                            .id(option.id)
                    }
                    otherCard(position: cardIds.count)
                        .containerRelativeFrame(.horizontal, alignment: .leading) { width, _ in
                            width / cardsVisible
                        }
                        .id(otherCardId)
                }
                .scrollTargetLayout()
            }
            .scrollTargetBehavior(.viewAligned)
            .scrollPosition(id: $activeCardId, anchor: .leading)
            .scrollIndicators(.hidden)

            dots
        }
    }

    /// dots marks the carousel position: one tick per card, the leading card's tick inked
    /// with the accent. Decorative — VoiceOver reads position off each card instead.
    private var dots: some View {
        let current = activeCardId ?? cardIds.first
        return HStack(spacing: Metrics.space2) {
            ForEach(cardIds, id: \.self) { id in
                Circle()
                    .fill(id == current ? BlockPalette.accentInk : BlockPalette.borderStrong)
                    .frame(width: 6, height: 6)
            }
        }
        .frame(maxWidth: .infinity)
        .accessibilityHidden(true)
    }

    private func optionCard(_ option: Block.Option, isOn: Bool, position: Int) -> some View {
        cardChrome(selected: isOn) {
            VStack(alignment: .leading, spacing: Metrics.space3) {
                VStack(alignment: .leading, spacing: Metrics.space2) {
                    HStack(alignment: .top, spacing: Metrics.space3) {
                        OptionIndicator(multi: multi, isOn: isOn)
                            .padding(.top, 2)
                        VStack(alignment: .leading, spacing: Metrics.space1) {
                            HStack(alignment: .firstTextBaseline, spacing: Metrics.space2) {
                                Text(option.label)
                                    .font(.body)
                                    .fontWeight(.semibold)
                                    .foregroundStyle(BlockPalette.ink)
                                if option.recommended == true {
                                    RecommendedStamp()
                                }
                                Spacer(minLength: 0)
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
                    }
                    if let facts = option.facts, !facts.isEmpty {
                        OptionFactRows(facts: facts)
                            .padding(.top, Metrics.space1)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .contentShape(Rectangle())
                .onTapGesture { toggle(option.id) }
                .accessibilityElement(children: .combine)
                .accessibilityAddTraits(traits(isOn: isOn))
                .accessibilityValue("option \(position) of \(cardIds.count)")
                .accessibilityAction { toggle(option.id) }

                if let detail = option.detail {
                    DetailView(detail: detail)
                }
                if let visual = option.visual {
                    OptionVisualDisclosure(visual: visual, context: packContext, client: client)
                }
            }
        }
    }

    /// otherCard is the trailing write-in card: a selection marker and an inline field that
    /// commits on submit, sharing the option cards' chrome and selected accent. Its look
    /// follows a committed `other`, and the field mirrors it.
    private func otherCard(position: Int) -> some View {
        cardChrome(selected: otherSelected) {
            HStack(alignment: .center, spacing: Metrics.space3) {
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
                    .accessibilityValue("option \(position) of \(cardIds.count)")
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    /// cardChrome lays an option card on the raised `cardLift` ground: a hairline border, a
    /// top edge-lift highlight for dark elevation, and the pencil-accent wash plus border
    /// when selected. A locked board dims the whole card.
    private func cardChrome(selected: Bool, @ViewBuilder content: () -> some View) -> some View {
        let shape = RoundedRectangle(cornerRadius: Metrics.radiusLg, style: .continuous)
        return content()
            .padding(Metrics.space4)
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .background {
                shape
                    .fill(BlockPalette.cardLift)
                    .overlay {
                        if selected {
                            shape.fill(BlockPalette.accentInk.opacity(0.1))
                        }
                    }
            }
            .overlay {
                shape.strokeBorder(
                    selected ? BlockPalette.accentInk : BlockPalette.line,
                    lineWidth: selected ? 1.5 : 1
                )
            }
            .overlay {
                shape.strokeBorder(
                    LinearGradient(colors: [BlockPalette.edgeLift, .clear], startPoint: .top, endPoint: .bottom),
                    lineWidth: 1
                )
            }
            .opacity(locked ? 0.55 : 1)
    }

    @ViewBuilder
    private var noteAffordance: some View {
        if noteComposing {
            VStack(alignment: .leading, spacing: Metrics.space3) {
                TextField("Add a note for the agent…", text: $noteDraft, axis: .vertical)
                    .lineLimit(2 ... 5)
                    .font(.subheadline)
                    .padding(Metrics.space3)
                    .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: Metrics.radiusMd))
                    .overlay(
                        RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(BlockPalette.line, lineWidth: 1)
                    )
                    .focused($noteFocused)
                    .disabled(locked)
                    .accessibilityLabel("Note for the agent")

                HStack(spacing: Metrics.space3) {
                    Button("Send", action: sendNote)
                        .buttonStyle(PrimaryButtonStyle())
                        .disabled(locked || noteDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                    Button("Cancel") {
                        noteComposing = false
                        noteDraft = ""
                    }
                    .buttonStyle(GhostButtonStyle(tint: BlockPalette.muted))
                }
            }
        } else if !locked {
            Button("Add note") {
                noteComposing = true
            }
            .buttonStyle(GhostButtonStyle())
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
                .padding(.top, Metrics.space2)
        } label: {
            Text(optionVisualTitle(visual))
                .voice(.mono, .caption)
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
            .voice(.stamp, size: 9, weight: .semibold)
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
            RoundedRectangle(cornerRadius: Metrics.radiusMd, style: .continuous)
                .fill(isOn ? BlockPalette.accentInk : Color.clear)
                .overlay(
                    RoundedRectangle(cornerRadius: Metrics.radiusMd, style: .continuous)
                        .strokeBorder(borderColor, lineWidth: 1.5)
                )
                .overlay {
                    if isOn {
                        Image(systemName: "checkmark")
                            .voice(.prose, size: 10, weight: .bold)
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

/// OptionFactRows renders an option's Tier-1 facts as per-card label/value rows: the dim
/// uppercase label on the leading edge, the tone-tinted value trailing. Each card keeps its
/// facts in factAxes order, replacing the retired cross-row comparison grid.
private struct OptionFactRows: View {
    let facts: [Block.Fact]

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space1) {
            ForEach(Array(facts.enumerated()), id: \.offset) { _, fact in
                HStack(alignment: .firstTextBaseline, spacing: Metrics.space3) {
                    if let label = fact.label, !label.isEmpty {
                        Text(label)
                            .voice(.stamp, size: 10, weight: .medium)
                            .foregroundStyle(BlockPalette.muted)
                    }
                    Spacer(minLength: Metrics.space2)
                    Text(fact.value)
                        .font(.system(size: 14, weight: .semibold, design: .rounded))
                        .foregroundStyle(factToneColor(fact.tone))
                        .multilineTextAlignment(.trailing)
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
