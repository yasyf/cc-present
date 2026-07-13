@testable import CcPresentApp
import CcPresentKit
import Testing

private let decisionStep = focusSteps([.approval(Block.Approval(id: "a1"))], [])
private let contextStep = focusSteps([.markdown(Block.Markdown(id: "m1", md: "hi"))], [])

private struct ModeCase: CustomStringConvertible {
    let name: String
    let presentation: Doc.Presentation?
    let override: ViewMode?
    let steps: [FocusStep]
    let expected: ViewMode

    var description: String {
        name
    }
}

private let modeCases: [ModeCase] = [
    ModeCase(
        name: "an explicit override beats the hint and the default",
        presentation: .focus,
        override: .board,
        steps: decisionStep,
        expected: .board
    ),
    ModeCase(
        name: "a focus override wins over a board hint",
        presentation: .board,
        override: .focus,
        steps: contextStep,
        expected: .focus
    ),
    ModeCase(
        name: "the doc hint wins when there is no override",
        presentation: .board,
        override: nil,
        steps: decisionStep,
        expected: .board
    ),
    ModeCase(
        name: "a focus hint wins over a content-only default",
        presentation: .focus,
        override: nil,
        steps: contextStep,
        expected: .focus
    ),
    ModeCase(
        name: "the default is focus when any step decides",
        presentation: nil,
        override: nil,
        steps: decisionStep,
        expected: .focus
    ),
    ModeCase(
        name: "the default is board for a content-only deck",
        presentation: nil,
        override: nil,
        steps: contextStep,
        expected: .board
    ),
    ModeCase(
        name: "the default is board for an empty deck",
        presentation: nil,
        override: nil,
        steps: [],
        expected: .board
    ),
]

@Test("resolveViewMode follows override then hint then derived default", arguments: modeCases)
private func resolveViewModeFollowsPrecedence(_ testCase: ModeCase) {
    let mode = resolveViewMode(presentation: testCase.presentation, override: testCase.override, steps: testCase.steps)
    #expect(mode == testCase.expected, "case: \(testCase.name)")
}
