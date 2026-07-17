// Pairing: how the app remembers a daemon it scanned. A Machine (the identity and
// base URL) persists as JSON under Application Support; its bearer token lives in
// the Keychain as a generic-password item keyed by machine id, so the secret never
// touches the JSON file. PairPayload decodes the QR the daemon renders. The
// Keychain path uses only portable Security-framework calls, so it compiles — and
// `swift test` runs — on macOS as well as iOS, with no app entitlement required.

import Foundation
import Security

/// PairPayload is the QR-encoded handshake the daemon renders:
/// `{"v":1,"url":"http://host:port","token":"…"}`. Decoding rejects any version
/// but 1 up front, so an incompatible payload fails loudly at the boundary.
public struct PairPayload: Decodable, Equatable, Sendable {
    public let version: Int
    public let url: URL
    public let token: String

    public init(version: Int, url: URL, token: String) {
        self.version = version
        self.url = url
        self.token = token
    }

    private enum CodingKeys: String, CodingKey {
        case version = "v"
        case url
        case token
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let decoded = try container.decode(Int.self, forKey: .version)
        guard decoded == 1 else { throw PairError.unsupportedVersion(decoded) }
        version = decoded
        url = try container.decode(URL.self, forKey: .url)
        token = try container.decode(String.self, forKey: .token)
    }
}

/// PairError is a rejected pairing payload.
public enum PairError: Error, Equatable {
    case unsupportedVersion(Int)
}

/// Machine is a paired daemon the app remembers: a stable id (the Keychain account
/// for its token), a human name, and its base URL. The token is stored separately
/// in the Keychain, never in this record.
public struct Machine: Codable, Equatable, Sendable, Identifiable {
    public let id: String
    public var name: String
    public var baseURL: URL

    public init(id: String = UUID().uuidString, name: String, baseURL: URL) {
        self.id = id
        self.name = name
        self.baseURL = baseURL
    }
}

/// MachineStore persists the paired-machine roster as JSON in an injected
/// directory. Production points it at Application Support via `defaultDirectory`;
/// tests point it at a temp directory so they never touch the real store.
public struct MachineStore: Sendable {
    private let fileURL: URL

    /// Creates a store whose `machines.json` lives in `directory`.
    public init(directory: URL) {
        fileURL = directory.appendingPathComponent("machines.json")
    }

    /// defaultDirectory is `Application Support/com.yasyf.cc-present`, created if
    /// absent.
    public static func defaultDirectory() throws -> URL {
        let base = try FileManager.default.url(
            for: .applicationSupportDirectory,
            in: .userDomainMask,
            appropriateFor: nil,
            create: true
        )
        let directory = base.appendingPathComponent("com.yasyf.cc-present", isDirectory: true)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        return directory
    }

    /// load reads the roster, returning an empty list when nothing has been saved.
    public func load() throws -> [Machine] {
        guard FileManager.default.fileExists(atPath: fileURL.path) else { return [] }
        let data = try Data(contentsOf: fileURL)
        return try JSONDecoder().decode([Machine].self, from: data)
    }

    /// save writes the roster atomically, creating the directory if needed.
    public func save(_ machines: [Machine]) throws {
        try FileManager.default.createDirectory(
            at: fileURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        let data = try JSONEncoder().encode(machines)
        try data.write(to: fileURL, options: .atomic)
    }
}

/// KeychainError wraps a non-success Security-framework OSStatus.
public struct KeychainError: Error, Equatable {
    public let status: OSStatus
    public init(status: OSStatus) {
        self.status = status
    }
}

/// TokenStore keeps a machine's bearer token in the Keychain as a generic-password
/// item under service `com.yasyf.cc-present`, keyed by the machine id. The calls
/// are the portable SecItem surface, so they compile on macOS and iOS alike.
public enum TokenStore {
    /// service is the Keychain service every token item shares.
    public static let service = "com.yasyf.cc-present"

    /// setToken writes (replacing any prior value) the token for `machineID`, bound to
    /// this device and readable by a background read after the first post-boot unlock.
    public static func setToken(_ token: String, machineID: String) throws {
        SecItemDelete(baseQuery(machineID) as CFDictionary)
        var attributes = baseQuery(machineID)
        attributes[kSecValueData as String] = Data(token.utf8)
        attributes[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    /// token reads the token for `machineID`, or nil when none is stored. A token
    /// written before the device-only hardening is upgraded to it in place on read.
    public static func token(machineID: String) throws -> String? {
        var query = baseQuery(machineID)
        query[kSecReturnData as String] = true
        query[kSecReturnAttributes as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound {
            return nil
        }
        guard status == errSecSuccess,
              let item = result as? [String: Any],
              let data = item[kSecValueData as String] as? Data
        else {
            throw KeychainError(status: status)
        }
        if needsAccessibilityUpgrade(current: item[kSecAttrAccessible as String] as? String) {
            try upgradeAccessibility(machineID: machineID)
        }
        return String(decoding: data, as: UTF8.self)
    }

    /// deleteToken removes the token for `machineID`; a missing item is not an error.
    public static func deleteToken(machineID: String) throws {
        let status = SecItemDelete(baseQuery(machineID) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError(status: status)
        }
    }

    static func needsAccessibilityUpgrade(current: String?) -> Bool {
        current != (kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String)
    }

    private static func upgradeAccessibility(machineID: String) throws {
        let status = SecItemUpdate(
            baseQuery(machineID) as CFDictionary,
            [kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly] as CFDictionary
        )
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    private static func baseQuery(_ machineID: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: machineID,
        ]
    }
}
