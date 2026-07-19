@testable import CcPresentApp
import CcPresentKit
import Testing

private func card(_ id: String, children: [Block]) -> Block {
    .card(Block.Card(id: id, children: children))
}

private func pack(_ id: String, _ type: String) -> Block {
    .pack(Block.Pack(id: id, packType: type, raw: .object(["id": .string(id), "type": .string(type)])))
}

private func section(_ id: String) -> Block {
    .section(Block.Section(id: id, title: "Section \(id)"))
}

private func markdown(_ id: String) -> Block {
    .markdown(Block.Markdown(id: id, md: "text"))
}

private func approval(_ id: String) -> Block {
    .approval(Block.Approval(id: id))
}

@Test func flattenInlinesCardChildrenOneLevel() {
    let blocks: [Block] = [
        .approval(Block.Approval(id: "top-ap")),
        card("card", children: [
            .choice(Block.Choice(id: "child-ch", options: [Block.Option(id: "o1", label: "A")])),
            .markdown(Block.Markdown(id: "child-md", md: "note")),
        ]),
    ]

    let ids = flatten(blocks).map(\.id)

    #expect(ids == ["top-ap", "card", "child-ch", "child-md"])
}

@Test func submitItemsTalliesApprovalsAndChoicesInOrder() {
    let blocks: [Block] = [
        .approval(Block.Approval(id: "ap1")),
        card("card", children: [
            .approval(Block.Approval(id: "ap2")),
            .choice(Block.Choice(id: "ch1", options: [Block.Option(id: "o1", label: "A")])),
            .input(Block.Input(id: "in1", label: "note")),
        ]),
    ]
    let interactions = Interactions(
        decisions: ["ap1": Decision(verdict: "approved")],
        choices: ["ch1": Selection(optionIds: ["o1"])]
    )

    let items = submitItems(blocks, interactions, [])

    #expect(items.map(\.id) == ["ap1", "ap2", "ch1"])
    #expect(items.map(\.kind) == [.approval, .approval, .choice])
    #expect(items.map(\.decided) == [true, false, true])
}

@Test func submitItemsTalliesInteractivePackButSkipsStaticPack() {
    let blocks: [Block] = [
        pack("r1", "ex.rating"),
        pack("c1", "ex.callout"),
        .approval(Block.Approval(id: "ap1")),
    ]
    let interactions = Interactions(packs: ["r1": PackValue(payload: .object(["value": .int(4)]))])

    let items = submitItems(blocks, interactions, ["ex.rating"])

    #expect(items.map(\.id) == ["r1", "ap1"])
    #expect(items.map(\.kind) == [.pack, .approval])
    #expect(items.map(\.decided) == [true, false])
}

@Test func isDecidedTreatsEmptyChoiceSelectionAsUndecided() {
    let choice = Block.choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")]))

    #expect(isDecided(choice, Interactions(choices: ["ch": Selection(optionIds: [])])) == false)
    #expect(isDecided(choice, Interactions(choices: ["ch": Selection(optionIds: ["o1"])])) == true)
}

@Test func isDecidedTreatsOtherOnlyChoiceSelectionAsDecided() {
    let choice = Block.choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")]))

    #expect(isDecided(choice, Interactions(choices: ["ch": Selection(optionIds: [], other: "custom")])) == true)
}

@Test func isDecidedTreatsStoredPackInteractionAsDecided() {
    let block = pack("pk", "ex.rating")

    #expect(isDecided(block, Interactions()) == false)
    #expect(isDecided(block, Interactions(packs: ["pk": PackValue(payload: .object(["value": .int(5)]))])) == true)
}

private struct BlockDecidedCase {
    let name: String
    let block: Block
    let interactions: Interactions
    let packInteractive: Set<String>
    let decided: Bool
}

private let blockDecidedCases: [BlockDecidedCase] = [
    BlockDecidedCase(
        name: "undecided approval",
        block: .approval(Block.Approval(id: "ap")),
        interactions: Interactions(),
        packInteractive: [],
        decided: false
    ),
    BlockDecidedCase(
        name: "decided approval",
        block: .approval(Block.Approval(id: "ap")),
        interactions: Interactions(decisions: ["ap": Decision(verdict: "approved")]),
        packInteractive: [],
        decided: true
    ),
    BlockDecidedCase(
        name: "empty choice selection",
        block: .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
        interactions: Interactions(choices: ["ch": Selection(optionIds: [])]),
        packInteractive: [],
        decided: false
    ),
    BlockDecidedCase(
        name: "picked choice",
        block: .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
        interactions: Interactions(choices: ["ch": Selection(optionIds: ["o1"])]),
        packInteractive: [],
        decided: true
    ),
    BlockDecidedCase(
        name: "other-only choice selection",
        block: .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
        interactions: Interactions(choices: ["ch": Selection(optionIds: [], other: "custom")]),
        packInteractive: [],
        decided: true
    ),
    BlockDecidedCase(
        name: "markdown never decides",
        block: .markdown(Block.Markdown(id: "md", md: "note")),
        interactions: Interactions(),
        packInteractive: [],
        decided: false
    ),
    BlockDecidedCase(
        name: "card with every decidable decided",
        block: card("card", children: [
            .approval(Block.Approval(id: "ap")),
            .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
            .markdown(Block.Markdown(id: "md", md: "note")),
        ]),
        interactions: Interactions(
            decisions: ["ap": Decision(verdict: "approved")],
            choices: ["ch": Selection(optionIds: ["o1"])]
        ),
        packInteractive: [],
        decided: true
    ),
    BlockDecidedCase(
        name: "card with one decidable outstanding",
        block: card("card", children: [
            .approval(Block.Approval(id: "ap")),
            .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
        ]),
        interactions: Interactions(decisions: ["ap": Decision(verdict: "approved")]),
        packInteractive: [],
        decided: false
    ),
    BlockDecidedCase(
        name: "card with no decidables",
        block: card("card", children: [.markdown(Block.Markdown(id: "md", md: "note"))]),
        interactions: Interactions(),
        packInteractive: [],
        decided: false
    ),
    BlockDecidedCase(
        name: "interactive pack decided",
        block: pack("pk", "ex.rating"),
        interactions: Interactions(packs: ["pk": PackValue(payload: .object(["value": .int(4)]))]),
        packInteractive: ["ex.rating"],
        decided: true
    ),
    BlockDecidedCase(
        name: "static pack never decides",
        block: pack("pk", "ex.rating"),
        interactions: Interactions(packs: ["pk": PackValue(payload: .object(["value": .int(4)]))]),
        packInteractive: [],
        decided: false
    ),
]

@Test("blockDecided receipts a row only when it holds decidables and all are decided", arguments: blockDecidedCases)
private func blockDecidedReceiptsFullyDecidedRows(_ testCase: BlockDecidedCase) {
    #expect(
        blockDecided(testCase.block, testCase.interactions, testCase.packInteractive) == testCase.decided,
        "case: \(testCase.name)"
    )
}

@Test func roundTallyCountsVerdictsPicksAndNotes() {
    let record = RoundRecord(
        number: 2,
        blocks: [
            .approval(Block.Approval(id: "ap1")),
            .approval(Block.Approval(id: "ap2")),
            .choice(Block.Choice(id: "ch1", options: [Block.Option(id: "o1", label: "A")])),
            .input(Block.Input(id: "in1", label: "note")),
        ],
        decisions: ["ap1": Decision(verdict: "approved"), "ap2": Decision(verdict: "rejected")],
        choices: ["ch1": Selection(optionIds: ["o1"])],
        inputs: ["in1": InputValue(text: "  filled  ", round: 2)],
        feedback: ["ap1": [Feedback(id: "f1", text: "one"), Feedback(id: "f2", text: "two")]]
    )

    let tally = roundTally(record)

    #expect(tally == RoundTally(approved: 1, rejected: 1, picks: 1, notes: 3))
}

@Test func roundTallyCountsOtherOnlyChoiceSelectionAsPick() {
    let record = RoundRecord(
        number: 4,
        blocks: [
            .choice(Block.Choice(id: "ch1", options: [Block.Option(id: "o1", label: "A")])),
        ],
        choices: ["ch1": Selection(optionIds: [], other: "custom")]
    )

    let tally = roundTally(record)

    #expect(tally == RoundTally(approved: 0, rejected: 0, picks: 1, notes: 0))
}

@Test func roundTallyCountsInteractedPackBlockAsPick() {
    let record = RoundRecord(
        number: 3,
        blocks: [
            .pack(Block.Pack(id: "pk1", packType: "ex.rating", raw: .object(["id": .string("pk1"), "type": .string("ex.rating")]))),
            .pack(Block.Pack(id: "pk2", packType: "ex.rating", raw: .object(["id": .string("pk2"), "type": .string("ex.rating")]))),
        ],
        packs: ["pk1": PackValue(payload: .object(["value": .int(5)]))]
    )

    let tally = roundTally(record)

    #expect(tally == RoundTally(approved: 0, rejected: 0, picks: 1, notes: 0))
}

private struct ReplyThreadCase {
    let name: String
    let block: Block
    let shows: Bool
}

private let replyThreadCases: [ReplyThreadCase] = [
    ReplyThreadCase(name: "section", block: .section(Block.Section(id: "s", title: "Header")), shows: true),
    ReplyThreadCase(name: "card", block: .card(Block.Card(id: "c", children: [])), shows: true),
    ReplyThreadCase(name: "approval", block: .approval(Block.Approval(id: "ap")), shows: false),
    ReplyThreadCase(
        name: "choice",
        block: .choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")])),
        shows: true
    ),
    ReplyThreadCase(name: "input", block: .input(Block.Input(id: "in", label: "Note")), shows: true),
    ReplyThreadCase(name: "markdown", block: .markdown(Block.Markdown(id: "md", md: "hi")), shows: true),
    ReplyThreadCase(name: "code", block: .code(Block.Code(id: "cd", lang: "swift", code: "let x = 1")), shows: true),
    ReplyThreadCase(name: "diff", block: .diff(Block.Diff(id: "df", diff: "@@ -1 +1 @@")), shows: true),
    ReplyThreadCase(
        name: "diagram",
        block: .diagram(Block.Diagram(id: "dg", kind: "mermaid", source: "graph TD; A-->B")),
        shows: true
    ),
    ReplyThreadCase(name: "image", block: .image(Block.Image(id: "im", src: "https://example.com/a.png", alt: "alt")), shows: true),
    ReplyThreadCase(
        name: "table",
        block: .table(Block.Table(id: "tb", columns: [Block.Column(key: "k", label: "K")], rows: [])),
        shows: true
    ),
    ReplyThreadCase(
        name: "progress",
        block: .progress(Block.Progress(id: "pg", label: "Build", value: 1, max: 2)),
        shows: true
    ),
    ReplyThreadCase(
        name: "pack",
        block: .pack(Block.Pack(id: "pk", packType: "ex.rating", raw: .object(["id": .string("pk"), "type": .string("ex.rating")]))),
        shows: false
    ),
]

@Test("showsNativeReplyThread hides the thread only for approval and pack", arguments: replyThreadCases)
private func showsNativeReplyThreadHidesOnlyApprovalAndPack(_ testCase: ReplyThreadCase) {
    #expect(showsNativeReplyThread(testCase.block) == testCase.shows, "case: \(testCase.name)")
}

private struct SectionGroupCase {
    let name: String
    let blocks: [Block]
    let ids: [String]
    let headerIDs: [String?]
    let blockIDs: [[String]]
}

private let sectionGroupCases: [SectionGroupCase] = [
    SectionGroupCase(name: "empty", blocks: [], ids: [], headerIDs: [], blockIDs: []),
    SectionGroupCase(
        name: "leading-only",
        blocks: [markdown("m1"), approval("a1")],
        ids: ["lead"],
        headerIDs: [nil],
        blockIDs: [["m1", "a1"]]
    ),
    SectionGroupCase(
        name: "section-first",
        blocks: [section("s1")],
        ids: ["s#s1"],
        headerIDs: ["s1"],
        blockIDs: [[]]
    ),
    SectionGroupCase(
        name: "section-then-blocks",
        blocks: [section("s1"), markdown("m1"), approval("a1")],
        ids: ["s#s1"],
        headerIDs: ["s1"],
        blockIDs: [["m1", "a1"]]
    ),
    SectionGroupCase(
        name: "interleaved",
        blocks: [markdown("lead1"), section("s1"), markdown("b1"), approval("b2"), section("s2"), markdown("c1")],
        ids: ["lead", "s#s1", "s#s2"],
        headerIDs: [nil, "s1", "s2"],
        blockIDs: [["lead1"], ["b1", "b2"], ["c1"]]
    ),
    SectionGroupCase(
        name: "trailing-empty-section",
        blocks: [section("s1"), markdown("m1"), section("s2")],
        ids: ["s#s1", "s#s2"],
        headerIDs: ["s1", "s2"],
        blockIDs: [["m1"], []]
    ),
    SectionGroupCase(
        name: "sections-only",
        blocks: [section("s1"), section("s2")],
        ids: ["s#s1", "s#s2"],
        headerIDs: ["s1", "s2"],
        blockIDs: [[], []]
    ),
    SectionGroupCase(
        name: "card-block-rides-in-body",
        blocks: [section("sec"), card("card", children: [markdown("cm"), approval("ca")])],
        ids: ["s#sec"],
        headerIDs: ["sec"],
        blockIDs: [["card"]]
    ),
    SectionGroupCase(
        name: "lead-sentinel-collision",
        blocks: [markdown("m"), section("__lead__")],
        ids: ["lead", "s#__lead__"],
        headerIDs: [nil, "__lead__"],
        blockIDs: [["m"], []]
    ),
]

@Test("sectionGroups splits top-level blocks into header-led runs", arguments: sectionGroupCases)
private func sectionGroupsSplitsIntoHeaderLedRuns(_ testCase: SectionGroupCase) {
    let groups = sectionGroups(testCase.blocks)
    #expect(groups.map(\.id) == testCase.ids, "case: \(testCase.name)")
    #expect(groups.map(\.header?.id) == testCase.headerIDs, "case: \(testCase.name)")
    #expect(groups.map { $0.blocks.map(\.id) } == testCase.blockIDs, "case: \(testCase.name)")
    let reconstructed = groups.flatMap { ($0.header.map { [Block.section($0)] } ?? []) + $0.blocks }
    #expect(reconstructed == testCase.blocks, "case: \(testCase.name)")
}
