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
