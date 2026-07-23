@testable import CcPresentApp
import Foundation
import Testing

private func preferenceDefaults() -> UserDefaults {
    let suite = "CcPresentPreferencesTests.\(UUID().uuidString)"
    let defaults = UserDefaults(suiteName: suite)!
    defaults.removePersistentDomain(forName: suite)
    return defaults
}

private let exactSchemaJSON =
    #"{"fingerprint":"f8418bf7f90369f282a0a051d2a358d37e1e0c9a57f731f34d4f07df4a3925d0","identity":"cc-present-ios-preferences-v1","version":1}"#

private func preferenceData(schema: String = exactSchemaJSON, views: String = #"{"alpha":"focus"}"#) -> Data {
    Data(#"{"schema":\#(schema),"views":\#(views)}"#.utf8)
}

@Test("missing exact state uses the product default without reading legacy keys")
private func missingPreferencesUseDefault() throws {
    let defaults = preferenceDefaults()
    defaults.set("focus", forKey: "cc-present:view:alpha")

    #expect(try loadViewOverride(subject: "alpha", defaults: defaults) == nil)
    #expect(defaults.string(forKey: "cc-present:view:alpha") == "focus")
    #expect(defaults.object(forKey: appPreferencesKey) == nil)
}

@Test("preferences round-trip one consolidated exact v1 envelope")
private func preferencesRoundTrip() throws {
    let defaults = preferenceDefaults()

    try saveViewOverride(subject: "beta", mode: .board, defaults: defaults)
    try saveViewOverride(subject: "alpha", mode: .focus, defaults: defaults)

    #expect(try loadViewOverride(subject: "alpha", defaults: defaults) == .focus)
    #expect(try loadViewOverride(subject: "beta", defaults: defaults) == .board)
    #expect(defaults.data(forKey: appPreferencesKey) == preferenceData(views: #"{"alpha":"focus","beta":"board"}"#))
}

@Test("foreign and corrupt preferences fail without deletion or replacement")
private func invalidPreferencesRemain() {
    let invalid: [(String, Data)] = [
        ("corrupt", Data("{".utf8)),
        ("foreign identity", preferenceData(schema: exactSchemaJSON.replacingOccurrences(of: "cc-present-ios-preferences-v1", with: "other-v1"))),
        ("wrong version", preferenceData(schema: exactSchemaJSON.replacingOccurrences(of: #""version":1"#, with: #""version":2"#))),
        ("wrong fingerprint", preferenceData(schema: exactSchemaJSON.replacingOccurrences(of: appPreferencesSchema.fingerprint, with: "wrong"))),
        ("unknown envelope key", Data(#"{"legacy":true,"schema":\#(exactSchemaJSON),"views":{"alpha":"focus"}}"#.utf8)),
        ("unknown schema key", preferenceData(schema: String(exactSchemaJSON.dropLast()) + #", "legacy":true}"#)),
        ("null views", preferenceData(views: "null")),
        ("wrong views type", preferenceData(views: "[]")),
        ("wrong view value", preferenceData(views: #"{"alpha":"grid"}"#)),
        ("empty subject", preferenceData(views: #"{"":"focus"}"#)),
        ("trailing", preferenceData() + Data("{}".utf8)),
        ("duplicate", Data(#"{"schema":\#(exactSchemaJSON),"views":{"alpha":"focus"},"views":{"alpha":"board"}}"#.utf8)),
    ]

    for (name, raw) in invalid {
        let defaults = preferenceDefaults()
        defaults.set(raw, forKey: appPreferencesKey)

        #expect(throws: (any Error).self, "case: \(name)") {
            try loadViewOverride(subject: "alpha", defaults: defaults)
        }
        #expect(throws: (any Error).self, "case: \(name)") {
            try saveViewOverride(subject: "alpha", mode: .board, defaults: defaults)
        }
        #expect(defaults.data(forKey: appPreferencesKey) == raw, "case: \(name)")
    }
}

@Test("wrong UserDefaults value type fails without deletion")
private func wrongPreferenceStorageTypeRemains() {
    let defaults = preferenceDefaults()
    defaults.set("focus", forKey: appPreferencesKey)

    #expect(throws: AppPreferencesError.wrongStoredType) {
        try loadViewOverride(subject: "alpha", defaults: defaults)
    }
    #expect(defaults.string(forKey: appPreferencesKey) == "focus")
}
