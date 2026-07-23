import Foundation

let appPreferencesKey = "cc-present:preferences:v1"

let appPreferencesSchema = PreferenceSchema(
    identity: "cc-present-ios-preferences-v1",
    version: 1,
    fingerprint: "f8418bf7f90369f282a0a051d2a358d37e1e0c9a57f731f34d4f07df4a3925d0"
)

struct PreferenceSchema: Codable, Equatable {
    let identity: String
    let version: UInt64
    let fingerprint: String
}

struct AppPreferences: Codable, Equatable {
    let schema: PreferenceSchema
    var views: [String: ViewMode]
}

enum AppPreferencesError: LocalizedError, Equatable {
    case wrongStoredType
    case corrupt
    case nonCanonical
    case schema
    case invalidView

    var errorDescription: String? {
        switch self {
        case .wrongStoredType: "Stored preferences have the wrong type. Remove them manually."
        case .corrupt: "Stored preferences are corrupt. Remove them manually."
        case .nonCanonical: "Stored preferences are not exact canonical JSON. Remove them manually."
        case .schema: "Stored preferences do not match exact schema v1. Remove them manually."
        case .invalidView: "Stored preferences contain an invalid board view. Remove them manually."
        }
    }
}

private func preferenceEncoder() -> JSONEncoder {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
    return encoder
}

private func loadPreferences(defaults: UserDefaults) throws -> AppPreferences {
    guard let stored = defaults.object(forKey: appPreferencesKey) else {
        return AppPreferences(schema: appPreferencesSchema, views: [:])
    }
    guard let data = stored as? Data else {
        throw AppPreferencesError.wrongStoredType
    }
    let preferences: AppPreferences
    do {
        preferences = try JSONDecoder().decode(AppPreferences.self, from: data)
    } catch {
        throw AppPreferencesError.corrupt
    }
    guard preferences.schema == appPreferencesSchema else {
        throw AppPreferencesError.schema
    }
    guard preferences.views.allSatisfy({ !$0.key.isEmpty }) else {
        throw AppPreferencesError.invalidView
    }
    guard try preferenceEncoder().encode(preferences) == data else {
        throw AppPreferencesError.nonCanonical
    }
    return preferences
}

func loadViewOverride(subject: String, defaults: UserDefaults = .standard) throws -> ViewMode? {
    try loadPreferences(defaults: defaults).views[subject]
}

func saveViewOverride(subject: String, mode: ViewMode, defaults: UserDefaults = .standard) throws {
    guard !subject.isEmpty else {
        throw AppPreferencesError.invalidView
    }
    var preferences = try loadPreferences(defaults: defaults)
    preferences.views[subject] = mode
    try defaults.set(preferenceEncoder().encode(preferences), forKey: appPreferencesKey)
}
