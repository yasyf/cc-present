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

    @Test("an http(s) new-window request yields its url for external open")
    func externalURLForHttpLinks() throws {
        let https = try #require(URL(string: "https://example.com/docs"))
        let http = try #require(URL(string: "http://example.com"))
        #expect(SingleBlockWebView.Coordinator.externalURL(for: URLRequest(url: https)) == https)
        #expect(SingleBlockWebView.Coordinator.externalURL(for: URLRequest(url: http)) == http)
    }

    @Test("a non-http scheme is not opened externally")
    func externalURLRejectsOtherSchemes() throws {
        for raw in ["mailto:a@example.com", "ftp://example.com/x", "tel:+15551234567"] {
            let url = try #require(URL(string: raw))
            #expect(SingleBlockWebView.Coordinator.externalURL(for: URLRequest(url: url)) == nil)
        }
    }
}
