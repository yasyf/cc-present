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

    /// next lands on the nearest undecided step after the cursor, wrapping across
    /// the deck (mirroring nextUndecided); it settles on the summary only when
    /// nothing is undecided.
    func next(_ steps: [FocusStep], _ interactions: Interactions) {
        guard !steps.isEmpty else { return }
        let from = index(steps)
        for hop in 1 ... steps.count {
            let idx = (from + hop) % steps.count
            if stepUndecided(steps[idx], interactions) {
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

    /// scheduleAdvance arms the 450ms approval auto-advance; the timer no-ops if
    /// navigation left the armed step or the feedback composer opened meanwhile.
    func scheduleAdvance(armed: String) {
        cancelAdvance()
        advance = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .milliseconds(450))
            guard let self, !Task.isCancelled, !composer.isComposing, anchorId == armed else { return }
            move(steps, 1)
        }
    }
}

private struct AdvanceKey: Equatable {
    let stepId: String
    let decided: Bool
}

private struct FocusMetricKey: PreferenceKey {
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
    var client: APIClient?
    var packContext: PackContext?

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var model: FocusDeckModel
    @State private var stageWidth: CGFloat = 360

    init(steps: [FocusStep], store: BoardStore, client: APIClient? = nil, packContext: PackContext? = nil) {
        self.steps = steps
        self.store = store
        self.client = client
        self.packContext = packContext
        _model = State(initialValue: FocusDeckModel(anchorId: steps.first?.id ?? deckEnd))
    }

    private var interactions: Interactions {
        store.state.interactions
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
        guard !store.isClosed, let step = currentStep, step.swipeable, let pid = step.primary?.id else { return nil }
        return AdvanceKey(stepId: step.id, decided: interactions.decisions[pid] != nil)
    }

    var body: some View {
        VStack(spacing: 16) {
            FocusProgressView(steps: steps, index: index, interactions: interactions, onJump: jump)
            stage
            FocusNavView(
                index: index,
                total: steps.count,
                onBack: { model.move(steps, -1) },
                onSkip: { model.move(steps, 1) },
                onNext: { model.next(steps, interactions) }
            )
        }
        .frame(maxWidth: .infinity)
        .environment(\.focusComposer, model.composer)
        .onAppear { model.reconcile(steps) }
        .onChange(of: steps.map(\.id)) { _, _ in model.reconcile(steps) }
        .onChange(of: round) { _, _ in model.reset(steps) }
        .onChange(of: advanceKey) { old, new in
            model.cancelAdvance()
            guard let new, new.decided, let old, old.stepId == new.stepId, !old.decided else { return }
            model.scheduleAdvance(armed: new.stepId)
        }
    }

    private var stage: some View {
        ZStack(alignment: .top) {
            if !onSummary, index + 1 < steps.count {
                FocusPeekView(step: steps[index + 1])
                    .scaleEffect(0.96)
                    .offset(y: 10)
                    .opacity(0.45)
                    .allowsHitTesting(false)
            }
            if onSummary {
                FocusSummaryView(steps: steps, interactions: interactions, onJump: jump)
            } else if let step = currentStep {
                card(step)
                    .id("\(round):\(step.id)")
            }
        }
        .frame(maxWidth: .infinity)
        .background(
            GeometryReader { proxy in
                Color.clear.preference(key: FocusMetricKey.self, value: proxy.size.width)
            }
        )
        .onPreferenceChange(FocusMetricKey.self) { stageWidth = $0 }
    }

    @ViewBuilder
    private func card(_ step: FocusStep) -> some View {
        if step.swipeable, case let .approval(approval)? = step.primary, !store.isClosed {
            SwipeableFocusCard(
                step: step,
                store: store,
                client: client,
                packContext: packContext,
                approvalId: approval.id,
                stageWidth: stageWidth,
                reduceMotion: reduceMotion
            )
        } else {
            FocusCardView(step: step, store: store, client: client, packContext: packContext)
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
    let onJump: (String) -> Void

    private let railMax = 10

    var body: some View {
        let total = steps.count
        let shown = min(index + 1, total)
        let tier = index < total ? steps[index].tier : nil
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Text("STEP \(shown) / \(total)")
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
                HStack(spacing: 6) {
                    ForEach(Array(steps.enumerated()), id: \.element.id) { position, step in
                        dot(step, position: position)
                    }
                    Spacer(minLength: 0)
                }
            } else {
                bar(shown: shown, total: total)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func dot(_ step: FocusStep, position: Int) -> some View {
        Button { onJump(step.id) } label: {
            Circle()
                .fill(fill(stepStatus(step, interactions)))
                .frame(width: 9, height: 9)
                .overlay(
                    Circle().strokeBorder(
                        position == index ? BlockPalette.accentInk : BlockPalette.borderStrong,
                        lineWidth: position == index ? 2 : 1
                    )
                )
                .frame(width: 22, height: 22)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Step \(position + 1): \(stepTitle(step))")
    }

    private func fill(_ status: StepStatus?) -> Color {
        switch status {
        case .approved: BlockPalette.approve
        case .rejected: BlockPalette.reject
        case .decided: BlockPalette.accentInk
        case .undecided, nil: BlockPalette.chipBg
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
    }
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

/// FocusCardView renders the live step body — its lead-in context then the focal
/// block through the shared BlockView, so decidables stay interactive. The body
/// scrolls inside a capped area when it overflows (including a PackBlockWebView
/// whose self-reported height exceeds the card). Mirrors web/src/components/FocusCard.tsx.
struct FocusCardView: View {
    let step: FocusStep
    let store: BoardStore
    var client: APIClient?
    var packContext: PackContext?

    private let cardMaxHeight: CGFloat = 480
    @State private var contentHeight: CGFloat = 260

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let tier = step.tier, !tier.isEmpty {
                Text(tier.uppercased())
                    .font(.system(.caption2, design: .monospaced))
                    .foregroundStyle(BlockPalette.accentInk)
            }
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    ForEach(step.context, id: \.id) { block in
                        BlockView(block: block, store: store, client: client, packContext: packContext)
                    }
                    BlockView(block: step.block, store: store, client: client, packContext: packContext)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(
                    GeometryReader { proxy in
                        Color.clear.preference(key: FocusMetricKey.self, value: proxy.size.height)
                    }
                )
            }
            .frame(height: min(contentHeight, cardMaxHeight))
            .scrollBounceBehavior(.basedOnSize)
            .onPreferenceChange(FocusMetricKey.self) { contentHeight = $0 }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(BlockPalette.monoBg, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(BlockPalette.line))
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
    let approvalId: String
    let stageWidth: CGFloat
    let reduceMotion: Bool

    @GestureState private var drag: CGSize = .zero
    @State private var committed: CGSize = .zero
    @State private var opacity: Double = 1

    private let commitDistance: CGFloat = 120
    private let predictedCommit: CGFloat = 250

    private var offset: CGSize {
        CGSize(width: drag.width + committed.width, height: drag.height + committed.height)
    }

    var body: some View {
        FocusCardView(step: step, store: store, client: client, packContext: packContext)
            .overlay(alignment: .topLeading) {
                verdictLabel("APPROVE", color: BlockPalette.approve, tilt: -12, magnitude: offset.width)
            }
            .overlay(alignment: .topTrailing) {
                verdictLabel("REJECT", color: BlockPalette.reject, tilt: 12, magnitude: -offset.width)
            }
            .offset(offset)
            .rotationEffect(.degrees(offset.width / 28))
            .opacity(opacity)
            .gesture(swipe)
    }

    private var swipe: some Gesture {
        DragGesture(minimumDistance: 12)
            .updating($drag) { value, state, _ in
                state = value.translation
            }
            .onEnded { value in
                if commits(value) {
                    commit(direction: value.translation.width > 0 ? 1 : -1, from: value.translation)
                } else {
                    committed = value.translation
                    withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) { committed = .zero }
                }
            }
    }

    private func commits(_ value: DragGesture.Value) -> Bool {
        abs(value.translation.width) > commitDistance || abs(value.predictedEndTranslation.width) > predictedCommit
    }

    private func commit(direction: CGFloat, from translation: CGSize) {
        store.decide(blockId: approvalId, verdict: direction > 0 ? .approved : .rejected)
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
    let onJump: (String) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("REVIEW")
                .font(.system(.caption, design: .monospaced).weight(.semibold))
                .foregroundStyle(BlockPalette.muted)
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
        let status = stepStatus(step, interactions)
        return HStack {
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
        .padding(.vertical, 10)
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
