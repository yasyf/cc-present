import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI

/// CcPresentAppApp is the app entry point. Its single scene roots the navigation
/// at MachinesView, which owns the stack that drills machines → boards →
/// BoardScreen.
@main
struct CcPresentAppApp: App {
    var body: some Scene {
        WindowGroup {
            MachinesView(identity: .ccPresent) { machine in
                let client = HTTPClient(
                    baseURL: machine.baseURL,
                    bearerToken: try? KeychainMachineRegistry(identity: .ccPresent).token(for: machine.id)
                )
                SessionsView(machine: machine, client: client, noun: "board") { session in
                    BoardScreen(machine: machine, client: client, subject: session.subject)
                }
            }
        }
    }
}

extension AppIdentity {
    static let ccPresent = AppIdentity(
        name: "CcPresent",
        storageNamespace: "com.yasyf.cc-present",
        bonjourType: "_cc-present._tcp",
        pairScheme: "ccpresent"
    )
}
