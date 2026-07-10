@testable import CcPresentApp
import CcPresentKit
import Testing

private func card(_ id: String, children: [Block]) -> Block {
    .card(Block.Card(id: id, children: children))
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

    let items = submitItems(blocks, interactions)

    #expect(items.map(\.id) == ["ap1", "ap2", "ch1"])
    #expect(items.map(\.kind) == [.approval, .approval, .choice])
    #expect(items.map(\.decided) == [true, false, true])
}

@Test func isDecidedTreatsEmptyChoiceSelectionAsUndecided() {
    let choice = Block.choice(Block.Choice(id: "ch", options: [Block.Option(id: "o1", label: "A")]))

    #expect(isDecided(choice, Interactions(choices: ["ch": Selection(optionIds: [])])) == false)
    #expect(isDecided(choice, Interactions(choices: ["ch": Selection(optionIds: ["o1"])])) == true)
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
