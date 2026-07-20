import CcPresentKit
import SwiftUI

/// FocusComposer is the deck-scoped signal the auto-advance timer consults: an
/// approval's feedback composer registers itself while open so a verdict's 450ms
/// advance is cancelled mid-compose, the native mirror of the web `data-composing`
/// attribute. FocusDeckView owns one and injects it; board mode leaves it nil.
@MainActor
@Observable
final class FocusComposer {
    private var open: Set<String> = []

    var isComposing: Bool {
        !open.isEmpty
    }

    func set(_ id: String, composing: Bool) {
        if composing {
            open.insert(id)
        } else {
            open.remove(id)
        }
    }
}

extension EnvironmentValues {
    /// focusComposer is the open-composer signal ApprovalBlockView writes to while
    /// its feedback composer is up, so the focus deck can hold its auto-advance.
    @Entry var focusComposer: FocusComposer? = nil

    /// focusHeadlineId names the decidable whose prompt the focus card hoisted into the
    /// step headline, so ChoiceBlockView/ApprovalBlockView suppress the now-duplicate
    /// inline prompt (keeping it as an accessibility label). nil in board mode.
    @Entry var focusHeadlineId: String? = nil

    /// inFocusCard is true for the focal block of a focus step, so CardView drops its
    /// head — the card's title, status, and chips are hoisted into the deck's meta row.
    /// false in board mode, where the card renders its head as authored.
    @Entry var inFocusCard: Bool = false
}

/// FocusDeckModel owns the deck's position as an anchor block id (re-derived to an
/// index on every recompute and clamped when the anchor vanishes) plus the approval
/// auto-advance timer. It mirrors the ref-driven bookkeeping of
/// web/src/components/FocusDeck.tsx: navigation methods take the freshest step list
/// from the view, and the async advance reads the copy pushed in by `reconcile`.
@MainActor
@Observable
final class FocusDeckModel {
    var anchorId: String

    @ObservationIgnored var lastIndex = 0
    @ObservationIgnored private var steps: [FocusStep] = []
    @ObservationIgnored private var advance: Task<Void, Never>?

    let composer = FocusComposer()

    init(anchorId: String) {
        self.anchorId = anchorId
    }

    private func index(_ steps: [FocusStep]) -> Int {
        deckIndex(steps, currentId: anchorId, lastIndex: lastIndex)
    }

    func go(_ steps: [FocusStep], to target: Int) {
        let clamped = max(0, min(target, steps.count))
        lastIndex = min(clamped, max(0, steps.count - 1))
        anchorId = clamped >= steps.count ? deckEnd : steps[clamped].id
    }

    func move(_ steps: [FocusStep], _ delta: Int) {
        go(steps, to: index(steps) + delta)
    }

    /// next lands on the nearest undecided step after the cursor, wrapping across the
    /// deck. A momentum first pass prefers steps the agent is not actively `revising`;
    /// a revising step stays reachable on the fallback pass, never locked out. It
    /// settles on the summary only when nothing is undecided. Mirrors the web next().
    func next(_ steps: [FocusStep], _ interactions: Interactions, _ packInteractive: Set<String>, _ revising: Set<String>) {
        guard !steps.isEmpty else { return }
        let from = index(steps)
        for hop in 1 ... steps.count {
            let idx = (from + hop) % steps.count
            if stepUndecided(steps[idx], interactions, packInteractive), !revising.contains(steps[idx].id) {
                go(steps, to: idx)
                return
            }
        }
        for hop in 1 ... steps.count {
            let idx = (from + hop) % steps.count
            if stepUndecided(steps[idx], interactions, packInteractive) {
                go(steps, to: idx)
                return
            }
        }
        go(steps, to: steps.count)
    }

    func jump(_ steps: [FocusStep], to id: String) {
        guard let idx = steps.firstIndex(where: { $0.id == id || $0.decidables.contains(id) }) else { return }
        go(steps, to: idx)
    }

    func reset(_ steps: [FocusStep]) {
        cancelAdvance()
        self.steps = steps
        lastIndex = 0
        anchorId = steps.first?.id ?? deckEnd
    }

    /// reconcile pushes the freshest step list in and re-anchors after a recompute:
    /// a vanished anchor clamps to the nearest surviving step (using the stale last
    /// index), then the anchor is canonicalized to that step's id.
    func reconcile(_ steps: [FocusStep]) {
        self.steps = steps
        let idx = deckIndex(steps, currentId: anchorId, lastIndex: lastIndex)
        lastIndex = min(idx, max(0, steps.count - 1))
        let want = idx < steps.count ? steps[idx].id : deckEnd
        if want != anchorId {
            anchorId = want
        }
    }

    func cancelAdvance() {
        advance?.cancel()
        advance = nil
    }

    /// reconcileAdvance arms or cancels the 450ms auto-advance from a change in the
    /// current step's decision signature, mirroring the web advanceKey effect: an echo
    /// (unchanged key) keeps an armed timer, an emptied signature or step change cancels,
    /// and a same-step move to a non-empty signature (a pick or re-pick) arms a fresh one.
    func reconcileAdvance(from old: AdvanceKey?, to new: AdvanceKey?) {
        guard old != new else { return }
        guard let new, !new.signature.isEmpty, let old, old.stepId == new.stepId else {
            cancelAdvance()
            return
        }
        scheduleAdvance(armed: new.stepId)
    }

    /// scheduleAdvance arms the 450ms approval auto-advance; the timer no-ops if
    /// navigation left the armed step or the feedback composer opened meanwhile.
    private func scheduleAdvance(armed: String) {
        cancelAdvance()
        advance = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .milliseconds(450))
            guard let self, !Task.isCancelled, !composer.isComposing, anchorId == armed else { return }
            move(steps, 1)
        }
    }
}

/// AdvanceKey is a lone-decidable step's auto-advance trigger: the step's id and its
/// decision signature (empty = undecided) — the verdict for a lone approval, the joined
/// option ids plus write-in for a lone single-select choice. A same-step move to a
/// non-empty signature arms the timer; the deck feeds successive keys to
/// `reconcileAdvance`.
struct AdvanceKey: Equatable {
    let stepId: String
    let signature: String
}

private struct FocusStageWidthKey: PreferenceKey {
    static let defaultValue: CGFloat = 0
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = max(value, nextValue())
    }
}

private struct FocusContentHeightKey: PreferenceKey {
    static let defaultValue: CGFloat = 0
    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = max(value, nextValue())
    }
}

/// FocusDeckView is the tinder-style deck: a progress header, a centered focal card
/// with the next step peeking behind it, and Back/Skip/Next controls. It keys the
/// card on `round:stepId` so a mid-deck round close remounts at step 0, and drives
/// the approval auto-advance off an undecided→decided transition. Mirrors
/// web/src/components/FocusDeck.tsx.
struct FocusDeckView: View {
    let steps: [FocusStep]
    let store: BoardStore
    let packInteractive: Set<String>
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var model: FocusDeckModel
    @State private var stageWidth: CGFloat = 360
    @AccessibilityFocusState private var cardFocused: Bool

    init(
        steps: [FocusStep],
        store: BoardStore,
        packInteractive: Set<String>,
        client: APIClient? = nil,
        packContext: PackContext? = nil
    ) {
        self.steps = steps
        self.store = store
        self.packInteractive = packInteractive
        self.client = client
        self.packContext = packContext
        _model = State(initialValue: FocusDeckModel(anchorId: steps.first?.id ?? deckEnd))
    }

    private var interactions: Interactions {
        store.state.interactions
    }

    /// revisions is the store-owned seen store: the store feeds it per SSE frame, the
    /// deck reads it read-only so marks survive view identity and mode switches.
    private var revisions: RevisionState {
        store.revisions
    }

    private var round: Int {
        store.state.rounds.current
    }

    private var index: Int {
        deckIndex(steps, currentId: model.anchorId, lastIndex: model.lastIndex)
    }

    private var currentStep: FocusStep? {
        index < steps.count ? steps[index] : nil
    }

    private var onSummary: Bool {
        index >= steps.count
    }

    private var advanceKey: AdvanceKey? {
        guard !store.isClosed, let step = currentStep, autoAdvances(step) else { return nil }
        return AdvanceKey(stepId: step.id, signature: advanceSignature(step, interactions))
    }

    var body: some View {
        VStack(spacing: 16) {
            FocusProgressView(
                steps: steps,
                index: index,
                interactions: interactions,
                packInteractive: packInteractive,
                revisions: revisions,
                onJump: jump
            )
            if let note = revisions.docDraftingNote {
                FocusDraftingLine(note: note)
            }
            stage
            FocusNavView(
                index: index,
                total: steps.count,
                onBack: { model.move(steps, -1) },
                onSkip: { model.move(steps, 1) },
                onNext: { model.next(steps, interactions, packInteractive, Set(revisions.revising.blockIds)) }
            )
        }
        .frame(maxWidth: .infinity)
        .environment(\.focusComposer, model.composer)
        .onAppear { model.reconcile(steps) }
        .onChange(of: steps.map(\.id)) { _, _ in model.reconcile(steps) }
        .onChange(of: round) { _, _ in model.reset(steps) }
        .onChange(of: advanceKey) { old, new in model.reconcileAdvance(from: old, to: new) }
        .onChange(of: index) { _, _ in announceStep() }
        .onChange(of: currentStep?.id) { old, _ in
            if let old {
                revisions.markSeen(old)
            }
        }
        .onChange(of: steps.count) { old, new in
            if new > old {
                AccessibilityNotification.Announcement("Deck grew to \(new) steps").post()
            }
        }
    }

    /// announceStep speaks the new step and moves VoiceOver focus onto the freshly
    /// mounted card (or the review summary), mirroring the web deck's announce +
    /// focus move on every step change.
    private func announceStep() {
        let message = currentStep.map { "Step \(index + 1) of \(steps.count) — \(stepTitle($0))" } ?? "Review"
        AccessibilityNotification.Announcement(message).post()
        cardFocused = true
    }

    private var stage: some View {
        ZStack(alignment: .top) {
            if !onSummary, index + 1 < steps.count {
                FocusPeekView(step: steps[index + 1])
                    .scaleEffect(0.96)
                    .offset(y: 10)
                    .opacity(0.45)
                    .allowsHitTesting(false)
                    .accessibilityHidden(true)
            }
            if onSummary {
                FocusSummaryView(
                    steps: steps,
                    interactions: interactions,
                    packInteractive: packInteractive,
                    revisions: revisions,
                    onJump: jump
                )
                .accessibilityFocused($cardFocused)
            } else if let step = currentStep {
                card(step)
                    .id("\(round):\(step.id)")
                    .accessibilityFocused($cardFocused)
            }
        }
        .frame(maxWidth: .infinity)
        .background(
            GeometryReader { proxy in
                Color.clear.preference(key: FocusStageWidthKey.self, value: proxy.size.width)
            }
        )
        .onPreferenceChange(FocusStageWidthKey.self) { stageWidth = $0 }
    }

    @ViewBuilder
    private func card(_ step: FocusStep) -> some View {
        if step.swipeable, case let .approval(approval)? = step.primary, !store.isClosed {
            SwipeableFocusCard(
                step: step,
                store: store,
                client: client,
                packContext: packContext,
                revisions: revisions,
                approvalId: approval.id,
                stageWidth: stageWidth,
                reduceMotion: reduceMotion
            )
        } else {
            FocusCardView(step: step, store: store, client: client, packContext: packContext, revisions: revisions)
        }
    }

    private func jump(_ id: String) {
        model.jump(steps, to: id)
    }
}

/// FocusProgressView is the deck header: a mono step counter, the tier label, and a
/// tap-to-jump dot rail that fills decided dots with the verdict color, collapsing
/// to a proportional bar past ten steps. Mirrors web/src/components/FocusProgress.tsx.
struct FocusProgressView: View {
    let steps: [FocusStep]
    let index: Int
    let interactions: Interactions
    let packInteractive: Set<String>
    let revisions: RevisionState
    let onJump: (String) -> Void

    private let railMax = 10

    var body: some View {
        let total = steps.count
        let onSummary = index >= total
        let shown = min(index + 1, total)
        let tier = index < total ? steps[index].tier : nil
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Text(onSummary ? "REVIEW" : "STEP \(shown) / \(total)")
                    .font(.system(.caption, design: .monospaced).weight(.semibold))
                    .foregroundStyle(BlockPalette.muted)
                if let tier, !tier.isEmpty {
                    Text(tier.uppercased())
                        .font(.system(.caption2, design: .monospaced))
                        .foregroundStyle(BlockPalette.accentInk)
                        .lineLimit(1)
                }
                Spacer(minLength: 0)
            }
            if total <= railMax {
                HStack(spacing: 4) {
                    ForEach(Array(steps.enumerated()), id: \.element.id) { position, step in
                        dot(step, position: position)
                    }
                }
            } else {
                bar(shown: shown, total: total)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func dot(_ step: FocusStep, position: Int) -> some View {
        let status = stepStatus(step, interactions, packInteractive)
        let look = dotAppearance(status)
        let current = position == index
        let revState = revisionDotState(
            isRevising: revisions.isRevising(step.id),
            changeKind: revisions.mark(for: step.id)?.kind
        )
        // The current ring wins the 9pt slot; a non-current step shows its revision
        // overlay there instead, so the two never stack.
        return Button { onJump(step.id) } label: {
            Circle()
                .fill(look.fill)
                .frame(width: look.size, height: look.size)
                .overlay {
                    if !current, let stroke = look.stroke {
                        Circle().strokeBorder(stroke, lineWidth: 1)
                    }
                }
                .frame(width: 9, height: 9)
                .overlay {
                    if current {
                        Circle().strokeBorder(BlockPalette.accentInk, lineWidth: 2)
                    } else {
                        RevisionDotOverlay(state: revState)
                    }
                }
                .frame(width: 22, height: 22)
                .frame(maxWidth: .infinity, minHeight: 44)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(dotLabel(step, position: position, status: status, revision: revState))
        .accessibilityAddTraits(current ? [.isSelected] : [])
    }

    private func dotLabel(_ step: FocusStep, position: Int, status: StepStatus?, revision: RevisionDotState) -> String {
        let base = "Step \(position + 1): \(stepTitle(step)), \(status?.rawValue ?? "no decision")"
        guard let phrase = revisionPhrase(revision) else { return base }
        return "\(base), \(phrase)"
    }

    private func revisionPhrase(_ state: RevisionDotState) -> String? {
        switch state {
        case .revising: "being revised"
        case .added: "new step"
        case .changed: "updated since you saw it"
        case .none: nil
        }
    }

    private func bar(shown: Int, total: Int) -> some View {
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                Capsule().fill(BlockPalette.chipBg)
                Capsule()
                    .fill(BlockPalette.accentInk)
                    .frame(width: proxy.size.width * CGFloat(shown) / CGFloat(max(total, 1)))
            }
        }
        .frame(height: 4)
        .accessibilityElement()
        .accessibilityLabel("Step progress")
        .accessibilityValue("Step \(shown) of \(total)")
    }
}

/// DotAppearance is the resolved look of one progress dot — the glyph diameter, its
/// fill, and an optional 1pt border. Derived purely from a step's StepStatus so the
/// mapping stays table-testable apart from the view.
struct DotAppearance: Equatable {
    let size: CGFloat
    let fill: Color
    let stroke: Color?
}

/// dotAppearance maps tally status to the dot glyph, mirroring the web dot rail
/// (web/src/styles/focus.css): nil is a small filled tick, undecided a hollow
/// warn-bordered ring, and each decided verdict a filled dot.
func dotAppearance(_ status: StepStatus?) -> DotAppearance {
    switch status {
    case nil:
        DotAppearance(size: 4, fill: BlockPalette.borderStrong, stroke: nil)
    case .undecided:
        DotAppearance(size: 9, fill: .clear, stroke: BlockPalette.warn)
    case .approved:
        DotAppearance(size: 9, fill: BlockPalette.approve, stroke: BlockPalette.borderStrong)
    case .rejected:
        DotAppearance(size: 9, fill: BlockPalette.reject, stroke: BlockPalette.borderStrong)
    case .decided:
        DotAppearance(size: 9, fill: BlockPalette.accentInk, stroke: BlockPalette.borderStrong)
    }
}

/// RevisionDotOverlay draws a non-current progress dot's live-revision ring: a pulsing
/// ring while the step is being revised, a heavier ring plus an insert badge for an
/// added step, a hairline ring for a changed-since-seen step. Reduce Motion drops the
/// pulse. Mirrors the web rail's revising/added/changed dot forms.
private struct RevisionDotOverlay: View {
    let state: RevisionDotState

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var pulse = false

    var body: some View {
        switch state {
        case .none:
            EmptyView()
        case .revising:
            Circle()
                .strokeBorder(BlockPalette.accentInk, lineWidth: 1.5)
                .opacity(reduceMotion ? 1 : (pulse ? 0.3 : 1))
                .animation(
                    reduceMotion ? nil : .easeInOut(duration: 0.8).repeatForever(autoreverses: true),
                    value: pulse
                )
                .onAppear { pulse = true }
                .onDisappear { pulse = false }
        case .added:
            Circle()
                .strokeBorder(BlockPalette.accentInk, lineWidth: 2)
                .overlay(alignment: .topTrailing) {
                    Circle()
                        .fill(BlockPalette.accentInk)
                        .frame(width: 4, height: 4)
                        .offset(x: 1.5, y: -1.5)
                }
        case .changed:
            Circle().strokeBorder(BlockPalette.accentInk, lineWidth: 1)
        }
    }
}

/// FocusDraftingLine is the quiet doc-level "working" note the deck shows under its
/// progress header while the agent drafts a step that has no block to mark yet.
private struct FocusDraftingLine: View {
    let note: String

    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: "pencil.and.outline")
                .font(.caption2)
            Text(note)
                .font(.footnote)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .foregroundStyle(BlockPalette.muted)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}

/// RevisingBanner is the warn-only strip above a step the agent is rewriting: controls
/// stay live, so it never disables anything. After 120s with no resolution it decays to
/// a muted "may be out of date" line — the dead-agent fallback. Mirrors web RevisionCallout.
private struct RevisingBanner: View {
    let note: String?
    let decayed: Bool

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            if decayed {
                Image(systemName: "clock.badge.questionmark")
                    .font(.caption2)
                Text("This step may be out of date")
                    .font(.caption)
            } else {
                Image(systemName: "pencil.line")
                    .font(.caption2)
                Text(revisingBannerText(note: note))
                    .font(.caption.weight(.medium))
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
        }
        .foregroundStyle(decayed ? BlockPalette.muted : BlockPalette.warn)
        .padding(.vertical, 6)
        .padding(.horizontal, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            decayed ? BlockPalette.chipBg : BlockPalette.warn.opacity(0.12),
            in: RoundedRectangle(cornerRadius: 8)
        )
        .accessibilityElement(children: .combine)
    }
}

/// RevisionCallout is the arrival notice on a step changed since it was last seen: a
/// distinct lead-in for an added vs revised step, the working-set note appended when
/// present — the formal replacement for the informal "Updated:" prefix.
private struct RevisionCallout: View {
    let mark: RevisionState.Mark

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Image(systemName: mark.kind == .added ? "plus.circle" : "arrow.triangle.2.circlepath")
                .font(.caption2)
            Text(revisionCalloutText(mark))
                .font(.caption.weight(.medium))
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .foregroundStyle(BlockPalette.accentInk)
        .padding(.vertical, 6)
        .padding(.horizontal, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.accentInk.opacity(0.1), in: RoundedRectangle(cornerRadius: 8))
        .accessibilityElement(children: .combine)
    }
}

/// revisingBannerText is the warn banner copy above a step being rewritten, appending
/// the working-set note when present. Mirrors the web revising banner copy.
func revisingBannerText(note: String?) -> String {
    if let note, !note.isEmpty {
        return "Claude is rewriting this step — \(note)"
    }
    return "Claude is rewriting this step"
}

/// revisionCalloutText is the arrival callout copy for a changed step: distinct lead-ins
/// for an added vs revised step, the note appended when present. Mirrors the web callout.
func revisionCalloutText(_ mark: RevisionState.Mark) -> String {
    let lead = mark.kind == .added ? "Claude added this step" : "Updated after your earlier pick"
    if let note = mark.note, !note.isEmpty {
        return "\(lead) — \(note)"
    }
    return lead
}

/// FocusPeekView is the next card showing behind the current one — a facade of card
/// chrome, tier, and title only. It must never mount BlockView: a real next card
/// would register interactions and double-instantiate a pack's WKWebView. Mirrors
/// web/src/components/FocusPeek.tsx.
struct FocusPeekView: View {
    let step: FocusStep

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            if let tier = step.tier, !tier.isEmpty {
                Text(tier.uppercased())
                    .font(.system(.caption2, design: .monospaced))
                    .foregroundStyle(BlockPalette.accentInk)
            }
            Text(stepTitle(step))
                .font(.headline)
                .foregroundStyle(BlockPalette.ink)
                .lineLimit(2)
            if case let .card(card) = step.block, let summary = card.summary, !summary.isEmpty {
                Text(summary)
                    .font(.subheadline)
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(2)
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(BlockPalette.line))
    }
}

/// FocusCardView renders the step body question-first: a hoisted meta row and the step
/// headline pinned above a capped scroll body of demoted context and the focal block.
/// Suppressing the focal decidable's prompt (`focusHeadlineId`) and the focal card's head
/// (`inFocusCard`) keeps the headline the one heading. Mirrors web FocusCard.tsx.
struct FocusCardView: View {
    let step: FocusStep
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?
    let revisions: RevisionState

    private let cardMaxHeight: CGFloat = 480
    @State private var contentHeight: CGFloat = 260

    private var headline: FocusHeadline {
        stepHeadline(step)
    }

    private var card: Block.Card? {
        if case let .card(card) = step.block {
            return card
        }
        return nil
    }

    private var eyebrow: String? {
        // The card title demotes to the eyebrow only when the headline isn't it.
        headline.fromCard ? nil : card?.title
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            metaRow
            if let text = headline.text, !text.isEmpty {
                Text(text)
                    .font(.title3.weight(.semibold))
                    .foregroundStyle(BlockPalette.ink)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityAddTraits(.isHeader)
            }
            if revisions.isRevising(step.id) {
                RevisingBanner(note: revisions.revising.note, decayed: revisions.revisingPassive(step.id))
            }
            if let mark = revisions.mark(for: step.id) {
                RevisionCallout(mark: mark)
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    if !step.context.isEmpty {
                        VStack(alignment: .leading, spacing: 12) {
                            ForEach(step.context, id: \.id) { block in
                                FocusContextBlock(block: block, store: store, client: client, packContext: packContext)
                            }
                        }
                    }
                    BlockView(block: step.block, store: store, client: client, packContext: packContext)
                        .environment(\.focusHeadlineId, headline.suppressId)
                        .environment(\.inFocusCard, true)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(
                    GeometryReader { proxy in
                        Color.clear.preference(key: FocusContentHeightKey.self, value: proxy.size.height)
                    }
                )
            }
            .frame(height: min(contentHeight, cardMaxHeight))
            .scrollBounceBehavior(.basedOnSize)
            .onPreferenceChange(FocusContentHeightKey.self) { contentHeight = $0 }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(BlockPalette.line))
    }

    @ViewBuilder
    private var metaRow: some View {
        let status = card?.status
        let chips = card?.chips ?? []
        let hasMeta = (eyebrow?.isEmpty == false) || (status?.isEmpty == false) || !chips.isEmpty
        if hasMeta {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                if let eyebrow, !eyebrow.isEmpty {
                    Text(eyebrow.uppercased())
                        .font(.system(.caption2, design: .monospaced))
                        .foregroundStyle(BlockPalette.accentInk)
                        .lineLimit(1)
                }
                if let status, !status.isEmpty {
                    FocusMetaStatus(status: status)
                }
                Spacer(minLength: 8)
                if !chips.isEmpty {
                    HStack(spacing: 4) {
                        ForEach(Array(chips.enumerated()), id: \.offset) { _, chip in
                            FocusMetaChip(chip: chip)
                        }
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

/// FocusContextBlock demotes a step's lead-in context: markdown clamps and dims, and a
/// heavy block (code, diff, table, image, diagram, term, filetree) collapses to a
/// one-line titled disclosure. Mirrors web/src/components/FocusCard.tsx `FocusContextBlock`.
struct FocusContextBlock: View {
    let block: Block
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    @State private var expanded = false

    var body: some View {
        switch block {
        case let .markdown(markdown):
            MarkdownText(markdown.md, style: .clamped)
                .frame(maxWidth: .infinity, alignment: .leading)
                .opacity(0.8)
        case .code, .diff, .table, .image, .diagram, .term, .filetree:
            DisclosureGroup(isExpanded: $expanded) {
                BlockView(block: block, store: store, client: client, packContext: packContext)
                    .padding(.top, 8)
            } label: {
                Text(focusContextTitle(block))
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(BlockPalette.accentInk)
                    .lineLimit(1)
            }
            .tint(BlockPalette.accentInk)
        default:
            BlockView(block: block, store: store, client: client, packContext: packContext)
        }
    }
}

/// focusContextTitle labels a demoted heavy context block's disclosure by its authored
/// title, falling back to a type name. Mirrors web `contextTitle`.
func focusContextTitle(_ block: Block) -> String {
    switch block {
    case let .code(code): code.title ?? code.lang
    case let .diff(diff): diff.title ?? "Diff"
    case let .diagram(diagram): diagram.title ?? "Diagram"
    case let .image(image): image.caption ?? image.alt
    case .table: "Table"
    case let .term(term): term.title ?? "Terminal"
    case let .filetree(filetree): filetree.title ?? "Files"
    default: block.type
    }
}

/// FocusMetaStatus renders the hoisted card status pill in the deck meta row, tinting
/// resolved green and the rest accent, mirroring the card head status chip.
private struct FocusMetaStatus: View {
    let status: String

    var body: some View {
        let tint = status == "resolved" ? BlockPalette.approve : BlockPalette.accentInk
        Text(status)
            .font(.caption2)
            .foregroundStyle(tint)
            .padding(.vertical, 2)
            .padding(.horizontal, 8)
            .overlay(Capsule().strokeBorder(tint.opacity(0.45)))
    }
}

/// FocusMetaChip renders one hoisted card chip in the deck meta row, toning `flag` red,
/// `demo` accent, and the default muted, mirroring the card head chips.
private struct FocusMetaChip: View {
    let chip: Block.Chip

    private var foreground: Color {
        switch chip.tone {
        case "flag": BlockPalette.reject
        case "demo": BlockPalette.accentInk
        default: BlockPalette.muted
        }
    }

    var body: some View {
        Text(chip.label)
            .font(.caption2)
            .foregroundStyle(foreground)
            .padding(.vertical, 2)
            .padding(.horizontal, 8)
            .background(chip.tone == "flag" ? BlockPalette.reject.opacity(0.12) : BlockPalette.chipBg, in: Capsule())
            .overlay(Capsule().strokeBorder(chip.tone == "flag" ? BlockPalette.reject.opacity(0.4) : BlockPalette.line))
            .lineLimit(1)
    }
}

/// SwipeableFocusCard wraps a lone-approval FocusCardView with a swipe-to-decide
/// gesture: right approves, left rejects. The card rotates and reveals APPROVE /
/// REJECT overlays as it drags, commits past the distance or predicted-end
/// threshold (stamping the verdict through the same BoardStore.decide the buttons
/// use, then flying off ±1.2× the stage), and snaps back below it. Reduce Motion
/// swaps the fly-off for a fade. The deck's 450ms auto-advance handles the step
/// change, so the verdict is unified across swipe, buttons, and keys.
struct SwipeableFocusCard: View {
    let step: FocusStep
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?
    let revisions: RevisionState
    let approvalId: String
    let stageWidth: CGFloat
    let reduceMotion: Bool

    @Environment(\.focusComposer) private var focusComposer
    @GestureState private var drag: CGSize = .zero
    @State private var committed: CGSize = .zero
    @State private var opacity: Double = 1
    @State private var locked = false

    private let commitDistance: CGFloat = 120
    private let predictedCommit: CGFloat = 250

    private var offset: CGSize {
        CGSize(width: drag.width + committed.width, height: drag.height + committed.height)
    }

    private var decided: Bool {
        store.state.interactions.decisions[approvalId] != nil
    }

    var body: some View {
        FocusCardView(step: step, store: store, client: client, packContext: packContext, revisions: revisions)
            .overlay(alignment: .topLeading) {
                verdictLabel("APPROVE", color: BlockPalette.approve, tilt: -12, magnitude: offset.width)
            }
            .overlay(alignment: .topTrailing) {
                verdictLabel("REJECT", color: BlockPalette.reject, tilt: 12, magnitude: -offset.width)
            }
            .offset(offset)
            .rotationEffect(.degrees(offset.width / 28))
            .opacity(opacity)
            .allowsHitTesting(!locked)
            .gesture(swipe)
            .onChange(of: decided) { _, present in
                // A committed verdict that rolled back (or was cleared elsewhere)
                // cancels the deck's advance, so restore the flown-off card.
                if locked, !present {
                    restore()
                }
            }
    }

    private var swipe: some Gesture {
        DragGesture(minimumDistance: 12)
            .updating($drag) { value, state, _ in
                state = value.translation
            }
            .onEnded { value in
                guard !locked else { return }
                if let direction = commitDirection(value) {
                    commit(direction: direction, from: value.translation)
                } else {
                    snapBack(from: value.translation)
                }
            }
    }

    /// commitDirection is the verdict sign a gesture end commits, or nil to snap back:
    /// the crossing metric — drag distance or the flick's predicted end — sets the
    /// sign, and a translation/flick sign clash (a reversal flick) snaps back rather
    /// than stamp a stale verdict.
    private func commitDirection(_ value: DragGesture.Value) -> CGFloat? {
        let translation = value.translation.width
        let predicted = value.predictedEndTranslation.width
        let byDistance = abs(translation) > commitDistance
        let byFlick = abs(predicted) > predictedCommit
        guard byDistance || byFlick else { return nil }
        if (translation > 0) != (predicted > 0) {
            return nil
        }
        return (byDistance ? translation : predicted) > 0 ? 1 : -1
    }

    private func commit(direction: CGFloat, from translation: CGSize) {
        // The 450ms auto-advance fires only on an undecided→decided flip with no open
        // composer; when it will not, stamp the verdict but keep the card in place so
        // it never strands at opacity 0. The fly-off locks the gesture until the step
        // changes (or the verdict rolls back), so a second flick can't double-post.
        let willAdvance = !decided && !(focusComposer?.isComposing ?? false)
        store.decide(blockId: approvalId, verdict: direction > 0 ? .approved : .rejected)
        guard willAdvance else {
            snapBack(from: translation)
            return
        }
        locked = true
        committed = translation
        if reduceMotion {
            withAnimation(.easeOut(duration: 0.2)) {
                committed = .zero
                opacity = 0
            }
        } else {
            withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) {
                committed = CGSize(width: direction * 1.2 * stageWidth, height: translation.height)
                opacity = 0
            }
        }
    }

    /// snapBack returns the card to center, honoring Reduce Motion with an instant
    /// reset in place of the spring.
    private func snapBack(from translation: CGSize) {
        if reduceMotion {
            committed = .zero
        } else {
            committed = translation
            withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) { committed = .zero }
        }
    }

    /// restore reverses a fly-off whose advance never fired, bringing the card back to
    /// center and re-enabling the gesture.
    private func restore() {
        locked = false
        if reduceMotion {
            committed = .zero
            opacity = 1
        } else {
            withAnimation(.easeOut(duration: 0.2)) {
                committed = .zero
                opacity = 1
            }
        }
    }

    private func verdictLabel(_ text: String, color: Color, tilt: Double, magnitude: CGFloat) -> some View {
        Text(text)
            .font(.system(.title3, design: .monospaced).weight(.heavy))
            .foregroundStyle(color)
            .padding(.vertical, 5)
            .padding(.horizontal, 12)
            .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(color, lineWidth: 3))
            .rotationEffect(.degrees(tilt))
            .padding(22)
            .opacity(Double(max(0, min(1, magnitude / commitDistance))))
    }
}

/// FocusSummaryView is the deck-end receipt: one row per step with its verdict, an
/// undecided row jumping back to its step. The SubmitBar stays mounted below as the
/// single submit path. Mirrors web/src/components/FocusSummary.tsx.
struct FocusSummaryView: View {
    let steps: [FocusStep]
    let interactions: Interactions
    let packInteractive: Set<String>
    let revisions: RevisionState
    let onJump: (String) -> Void

    private var revisingCount: Int {
        revisions.revising.blockIds.count
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("REVIEW")
                .font(.system(.caption, design: .monospaced).weight(.semibold))
                .foregroundStyle(BlockPalette.muted)
            if revisingCount > 0 {
                Label("Claude is still revising \(revisingCount) steps", systemImage: "pencil.line")
                    .font(.caption.weight(.medium))
                    .foregroundStyle(BlockPalette.warn)
            }
            VStack(spacing: 0) {
                ForEach(Array(steps.enumerated()), id: \.element.id) { position, step in
                    receipt(step)
                    if position < steps.count - 1 {
                        Divider().overlay(BlockPalette.line)
                    }
                }
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(BlockPalette.line))
    }

    private func receipt(_ step: FocusStep) -> some View {
        let status = stepStatus(step, interactions, packInteractive)
        let answers = chosenAnswers(step)
        return VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(stepTitle(step))
                    .font(.subheadline)
                    .foregroundStyle(BlockPalette.ink)
                    .lineLimit(1)
                Spacer(minLength: 8)
                if status == .undecided {
                    Button("Decide") { onJump(step.id) }
                        .font(.system(.caption, design: .monospaced).weight(.semibold))
                        .foregroundStyle(BlockPalette.accentInk)
                        .buttonStyle(.plain)
                } else {
                    Text((status?.rawValue ?? "—").uppercased())
                        .font(.system(.caption2, design: .monospaced))
                        .foregroundStyle(color(status))
                }
            }
            if !answers.isEmpty {
                Text(answers.joined(separator: " · "))
                    .font(.footnote)
                    .foregroundStyle(BlockPalette.muted)
                    .lineLimit(2)
            }
            if hasNote(step) {
                Label("note", systemImage: "text.bubble")
                    .font(.caption2)
                    .foregroundStyle(BlockPalette.accentInk)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.vertical, 10)
    }

    /// chosenAnswers reads back a step's decided choices for the receipt: each choice's
    /// selected option labels, then its write-in quoted. Approvals surface as the row's
    /// verdict status, so only choices contribute here.
    private func chosenAnswers(_ step: FocusStep) -> [String] {
        var out: [String] = []
        for block in flatten([step.block]) {
            guard case let .choice(choice) = block, let selection = interactions.choices[choice.id] else { continue }
            var parts = selection.optionIds.compactMap { id in
                choice.options.first { $0.id == id }?.label
            }
            if let other = selection.other, !other.isEmpty {
                parts.append("“\(other)”")
            }
            if !parts.isEmpty {
                out.append(parts.joined(separator: ", "))
            }
        }
        return out
    }

    /// hasNote reports whether any decidable in the step carries human feedback, driving
    /// the receipt's note marker.
    private func hasNote(_ step: FocusStep) -> Bool {
        flatten([step.block]).contains { !(interactions.feedback[$0.id] ?? []).isEmpty }
    }

    private func color(_ status: StepStatus?) -> Color {
        switch status {
        case .approved: BlockPalette.approve
        case .rejected: BlockPalette.reject
        case .decided: BlockPalette.accentInk
        case .undecided, nil: BlockPalette.muted
        }
    }
}

/// FocusNavView is the deck's Back / Skip / Next control. Back steps to the prior
/// card; Skip advances one card without deciding; Next jumps to the nearest
/// undecided step, wrapping, and lands on the review summary when nothing is left.
struct FocusNavView: View {
    let index: Int
    let total: Int
    let onBack: () -> Void
    let onSkip: () -> Void
    let onNext: () -> Void

    private var onSummary: Bool {
        index >= total
    }

    var body: some View {
        HStack(spacing: 10) {
            Button(action: onBack) {
                Label("Back", systemImage: "chevron.left")
            }
            .buttonStyle(.bordered)
            .tint(BlockPalette.muted)
            .disabled(index <= 0)

            Spacer(minLength: 8)

            Button("Skip", action: onSkip)
                .buttonStyle(.bordered)
                .tint(BlockPalette.muted)
                .disabled(onSummary)

            Button(action: onNext) {
                Label("Next", systemImage: "chevron.right")
                    .labelStyle(.titleAndIcon)
            }
            .buttonStyle(.borderedProminent)
            .tint(BlockPalette.accentInk)
            .disabled(total == 0)
        }
        .font(.system(size: 14, weight: .semibold))
        .frame(maxWidth: .infinity)
    }
}
