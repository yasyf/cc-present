@testable import CcPresentApp
import CcPresentKit
import Testing

@Suite("Draft selection")
struct DraftSelectionTests {
    @Test func firstTapAnchorsTheCursor() {
        #expect(advanceDraftSelection(.idle, tapped: 3) == .selecting(.anchored(3)))
    }

    @Test func reTappingTheAnchorCancels() {
        #expect(advanceDraftSelection(.anchored(3), tapped: 3) == .selecting(.idle))
    }

    @Test func aSecondLineCommitsTheForwardRange() {
        #expect(advanceDraftSelection(.anchored(3), tapped: 7) == .commit(start: 3, end: 7))
    }

    @Test func aBackwardsSweepCommitsTheSameRange() {
        #expect(advanceDraftSelection(.anchored(7), tapped: 3) == .commit(start: 3, end: 7))
    }
}
