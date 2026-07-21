import CcPresentKit
import Observation
import SwiftUI

/// MachineRegistry is the persistence seam the pairing flow writes through and the
/// roster reads from: the Machine record in the JSON store and its bearer token in
/// the Keychain, moved as one unit. KeychainMachineRegistry is the production
/// conformer; tests inject an in-memory fake so no filesystem or Keychain is touched.
protocol MachineRegistry: Sendable {
    func load() throws -> [Machine]
    func add(_ machine: Machine, token: String) throws
    func remove(_ machine: Machine) throws
    func token(for machineID: String) throws -> String?
}

/// KeychainMachineRegistry stores the roster in Application Support and each token
/// in the Keychain, keeping the secret out of the JSON record.
struct KeychainMachineRegistry: MachineRegistry {
    func load() throws -> [Machine] {
        try store().load()
    }

    func add(_ machine: Machine, token: String) throws {
        try TokenStore.setToken(token, machineID: machine.id)
        var machines = try store().load()
        machines.removeAll { $0.id == machine.id }
        machines.append(machine)
        try store().save(machines)
    }

    func remove(_ machine: Machine) throws {
        var machines = try store().load()
        machines.removeAll { $0.id == machine.id }
        try store().save(machines)
        try TokenStore.deleteToken(machineID: machine.id)
    }

    func token(for machineID: String) throws -> String? {
        try TokenStore.token(machineID: machineID)
    }

    private func store() throws -> MachineStore {
        try MachineStore(directory: MachineStore.defaultDirectory())
    }
}

/// SessionsClientFactory builds the probe client for a machine, given its bearer
/// token. Production returns an APIClient; tests return a fake.
typealias SessionsClientFactory = @Sendable (Machine, String?) -> any SessionsProviding

/// Reachability is the machine-row status dot: unknown until the first probe
/// resolves, then reachable, unreachable, or requiring a fresh pairing.
enum Reachability: Sendable {
    case unknown
    case reachable
    case unreachable
    case pairingRequired
}

/// MachinesModel owns the paired-machine roster and a lightweight reachability probe
/// per machine. The probe hits the same /api/sessions the board list uses, so a
/// green dot means the token is accepted and the daemon is up.
@MainActor
@Observable
final class MachinesModel {
    private(set) var machines: [Machine] = []
    private(set) var reachability: [String: Reachability] = [:]
    private(set) var loadError: String?

    private let registry: any MachineRegistry
    private let clientFactory: SessionsClientFactory

    init(
        registry: any MachineRegistry = KeychainMachineRegistry(),
        clientFactory: @escaping SessionsClientFactory = MachinesModel.defaultClientFactory
    ) {
        self.registry = registry
        self.clientFactory = clientFactory
    }

    static let defaultClientFactory: SessionsClientFactory = { machine, token in
        APIClient(baseURL: machine.baseURL, bearerToken: token)
    }

    /// load refreshes the roster from the registry.
    func load() {
        do {
            machines = try registry.load()
            loadError = nil
        } catch {
            loadError = "Couldn't load your machines."
        }
    }

    /// forget drops a machine from the roster and deletes its Keychain token.
    func forget(_ machine: Machine) {
        do {
            try registry.remove(machine)
        } catch {
            loadError = "Couldn't forget this machine."
            return
        }
        reachability.removeValue(forKey: machine.id)
        load()
    }

    /// probeAll probes every machine concurrently.
    func probeAll() async {
        await withTaskGroup(of: Void.self) { group in
            for machine in machines {
                group.addTask { await self.probe(machine) }
            }
        }
    }

    /// probe requires a v1 token before asking the machine for its sessions. A
    /// missing token requests manual re-pairing; transport failures are unreachable.
    func probe(_ machine: Machine) async {
        guard let token = try? registry.token(for: machine.id) else {
            reachability[machine.id] = .pairingRequired
            return
        }
        let client = clientFactory(machine, token)
        do {
            _ = try await client.sessions()
            reachability[machine.id] = .reachable
        } catch {
            reachability[machine.id] = .unreachable
        }
    }

    func reachability(of machine: Machine) -> Reachability {
        reachability[machine.id] ?? .unknown
    }
}

/// AddRoute is the add-machine entry the roster's menu opens as a sheet.
private enum AddRoute: String, Identifiable {
    case scan
    case browse
    case manual

    var id: String {
        rawValue
    }
}

/// MachinesView is the app root: the roster of paired machines with a reachability
/// dot, an add-machine menu, and swipe-to-forget.
struct MachinesView: View {
    @State private var model: MachinesModel
    @State private var pairing: PairingModel
    @State private var addRoute: AddRoute?

    init(
        registry: some MachineRegistry = KeychainMachineRegistry(),
        clientFactory: @escaping SessionsClientFactory = MachinesModel.defaultClientFactory
    ) {
        _model = State(initialValue: MachinesModel(registry: registry, clientFactory: clientFactory))
        _pairing = State(initialValue: PairingModel(registry: registry))
    }

    var body: some View {
        NavigationStack {
            roster
                .navigationTitle("Machines")
                .toolbar { addMenu }
                .task {
                    model.load()
                    await model.probeAll()
                }
                .refreshable { await model.probeAll() }
        }
        .sheet(item: $addRoute) { route in
            addSheet(route)
        }
    }

    @ViewBuilder
    private var roster: some View {
        if model.machines.isEmpty {
            ContentUnavailableView {
                Label("No Machines", systemImage: "desktopcomputer")
            } description: {
                Text("Pair a machine running cc-present to see its boards.")
            } actions: {
                Button("Add Machine") { addRoute = .scan }
            }
        } else {
            List {
                ForEach(model.machines) { machine in
                    NavigationLink {
                        SessionsView(machine: machine)
                    } label: {
                        MachineRow(machine: machine, reachability: model.reachability(of: machine))
                    }
                    .swipeActions(edge: .trailing) {
                        Button(role: .destructive) {
                            model.forget(machine)
                        } label: {
                            Label("Forget", systemImage: "trash")
                        }
                    }
                }
            }
        }
    }

    private var addMenu: some ToolbarContent {
        ToolbarItem(placement: .primaryAction) {
            Menu {
                Button { addRoute = .scan } label: { Label("Scan QR Code", systemImage: "qrcode.viewfinder") }
                Button { addRoute = .browse } label: { Label("Browse Network", systemImage: "network") }
                Button { addRoute = .manual } label: { Label("Enter Manually", systemImage: "keyboard") }
            } label: {
                Label("Add Machine", systemImage: "plus")
            }
        }
    }

    private func addSheet(_ route: AddRoute) -> some View {
        NavigationStack {
            Group {
                switch route {
                case .scan: PairScannerView(pairing: pairing)
                case .browse: BonjourBrowserView(pairing: pairing)
                case .manual: ManualPairView(pairing: pairing)
                }
            }
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismissAdd() }
                }
            }
        }
        .onChange(of: pairing.phase) { _, phase in
            if case .paired = phase {
                addRoute = nil
                pairing.reset()
                model.load()
                Task { await model.probeAll() }
            }
        }
    }

    private func dismissAdd() {
        addRoute = nil
        pairing.reset()
    }
}

private struct MachineRow: View {
    let machine: Machine
    let reachability: Reachability

    var body: some View {
        HStack(spacing: 12) {
            Circle()
                .fill(dotColor)
                .frame(width: 10, height: 10)
            VStack(alignment: .leading, spacing: 2) {
                Text(machine.name)
                    .font(.headline)
                Text(machine.baseURL.absoluteString)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if case .pairingRequired = reachability {
                    Text("Forget and pair this machine again.")
                        .font(.caption)
                        .foregroundStyle(.orange)
                }
            }
        }
        .padding(.vertical, 2)
    }

    private var dotColor: Color {
        switch reachability {
        case .unknown: .secondary
        case .reachable: .green
        case .unreachable: .red
        case .pairingRequired: .orange
        }
    }
}
