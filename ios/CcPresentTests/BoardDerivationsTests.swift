@testable import CcPresentApp
import CcPresentKit
import Testing

private func card(_ id: String, children: [Block]) -> Block {
    .card(Block.Card(id: id, children: children))
}

private func pack(_ id: String, _ type: String) -> Block {
    .pack(Block.Pack(id: id, packType: type, raw: .object(["id": .string(id), "type": .string(type)])))
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

@Test func isDecidedTreatsStoredPackInteractionAsDecided() {
    let block = pack("pk", "ex.rating")

    #expect(isDecided(block, Interactions()) == false)
    #expect(isDecided(block, Interactions(packs: ["pk": PackValue(payload: .object(["value": .int(5)]))])) == true)
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
