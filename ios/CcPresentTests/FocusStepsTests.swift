@testable import CcPresentApp
import CcPresentKit
import Testing

/// Block constructors mirroring the helpers in web/src/focus.test.ts.
private func approval(_ id: String) -> Block {
    .approval(Block.Approval(id: id))
}

private func choice(_ id: String) -> Block {
    .choice(Block.Choice(id: id, options: [Block.Option(id: "\(id)o1", label: "one")]))
}

private func input(_ id: String) -> Block {
    .input(Block.Input(id: id, label: id))
}

private func markdown(_ id: String) -> Block {
    .markdown(Block.Markdown(id: id, md: id))
}

private func section(_ id: String, _ title: String) -> Block {
    .section(Block.Section(id: id, title: title))
}

private func card(_ id: String, _ children: [Block]) -> Block {
    .card(Block.Card(id: id, children: children))
}

private func pack(_ id: String, _ type: String = "ex.rating") -> Block {
    .pack(Block.Pack(id: id, packType: type, raw: .object(["id": .string(id), "type": .string(type)])))
}

/// Projected is the focus-step shape the web test compares against — id, kind, tier,
/// context ids, decidables, primary id, and swipeable — so the two derivations stay
/// honest.
private struct Projected: Equatable {
    let id: String
    let kind: FocusStepKind
    let tier: String?
    let context: [String]
    let decidables: [String]
    let primary: String?
    let swipeable: Bool
}

private func project(_ step: FocusStep) -> Projected {
    Projected(
        id: step.id,
        kind: step.kind,
        tier: step.tier,
        context: step.context.map(\.id),
        decidables: step.decidables,
        primary: step.primary?.id,
        swipeable: step.swipeable
    )
}

private struct StepsCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let pack: Set<String>
    let expected: [Projected]

    var description: String {
        name
    }
}

private let stepsCases: [StepsCase] = [
    StepsCase(name: "empty doc yields no steps", blocks: [], pack: [], expected: []),
    StepsCase(
        name: "a lone top-level approval is one swipeable decision step",
        blocks: [approval("a1")],
        pack: [],
        expected: [Projected(id: "a1", kind: .decision, tier: nil, context: [], decidables: ["a1"], primary: "a1", swipeable: true)]
    ),
    StepsCase(
        name: "a content run before a decision attaches as its context",
        blocks: [markdown("m1"), markdown("m2"), approval("a1")],
        pack: [],
        expected: [Projected(id: "a1", kind: .decision, tier: nil, context: ["m1", "m2"], decidables: ["a1"], primary: "a1", swipeable: true)]
    ),
    StepsCase(
        name: "a trailing content run is one read-only context step",
        blocks: [approval("a1"), markdown("m1"), markdown("m2")],
        pack: [],
        expected: [
            Projected(id: "a1", kind: .decision, tier: nil, context: [], decidables: ["a1"], primary: "a1", swipeable: true),
            Projected(id: "m2", kind: .context, tier: nil, context: ["m1"], decidables: [], primary: nil, swipeable: false),
        ]
    ),
    StepsCase(
        name: "sections set the tier and never become steps",
        blocks: [section("s1", "First"), card("c1", [approval("c1a")]), section("s2", "Second"), approval("a1")],
        pack: [],
        expected: [
            Projected(id: "c1", kind: .decision, tier: "First", context: [], decidables: ["c1a"], primary: "c1a", swipeable: true),
            Projected(id: "a1", kind: .decision, tier: "Second", context: [], decidables: ["a1"], primary: "a1", swipeable: true),
        ]
    ),
    StepsCase(
        name: "a section flushes a pending run as a standalone context step",
        blocks: [markdown("m1"), section("s1", "Later"), approval("a1")],
        pack: [],
        expected: [
            Projected(id: "m1", kind: .context, tier: nil, context: [], decidables: [], primary: nil, swipeable: false),
            Projected(id: "a1", kind: .decision, tier: "Later", context: [], decidables: ["a1"], primary: "a1", swipeable: true),
        ]
    ),
    StepsCase(
        name: "a card with multiple decidables is one step, not swipeable",
        blocks: [card("c1", [approval("c1a"), choice("c1c"), markdown("c1m")])],
        pack: [],
        expected: [Projected(id: "c1", kind: .decision, tier: nil, context: [], decidables: ["c1a", "c1c"], primary: "c1a", swipeable: false)]
    ),
    StepsCase(
        name: "a card with no decidables is a context step",
        blocks: [card("c1", [markdown("c1m")])],
        pack: [],
        expected: [Projected(id: "c1", kind: .context, tier: nil, context: [], decidables: [], primary: nil, swipeable: false)]
    ),
    StepsCase(
        name: "a content-only board is all context steps",
        blocks: [markdown("m1"), markdown("m2")],
        pack: [],
        expected: [Projected(id: "m2", kind: .context, tier: nil, context: ["m1"], decidables: [], primary: nil, swipeable: false)]
    ),
    StepsCase(
        name: "a top-level input is its own decision step",
        blocks: [input("i1")],
        pack: [],
        expected: [Projected(id: "i1", kind: .decision, tier: nil, context: [], decidables: ["i1"], primary: "i1", swipeable: false)]
    ),
    StepsCase(
        name: "an interactive pack decides while a static pack accumulates",
        blocks: [pack("r1", "ex.rating"), pack("c1", "ex.callout"), approval("a1")],
        pack: ["ex.rating"],
        expected: [
            Projected(id: "r1", kind: .decision, tier: nil, context: [], decidables: ["r1"], primary: "r1", swipeable: false),
            Projected(id: "a1", kind: .decision, tier: nil, context: ["c1"], decidables: ["a1"], primary: "a1", swipeable: true),
        ]
    ),
]

@Test("focusSteps mirrors the web derivation", arguments: stepsCases)
private func focusStepsMatchesWeb(_ testCase: StepsCase) {
    #expect(focusSteps(testCase.blocks, testCase.pack).map(project) == testCase.expected, "case: \(testCase.name)")
}

private struct TitleCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let expected: String

    var description: String {
        name
    }
}

private let titleCases: [TitleCase] = [
    TitleCase(name: "untitled card falls back to Card", blocks: [card("c1", [approval("c1a")])], expected: "Card"),
    TitleCase(name: "approval prompt fallback", blocks: [approval("a1")], expected: "Approval"),
    TitleCase(name: "input label", blocks: [input("i1")], expected: "i1"),
    TitleCase(name: "content run label", blocks: [markdown("m1")], expected: "Details"),
    TitleCase(
        name: "uses a card title when present",
        blocks: [.card(Block.Card(id: "c1", title: "Ship it", children: [approval("c1a")]))],
        expected: "Ship it"
    ),
]

@Test("stepTitle mirrors the web facade label", arguments: titleCases)
private func stepTitleMatchesWeb(_ testCase: TitleCase) {
    let step = focusSteps(testCase.blocks, [])[0]
    #expect(stepTitle(step) == testCase.expected, "case: \(testCase.name)")
}

private struct StatusCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let interactions: Interactions
    let expected: StepStatus?
    var pack: Set<String> = []

    var description: String {
        name
    }
}

private let statusCases: [StatusCase] = [
    StatusCase(name: "undecided approval", blocks: [approval("a1")], interactions: Interactions(), expected: .undecided),
    StatusCase(
        name: "approved approval fills approve",
        blocks: [approval("a1")],
        interactions: Interactions(decisions: ["a1": Decision(verdict: "approved")]),
        expected: .approved
    ),
    StatusCase(
        name: "rejected approval fills reject",
        blocks: [approval("a1")],
        interactions: Interactions(decisions: ["a1": Decision(verdict: "rejected")]),
        expected: .rejected
    ),
    StatusCase(
        name: "decided choice reads decided",
        blocks: [choice("ch1")],
        interactions: Interactions(choices: ["ch1": Selection(optionIds: ["ch1o1"])]),
        expected: .decided
    ),
    StatusCase(name: "input steps never fill", blocks: [input("i1")], interactions: Interactions(), expected: nil),
    StatusCase(name: "context runs never fill", blocks: [markdown("m1")], interactions: Interactions(), expected: nil),
    StatusCase(
        name: "undecided interactive pack reads undecided",
        blocks: [pack("r1", "ex.rating")],
        interactions: Interactions(),
        expected: .undecided,
        pack: ["ex.rating"]
    ),
    StatusCase(
        name: "decided interactive pack reads decided",
        blocks: [pack("r1", "ex.rating")],
        interactions: Interactions(packs: ["r1": PackValue(payload: .object(["value": .int(5)]))]),
        expected: .decided,
        pack: ["ex.rating"]
    ),
    StatusCase(
        name: "a static pack step never fills",
        blocks: [pack("c1", "ex.callout")],
        interactions: Interactions(),
        expected: nil
    ),
]

@Test("stepStatus mirrors the web receipt classification", arguments: statusCases)
private func stepStatusMatchesWeb(_ testCase: StatusCase) {
    let step = focusSteps(testCase.blocks, testCase.pack)[0]
    #expect(stepStatus(step, testCase.interactions, testCase.pack) == testCase.expected, "case: \(testCase.name)")
}

@Test("stepUndecided tracks an interactive pack decision")
private func stepUndecidedTracksPack() {
    let step = focusSteps([pack("r1", "ex.rating")], ["ex.rating"])[0]
    #expect(stepUndecided(step, Interactions(), ["ex.rating"]) == true)
    let decided = Interactions(packs: ["r1": PackValue(payload: .object(["value": .int(5)]))])
    #expect(stepUndecided(step, decided, ["ex.rating"]) == false)
    // A static pack contributes no tally item, so its step is never "undecided".
    let staticStep = focusSteps([pack("c1", "ex.callout")], [])[0]
    #expect(stepUndecided(staticStep, Interactions(), []) == false)
}

private struct ClassifyCase: CustomStringConvertible {
    let name: String
    let declared: Set<String>?
    let blocks: [Block]
    let expected: Set<String>

    var description: String {
        name
    }
}

private let classifyCases: [ClassifyCase] = [
    ClassifyCase(
        name: "pre-fetch falls back to every present pack type as interactive",
        declared: nil,
        blocks: [pack("r1", "ex.rating"), pack("c1", "ex.callout")],
        expected: ["ex.rating", "ex.callout"]
    ),
    ClassifyCase(
        name: "the declared manifest set replaces the fallback once it arrives",
        declared: ["ex.rating"],
        blocks: [pack("r1", "ex.rating"), pack("c1", "ex.callout")],
        expected: ["ex.rating"]
    ),
    ClassifyCase(
        name: "a declared set with nothing interactive classifies no pack",
        declared: [],
        blocks: [pack("c1", "ex.callout")],
        expected: []
    ),
]

@Test("interactivePackTypes progresses from all-interactive to the manifest set", arguments: classifyCases)
private func interactivePackTypesClassifies(_ testCase: ClassifyCase) {
    #expect(interactivePackTypes(declared: testCase.declared, blocks: testCase.blocks) == testCase.expected, "case: \(testCase.name)")
}

// MARK: - Headline resolution

private func promptedChoice(_ id: String, _ prompt: String?, multi: Bool = false) -> Block {
    .choice(Block.Choice(id: id, prompt: prompt, multi: multi, options: [Block.Option(id: "\(id)o1", label: "one")]))
}

private func promptedApproval(_ id: String, _ prompt: String?) -> Block {
    .approval(Block.Approval(id: id, prompt: prompt))
}

private struct HeadlineCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let text: String?
    let suppressId: String?
    let fromCard: Bool

    var description: String {
        name
    }
}

private let headlineCases: [HeadlineCase] = [
    HeadlineCase(
        name: "a lone choice prompt heads and suppresses its inline copy",
        blocks: [promptedChoice("c1", "Pick one?")],
        text: "Pick one?", suppressId: "c1", fromCard: false
    ),
    HeadlineCase(
        name: "a lone approval prompt heads",
        blocks: [promptedApproval("a1", "Ship it?")],
        text: "Ship it?", suppressId: "a1", fromCard: false
    ),
    HeadlineCase(
        name: "a lone input label heads",
        blocks: [input("i1")],
        text: "i1", suppressId: "i1", fromCard: false
    ),
    HeadlineCase(
        name: "a multi-decidable card title heads without suppressing children",
        blocks: [.card(Block.Card(id: "c1", title: "Rollout", children: [promptedApproval("a1", "A?"), promptedApproval("a2", "B?")]))],
        text: "Rollout", suppressId: nil, fromCard: true
    ),
    HeadlineCase(
        name: "a lone-decidable card demotes its title to the eyebrow",
        blocks: [.card(Block.Card(id: "c1", title: "Card title", children: [promptedChoice("ch", "Which transport?")]))],
        text: "Which transport?", suppressId: "ch", fromCard: false
    ),
    HeadlineCase(
        name: "a promptless lone choice yields no headline",
        blocks: [promptedChoice("c1", nil)],
        text: nil, suppressId: nil, fromCard: false
    ),
    HeadlineCase(
        name: "a content leaf yields no headline",
        blocks: [markdown("m1")],
        text: nil, suppressId: nil, fromCard: false
    ),
]

@Test("stepHeadline mirrors the web resolution", arguments: headlineCases)
private func stepHeadlineMatchesWeb(_ testCase: HeadlineCase) {
    let headline = stepHeadline(focusSteps(testCase.blocks, [])[0])
    #expect(headline.text == testCase.text, "case: \(testCase.name)")
    #expect(headline.suppressId == testCase.suppressId, "case: \(testCase.name)")
    #expect(headline.fromCard == testCase.fromCard, "case: \(testCase.name)")
}

// MARK: - Fact-axes gate

private func opt(_ id: String, _ facts: [Block.Fact]? = nil) -> Block.Option {
    Block.Option(id: id, label: id, facts: facts)
}

private func fact(_ label: String?, _ value: String) -> Block.Fact {
    Block.Fact(label: label, value: value)
}

private struct AxesCase: CustomStringConvertible {
    let name: String
    let options: [Block.Option]
    let expected: [String]?

    var description: String {
        name
    }
}

private let axesCases: [AxesCase] = [
    AxesCase(
        name: "matching label sequences yield the axes",
        options: [opt("a", [fact("Latency", "12ms"), fact("Cost", "$5")]), opt("b", [fact("Latency", "80ms"), fact("Cost", "$2")])],
        expected: ["Latency", "Cost"]
    ),
    AxesCase(
        name: "a mismatched label drops the grid",
        options: [opt("a", [fact("Latency", "12ms")]), opt("b", [fact("Cost", "$2")])],
        expected: nil
    ),
    AxesCase(
        name: "differing fact counts drop the grid",
        options: [opt("a", [fact("Latency", "12ms"), fact("Cost", "$5")]), opt("b", [fact("Latency", "80ms")])],
        expected: nil
    ),
    AxesCase(
        name: "an empty label drops the grid",
        options: [opt("a", [fact(nil, "12ms")]), opt("b", [fact(nil, "80ms")])],
        expected: nil
    ),
    AxesCase(
        name: "fewer than two fact-carrying options drops the grid",
        options: [opt("a", [fact("Latency", "12ms")]), opt("b")],
        expected: nil
    ),
    AxesCase(
        name: "a fact-free option is ignored when the rest align",
        options: [opt("a", [fact("Latency", "12ms")]), opt("b", [fact("Latency", "80ms")]), opt("c")],
        expected: ["Latency"]
    ),
]

@Test("factAxes gates the aligned comparison grid", arguments: axesCases)
private func factAxesGates(_ testCase: AxesCase) {
    #expect(factAxes(testCase.options) == testCase.expected, "case: \(testCase.name)")
}

// MARK: - Auto-advance classifier + signature

private struct AutoAdvanceCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    let expected: Bool

    var description: String {
        name
    }
}

private let autoAdvanceCases: [AutoAdvanceCase] = [
    AutoAdvanceCase(name: "a lone approval arms", blocks: [approval("a1")], expected: true),
    AutoAdvanceCase(name: "a lone single-select choice arms", blocks: [promptedChoice("c1", "?")], expected: true),
    AutoAdvanceCase(name: "a multi-select choice never arms", blocks: [promptedChoice("c1", "?", multi: true)], expected: false),
    AutoAdvanceCase(name: "a multi-decidable card never arms", blocks: [card("c1", [approval("a"), approval("b")])], expected: false),
    AutoAdvanceCase(name: "a lone input never arms", blocks: [input("i1")], expected: false),
    AutoAdvanceCase(name: "a context step never arms", blocks: [markdown("m1")], expected: false),
]

@Test("autoAdvances mirrors the web classifier", arguments: autoAdvanceCases)
private func autoAdvancesMatchesWeb(_ testCase: AutoAdvanceCase) {
    #expect(autoAdvances(focusSteps(testCase.blocks, [])[0]) == testCase.expected, "case: \(testCase.name)")
}

@Test("advanceSignature reflects the decision and never counts feedback")
private func advanceSignatureIgnoresFeedback() {
    let step = focusSteps([promptedChoice("c1", "?")], [])[0]
    #expect(advanceSignature(step, Interactions()) == "")
    // The space namespaces ids from the write-in, mirroring web decisionSignature.
    #expect(advanceSignature(step, Interactions(choices: ["c1": Selection(optionIds: ["c1o1"])])) == "c1o1 ")
    #expect(advanceSignature(step, Interactions(choices: ["c1": Selection(optionIds: [], other: "custom")])) == " custom")
    // A picked option id and a write-in of that same text sign differently, so
    // switching between them still re-arms the auto-advance.
    let asPick = advanceSignature(step, Interactions(choices: ["c1": Selection(optionIds: ["custom"])]))
    let asWriteIn = advanceSignature(step, Interactions(choices: ["c1": Selection(optionIds: [], other: "custom")]))
    #expect(asPick != asWriteIn)
    // A note on the choice leaves the signature unchanged — feedback never arms.
    let withNote = Interactions(choices: ["c1": Selection(optionIds: ["c1o1"])], feedback: ["c1": [Feedback(id: "f1", text: "but…")]])
    #expect(advanceSignature(step, withNote) == "c1o1 ")
    let approvalStep = focusSteps([approval("a1")], [])[0]
    #expect(advanceSignature(approvalStep, Interactions(decisions: ["a1": Decision(verdict: "approved")])) == "approved")
}

// MARK: - Choice-selection payloads (escape hatch)

@Test("a single-select tap replaces the pick and clears any write-in")
private func singleSelectTogglePost() {
    #expect(choiceTogglePost(multi: false, selectedIds: [], otherText: nil, optionId: "o1") == ChoicePost(optionIds: ["o1"], other: nil))
    // Re-tapping the sole pick clears it.
    #expect(choiceTogglePost(multi: false, selectedIds: ["o1"], otherText: nil, optionId: "o1") == ChoicePost(optionIds: [], other: nil))
    // A prior write-in is dropped when an authored option is picked.
    #expect(choiceTogglePost(multi: false, selectedIds: [], otherText: "custom", optionId: "o1") == ChoicePost(optionIds: ["o1"], other: nil))
}

@Test("a multi-select tap adds or removes and preserves the write-in")
private func multiSelectTogglePost() {
    #expect(choiceTogglePost(multi: true, selectedIds: ["o1"], otherText: "custom", optionId: "o2") == ChoicePost(optionIds: ["o1", "o2"], other: "custom"))
    #expect(choiceTogglePost(multi: true, selectedIds: ["o1", "o2"], otherText: "custom", optionId: "o1") == ChoicePost(optionIds: ["o2"], other: "custom"))
}

@Test("committing a write-in replaces single-select picks and coexists on multi")
private func writeInPost() {
    // Single-select: the write-in replaces the authored pick (last-write-wins).
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: nil, draft: "  custom ") == ChoicePost(optionIds: [], other: "custom"))
    // Multi-select: the write-in coexists with the picks.
    #expect(choiceOtherPost(multi: true, selectedIds: ["o1"], otherText: nil, draft: "custom") == ChoicePost(optionIds: ["o1"], other: "custom"))
    // An emptied field clears a prior write-in, keeping the picks.
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: "old", draft: "   ") == ChoicePost(optionIds: ["o1"], other: nil))
    // An emptied field with no prior write-in is a no-op.
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: "") == nil)
    // A zero-width-only draft reads as visually empty (mirroring the daemon), so it never
    // posts the invisible text: a no-op with no prior write-in, a clear with one.
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: "\u{200b}\u{200b}") == nil)
    #expect(choiceOtherPost(multi: false, selectedIds: ["o1"], otherText: "old", draft: "\u{200b}") == ChoicePost(optionIds: ["o1"], other: nil))
    // An over-cap draft never posts, so no optimistic apply races the daemon's reject;
    // a draft exactly at the 64 KiB cap is accepted.
    let atCap = String(repeating: "a", count: 64 << 10)
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: atCap) == ChoicePost(optionIds: [], other: atCap))
    #expect(choiceOtherPost(multi: false, selectedIds: [], otherText: nil, draft: atCap + "a") == nil)
}
