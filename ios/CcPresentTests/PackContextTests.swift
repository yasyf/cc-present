@testable import CcPresentApp
import Foundation
import Testing

private let base = URL(string: "http://192.168.1.42:8790")!

@Test func singleBlockURLCarriesTokenWhenPresent() {
    let context = PackContext(baseURL: base, token: "cafebabe", subject: "sess-1")
    #expect(
        context.singleBlockURL(blockId: "ex-rating").absoluteString
            == "http://192.168.1.42:8790/p/sess-1?block=ex-rating&token=cafebabe"
    )
}

@Test func singleBlockURLOmitsTokenWhenAbsent() {
    let context = PackContext(baseURL: base, token: nil, subject: "sess-1")
    #expect(
        context.singleBlockURL(blockId: "ex-rating").absoluteString
            == "http://192.168.1.42:8790/p/sess-1?block=ex-rating"
    )
}

@Test func singleBlockURLPercentEncodesSubjectBlockAndToken() throws {
    let context = try PackContext(baseURL: #require(URL(string: "http://10.0.0.5:8790")), token: "t k", subject: "a/b")
    #expect(
        context.singleBlockURL(blockId: "c d").absoluteString
            == "http://10.0.0.5:8790/p/a%2Fb?block=c%20d&token=t%20k"
    )
}
