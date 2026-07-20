import CcPresentKit
import Foundation

/// FocusStepKind marks whether a step carries a decision or is read-only lead-in.
enum FocusStepKind: Equatable {
    case decision
    case context
}

/// FocusStep is one card of the focus deck: a focal block, its non-decidable
/// lead-in context, and the decidables it groups. A pure mirror of the web
/// `FocusStep` in web/src/focus.ts, so the two derivations stay in lockstep.
struct FocusStep: Equatable {
    /// Anchor block id — the focal block's id, stable across recomputes so the deck
    /// keeps its place when the live document churns.
    let id: String
    let kind: FocusStepKind
    /// Non-decidable lead-in blocks rendered above the focal block.
    let context: [Block]
    /// The card or leaf rendered as the step body.
    let block: Block
    /// Every decidable id under this step in document order (card children inlined).
    let decidables: [String]
    /// The first decidable block; nil for a context step.
    let primary: Block?
    /// A lone approval — the only shape swipe-to-decide targets.
    let swipeable: Bool
    /// The nearest preceding section title.
    let tier: String?
}

/// decidableIds is the focus ring for one slice: every approval, choice, input, and
/// interactive pack block in document order, card children inlined. Mirrors
/// web/src/decide.ts `decidableIds` — the built-in decidables are approval, choice,
/// and input; a pack block decides only when its type is in `packInteractive`.
func decidableIds(_ blocks: [Block], _ packInteractive: Set<String>) -> [String] {
    flatten(blocks).compactMap { block in
        switch block {
        case .approval, .choice, .draft, .triage, .input:
            block.id
        case let .pack(pack):
            packInteractive.contains(pack.packType) ? block.id : nil
        default:
            nil
        }
    }
}

private func isApproval(_ block: Block?) -> Bool {
    if case .approval = block {
        return true
    }
    return false
}

private func startsOwnStep(_ block: Block, _ packInteractive: Set<String>) -> Bool {
    if case .card = block {
        return true
    }
    return !decidableIds([block], packInteractive).isEmpty
}

private func anchorStep(_ block: Block, context: [Block], tier: String?, _ packInteractive: Set<String>) -> FocusStep {
    let decidables = decidableIds([block], packInteractive)
    let primary = decidables.first.flatMap { first in flatten([block]).first { $0.id == first } }
    return FocusStep(
        id: block.id,
        kind: decidables.isEmpty ? .context : .decision,
        context: context,
        block: block,
        decidables: decidables,
        primary: primary,
        swipeable: decidables.count == 1 && isApproval(primary),
        tier: tier
    )
}

private func contextStep(last block: Block, context: [Block], tier: String?) -> FocusStep {
    FocusStep(
        id: block.id,
        kind: .context,
        context: context,
        block: block,
        decidables: [],
        primary: nil,
        swipeable: false,
        tier: tier
    )
}

/// focusSteps walks the top-level blocks in document order. A section flushes the
/// pending run as a standalone context step and updates the tier; a card or
/// decidable leaf becomes a step with the pending run as its context; every other
/// block accumulates into the pending run, and a trailing run is its own context
/// step. A pure mirror of web/src/focus.ts `focusSteps`.
func focusSteps(_ blocks: [Block], _ packInteractive: Set<String>) -> [FocusStep] {
    var steps: [FocusStep] = []
    var pending: [Block] = []
    var tier: String?

    func flush() {
        guard let block = pending.last else { return }
        steps.append(contextStep(last: block, context: Array(pending.dropLast()), tier: tier))
        pending = []
    }

    for block in blocks {
        if case let .section(section) = block {
            flush()
            tier = section.title
            continue
        }
        if startsOwnStep(block, packInteractive) {
            steps.append(anchorStep(block, context: pending, tier: tier, packInteractive))
            pending = []
        } else {
            pending.append(block)
        }
    }
    flush()
    return steps
}

/// stepTitle is the facade label FocusPeekView and FocusSummaryView show without
/// mounting the block — the focal block's own heading, falling back to its kind.
/// Mirrors web/src/focus.ts `stepTitle`.
func stepTitle(_ step: FocusStep) -> String {
    switch step.block {
    case let .card(card):
        card.title ?? "Card"
    case let .approval(approval):
        approval.prompt ?? "Approval"
    case let .choice(choice):
        choice.prompt ?? "Choice"
    case let .draft(draft):
        draft.title ?? "Draft"
    case let .triage(triage):
        triage.prompt ?? "Triage"
    case let .input(input):
        input.label
    case let .section(section):
        section.title
    default:
        "Details"
    }
}

/// FocusHeadline is the hoisted step headline: `text` is the question the deck pins
/// above the scroll body, `suppressId` the decidable whose inline prompt it replaces,
/// `fromCard` whether the text is the step card's own title (so the meta row omits a
/// duplicate eyebrow). Mirrors web/src/focus.ts `FocusHeadline`.
struct FocusHeadline: Equatable {
    let text: String?
    let suppressId: String?
    let fromCard: Bool
}

/// stepHeadline resolves the question a step leads with: a lone decidable's own prompt
/// (hoisted, its inline copy suppressed), else a card's title (multi-decidable prompts
/// stay inline as sub-headings), else nil for a bare content leaf. Mirrors
/// web/src/focus.ts `stepHeadline`.
func stepHeadline(_ step: FocusStep) -> FocusHeadline {
    if step.decidables.count == 1, let primary = step.primary {
        let text: String? = switch primary {
        case let .input(input): input.label
        case let .choice(choice): choice.prompt
        case let .approval(approval): approval.prompt
        default: nil
        }
        if let text, !text.isEmpty {
            return FocusHeadline(text: text, suppressId: primary.id, fromCard: false)
        }
    }
    if case let .card(card) = step.block {
        return FocusHeadline(text: card.title, suppressId: nil, fromCard: true)
    }
    return FocusHeadline(text: nil, suppressId: nil, fromCard: false)
}

/// factAxes is the aligned-grid gate: the shared ordered label list when at least two
/// fact-carrying options declare the same non-empty label sequence, else nil — any
/// mismatch drops the comparison grid and the per-option fallback renders. Mirrors
/// web/src/focus.ts `factAxes`.
func factAxes(_ options: [Block.Option]) -> [String]? {
    let withFacts = options.filter { !($0.facts ?? []).isEmpty }
    guard withFacts.count >= 2 else { return nil }
    let axes = (withFacts[0].facts ?? []).map { $0.label ?? "" }
    if axes.contains(where: \.isEmpty) {
        return nil
    }
    for option in withFacts {
        let labels = (option.facts ?? []).map { $0.label ?? "" }
        if labels != axes {
            return nil
        }
    }
    return axes
}

/// autoAdvances reports whether deciding this step arms the 450ms auto-advance: a lone
/// approval or a lone single-select choice. Multi-decidable cards, multi-select
/// choices, inputs, and packs stay explicit-Next. Mirrors web/src/focus.ts `autoAdvances`.
func autoAdvances(_ step: FocusStep) -> Bool {
    guard step.decidables.count == 1, let primary = step.primary else { return false }
    switch primary {
    case .approval:
        return true
    case let .choice(choice):
        return !(choice.multi ?? false)
    default:
        return false
    }
}

/// advanceSignature is the current lone decidable's decision as a string, empty when
/// undecided (an existing-but-empty selection, a cleared pick, included): a lone
/// approval's verdict, or a single-select choice's option ids joined then a space then
/// its write-in (the space stops an id colliding with an equal write-in). Feedback
/// never enters it. Mirrors the web `decisionSignature`.
func advanceSignature(_ step: FocusStep, _ interactions: Interactions) -> String {
    switch step.primary {
    case let .approval(approval):
        return interactions.decisions[approval.id]?.verdict ?? ""
    case let .choice(choice):
        guard let selection = interactions.choices[choice.id],
              !(selection.optionIds.isEmpty && selection.other == nil) else { return "" }
        return selection.optionIds.joined(separator: ",") + " " + (selection.other ?? "")
    default:
        return ""
    }
}

/// StepStatus classifies a step for the progress dots and summary receipts.
enum StepStatus: String {
    case approved
    case rejected
    case decided
    case undecided
}

/// stepStatus classifies a step: nil for a step with nothing to tally (context
/// runs, input-only steps — inputs are never decided, matching the SubmitBar
/// tally), otherwise decided/undecided, with approve/reject for a lone approval so
/// its dot fills with the verdict color. An interactive pack decision step tallies
/// like any other decidable now that submitItems is pack-aware. Mirrors
/// web/src/focus.ts `stepStatus`.
func stepStatus(_ step: FocusStep, _ interactions: Interactions, _ packInteractive: Set<String>) -> StepStatus? {
    let items = submitItems([step.block], interactions, packInteractive)
    if items.isEmpty {
        return nil
    }
    if !items.allSatisfy(\.decided) {
        return .undecided
    }
    if step.decidables.count == 1, let primary = step.primary, case let .approval(approval) = primary {
        return interactions.decisions[approval.id]?.verdict == "rejected" ? .rejected : .approved
    }
    return .decided
}

/// stepUndecided reports whether a step still has an undecided tally item — the
/// predicate the deck's next-undecided walk and auto-advance guard use.
func stepUndecided(_ step: FocusStep, _ interactions: Interactions, _ packInteractive: Set<String>) -> Bool {
    submitItems([step.block], interactions, packInteractive).contains { !$0.decided }
}

/// deckEnd is the sentinel anchor id for the review summary — never a real block id
/// — so the deck stores one anchor and the summary survives every recompute.
let deckEnd = "__deck_end__"

/// deckIndex resolves a stored anchor to a rendered index: the summary sentinel (or
/// an empty deck) maps past the last step; a vanished anchor clamps to the nearest
/// surviving step, never the summary. Mirrors web/src/components/FocusDeck.tsx
/// `deckIndex`.
func deckIndex(_ steps: [FocusStep], currentId: String, lastIndex: Int) -> Int {
    if currentId == deckEnd || steps.isEmpty {
        return steps.count
    }
    if let idx = steps.firstIndex(where: { $0.id == currentId }) {
        return idx
    }
    return min(lastIndex, steps.count - 1)
}

/// RevisionDotState is the live-revision overlay a progress dot carries, layered above
/// the verdict fill but below the current-step ring: revising (pulsing), an added step,
/// or a changed-since-seen step. Mirrors the web rail's changed/added/revising stack.
enum RevisionDotState: Equatable {
    case none
    case revising
    case added
    case changed
}

/// revisionDotState resolves a step's revision overlay in priority order — revising
/// wins, then an added step, then a changed one. The view layers the current-step ring
/// above it and the verdict fill below. Mirrors web/src/components/FocusProgress.tsx.
func revisionDotState(isRevising: Bool, changeKind: RevisionState.ChangeKind?) -> RevisionDotState {
    if isRevising {
        return .revising
    }
    switch changeKind {
    case .added: return .added
    case .revised: return .changed
    case nil: return .none
    }
}

/// ViewMode is the resolved board presentation: the tinder-style focus deck or the
/// flat board scroll.
enum ViewMode: String {
    case focus
    case board
}

/// resolveViewMode is the precedence the client renders by: an explicit viewer
/// override wins, then the doc's per-push hint, then the derived default — focus
/// when any step decides, board otherwise. Mirrors web/src/viewmode.ts `resolveMode`.
func resolveViewMode(presentation: Doc.Presentation?, override: ViewMode?, steps: [FocusStep]) -> ViewMode {
    if let override {
        return override
    }
    if let presentation {
        return presentation == .focus ? .focus : .board
    }
    return steps.contains { $0.kind == .decision } ? .focus : .board
}

/// viewOverrideKey is the per-subject UserDefaults key holding a viewer's explicit
/// toggle. Mirrors the web `cc-present:view:<ref>` localStorage key.
func viewOverrideKey(subject: String) -> String {
    "cc-present:view:\(subject)"
}

/// loadViewOverride reads a persisted override, treating any non-mode value as absent.
func loadViewOverride(subject: String, defaults: UserDefaults = .standard) -> ViewMode? {
    defaults.string(forKey: viewOverrideKey(subject: subject)).flatMap(ViewMode.init(rawValue:))
}

/// saveViewOverride persists a viewer's explicit toggle so it survives a relaunch.
func saveViewOverride(subject: String, mode: ViewMode, defaults: UserDefaults = .standard) {
    defaults.set(mode.rawValue, forKey: viewOverrideKey(subject: subject))
}

/// presentPackTypes is the all-interactive fallback: every pack type present in the
/// live blocks, treated as interactive. The classification the deck actually renders
/// by comes from the daemon's `/api/packs` manifest; until that response lands (or if
/// it fails) `interactivePackTypes` falls back here so a pack still earns a step.
func presentPackTypes(_ blocks: [Block]) -> Set<String> {
    Set(flatten(blocks).compactMap { block -> String? in
        if case let .pack(pack) = block {
            return pack.packType
        }
        return nil
    })
}

/// interactivePackTypes is the pack-interactivity set BoardScreen renders by: the
/// manifest's declared interactive types once `/api/packs` has answered, else the
/// all-interactive `presentPackTypes` fallback over the live blocks. Mirrors the web
/// registry's progressive load — every pack is interactive until the manifest
/// reclassifies it.
func interactivePackTypes(declared: Set<String>?, blocks: [Block]) -> Set<String> {
    declared ?? presentPackTypes(blocks)
}
