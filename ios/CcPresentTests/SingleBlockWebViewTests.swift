@testable import CcPresentApp
import CoreGraphics
import Foundation
import Testing

@Suite("SingleBlockWebView height plumbing")
struct SingleBlockWebViewTests {
    @Test("a well-formed ccPresentHeight body yields its px value")
    func heightReadsPxFromBody() {
        #expect(SingleBlockWebView.Coordinator.height(fromMessageBody: ["px": 214]) == 214)
        #expect(SingleBlockWebView.Coordinator.height(fromMessageBody: ["px": 88.5]) == 88.5)
    }

    @Test("a body of the wrong shape yields nil")
    func heightRejectsMalformedBody() {
        #expect(SingleBlockWebView.Coordinator.height(fromMessageBody: "214") == nil)
        #expect(SingleBlockWebView.Coordinator.height(fromMessageBody: ["height": 214]) == nil)
        #expect(SingleBlockWebView.Coordinator.height(fromMessageBody: ["px": "214"]) == nil)
    }

    @Test("a meaningfully different positive height is applied")
    func clampAcceptsMeaningfulChange() {
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: 320) == 320)
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: 141) == 141)
    }

    @Test("a sub-point or non-positive proposal is dropped")
    func clampDropsChurnAndNonPositive() {
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: 140.4) == nil)
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: 140) == nil)
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: 0) == nil)
        #expect(SingleBlockWebView.Coordinator.clampedHeight(current: 140, proposed: -5) == nil)
    }
}
