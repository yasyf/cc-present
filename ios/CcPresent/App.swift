import SwiftUI

/// CcPresentAppApp is the app entry point. Its single scene roots the navigation
/// at MachinesView, which owns the stack that drills MachinesView → SessionsView →
/// BoardScreen.
@main
struct CcPresentAppApp: App {
    var body: some Scene {
        WindowGroup {
            MachinesView()
        }
    }
}
