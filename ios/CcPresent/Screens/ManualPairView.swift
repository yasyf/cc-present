import CcPresentKit
import Foundation
import Observation
import SwiftUI

/// PairingModel is the shared pairing state machine every add-machine path drives:
/// a scanned QR payload, a manually typed host/port/token, or a decoded PairPayload
/// all converge on `commit`, which persists the machine through the registry. The
/// phase advances idle → paired on success or idle → failed with a display reason,
/// and `reset` returns it to idle when a sheet is dismissed or a scan is retried.
@MainActor
@Observable
final class PairingModel {
    /// Phase is the outcome of the current pairing attempt.
    enum Phase: Equatable {
        case idle
        case paired(Machine)
        case failed(String)
    }

    private(set) var phase: Phase = .idle

    private let registry: any MachineRegistry

    init(registry: any MachineRegistry) {
        self.registry = registry
    }

    /// reset clears the phase back to idle.
    func reset() {
        phase = .idle
    }

    /// pair(scanned:) decodes a scanned QR string into a PairPayload and pairs it,
    /// mapping a wrong version or unparseable string to a failure reason.
    func pair(scanned string: String) {
        let trimmed = string.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let payload = try JSONDecoder().decode(PairPayload.self, from: Data(trimmed.utf8))
            pair(payload: payload)
        } catch let PairError.unsupportedVersion(version) {
            phase = .failed("This pairing code is version \(version); update the app to pair with it.")
        } catch {
            phase = .failed("That QR code isn't a CcPresent pairing code.")
        }
    }

    /// pair(payload:) pairs a decoded handshake, naming the machine from its URL.
    func pair(payload: PairPayload) {
        commit(Machine(name: Self.name(for: payload.url), baseURL: payload.url), token: payload.token)
    }

    /// pairManual validates typed fields and pairs them, failing with a per-field
    /// reason when a value is missing or malformed.
    func pairManual(host: String, port: String, token: String) {
        switch Self.parseManual(host: host, port: port, token: token) {
        case let .success(draft):
            commit(Machine(name: Self.name(for: draft.url), baseURL: draft.url), token: draft.token)
        case let .failure(error):
            phase = .failed(error.message)
        }
    }

    private func commit(_ machine: Machine, token: String) {
        do {
            try registry.add(machine, token: token)
            phase = .paired(machine)
        } catch {
            phase = .failed("Couldn't save this machine.")
        }
    }

    static func name(for url: URL) -> String {
        guard let host = url.host() else { return url.absoluteString }
        if let port = url.port {
            return "\(host):\(port)"
        }
        return host
    }

    /// PairDraft is a validated manual entry ready to persist.
    struct PairDraft: Equatable {
        let url: URL
        let token: String
    }

    /// PairFormError is a manual-entry validation failure carrying its display reason.
    struct PairFormError: Error, Equatable {
        let message: String
    }

    /// parseManual validates a typed host, port, and token into a PairDraft. It is a
    /// pure function so the pairing rules can be exercised directly in tests.
    nonisolated static func parseManual(host: String, port: String, token: String) -> Result<PairDraft, PairFormError> {
        let trimmedHost = host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedHost.isEmpty else {
            return .failure(PairFormError(message: "Enter the machine's host."))
        }
        let trimmedPort = port.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let portValue = Int(trimmedPort), (1 ... 65535).contains(portValue) else {
            return .failure(PairFormError(message: "Enter a port between 1 and 65535."))
        }
        let trimmedToken = token.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedToken.isEmpty else {
            return .failure(PairFormError(message: "Enter the pairing token."))
        }
        var components = URLComponents()
        components.scheme = "http"
        components.host = trimmedHost
        components.port = portValue
        guard let url = components.url, url.host() != nil else {
            return .failure(PairFormError(message: "That host and port don't form a valid address."))
        }
        return .success(PairDraft(url: url, token: trimmedToken))
    }
}

/// ManualPairView collects a host, port, and token by hand — the fallback when a QR
/// scan isn't possible and the token entry for a Bonjour-discovered machine.
struct ManualPairView: View {
    let pairing: PairingModel
    var prefillHost: String = ""
    var prefillPort: String = ""

    @State private var host = ""
    @State private var port = ""
    @State private var token = ""

    var body: some View {
        Form {
            Section("Machine") {
                TextField("Host", text: $host)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.URL)
                TextField("Port", text: $port)
                    .keyboardType(.numberPad)
            }
            Section("Pairing Token") {
                TextField("Token", text: $token)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
            }
            if case let .failed(message) = pairing.phase {
                Section {
                    Label(message, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                }
            }
            Section {
                Button("Pair") {
                    pairing.pairManual(host: host, port: port, token: token)
                }
                .disabled(!canSubmit)
            }
        }
        .navigationTitle("Enter Manually")
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            if host.isEmpty {
                host = prefillHost
            }
            if port.isEmpty {
                port = prefillPort
            }
        }
    }

    private var canSubmit: Bool {
        !host.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && !token.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}
