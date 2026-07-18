@testable import CcPresentApp
import CcPresentKit
import SwiftUI
import Testing

/// Block constructors mirroring the helpers in FocusStepsTests.swift.
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

private func card(_ id: String, _ children: [Block]) -> Block {
    .card(Block.Card(id: id, children: children))
}

/// DotCase drives a block through the same focusSteps → stepStatus → dotAppearance
/// chain the rail renders, pinning the dot look to status — never step.kind, so an
/// input-only step (a decision kind that never tallies) still reads as a tick.
/// Mirrors web/src/components/FocusProgress.test.tsx.
private struct DotCase: CustomStringConvertible {
    let name: String
    let blocks: [Block]
    var interactions = Interactions()
    var pack: Set<String> = []
    let expected: DotAppearance

    var description: String {
        name
    }
}

private let dotCases: [DotCase] = [
    DotCase(
        name: "a context run is an untallied tick",
        blocks: [markdown("m1")],
        expected: DotAppearance(size: 4, fill: BlockPalette.borderStrong, stroke: nil)
    ),
    DotCase(
        name: "an input-only step is a tick despite its decision kind",
        blocks: [input("i1")],
        expected: DotAppearance(size: 4, fill: BlockPalette.borderStrong, stroke: nil)
    ),
    DotCase(
        name: "an undecided approval is a hollow warn ring",
        blocks: [approval("a1")],
        expected: DotAppearance(size: 9, fill: .clear, stroke: BlockPalette.warn)
    ),
    DotCase(
        name: "an approved approval fills approve, not a tick",
        blocks: [approval("a1")],
        interactions: Interactions(decisions: ["a1": Decision(verdict: "approved")]),
        expected: DotAppearance(size: 9, fill: BlockPalette.approve, stroke: BlockPalette.borderStrong)
    ),
    DotCase(
        name: "a rejected approval fills reject",
        blocks: [approval("a1")],
        interactions: Interactions(decisions: ["a1": Decision(verdict: "rejected")]),
        expected: DotAppearance(size: 9, fill: BlockPalette.reject, stroke: BlockPalette.borderStrong)
    ),
    DotCase(
        name: "a multi-item decided card fills accentInk",
        blocks: [card("c1", [approval("c1a"), choice("c1c")])],
        interactions: Interactions(
            decisions: ["c1a": Decision(verdict: "approved")],
            choices: ["c1c": Selection(optionIds: ["c1co1"])]
        ),
        expected: DotAppearance(size: 9, fill: BlockPalette.accentInk, stroke: BlockPalette.borderStrong)
    ),
]

@Test("dotAppearance mirrors the web dot rail", arguments: dotCases)
private func dotAppearanceMatchesWeb(_ testCase: DotCase) {
    let step = focusSteps(testCase.blocks, testCase.pack)[0]
    let status = stepStatus(step, testCase.interactions, testCase.pack)
    #expect(dotAppearance(status) == testCase.expected, "case: \(testCase.name)")
}
