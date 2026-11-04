@testable import CcPresentApp
import Testing
import UIKit
import WebKit

@MainActor
@Test func webViewsCountEveryNestedWebView() {
    let root = UIView()
    let nested = UIView()
    root.addSubview(WKWebView())
    root.addSubview(nested)
    nested.addSubview(WKWebView())
    nested.addSubview(UILabel())
    #expect(BoardStats.webViews(in: root) == 2)
    #expect(BoardStats.webViews(in: UILabel()) == 0)
}

@MainActor
@Test func contentTimeIsTakenOnce() {
    let stats = BoardStats()
    #expect(stats.contentAfter == nil)
    stats.contentShown()
    let first = stats.contentAfter
    stats.contentShown()
    #expect(first != nil)
    #expect(stats.contentAfter == first)
}
