@testable import CcPresentUI
import Testing

@Suite("SingleBlockWebView interaction plumbing")
struct SingleBlockWebViewTests {
    @Test("a ccPresentInteraction body yields the interacted block id")
    func interactionReadsBlockId() {
        #expect(SingleBlockWebView.blockId(fromMessageBody: ["type": "interaction", "blockId": "p1"]) == "p1")
    }

    @Test("an interaction body of the wrong shape yields nil")
    func interactionRejectsMalformedBody() {
        #expect(SingleBlockWebView.blockId(fromMessageBody: "p1") == nil)
        #expect(SingleBlockWebView.blockId(fromMessageBody: ["type": "interaction"]) == nil)
        #expect(SingleBlockWebView.blockId(fromMessageBody: ["blockId": 1]) == nil)
    }
}
