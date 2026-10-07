import CcPresentKit
import SwiftUI

/// blockViewIds is what one viewed mark covers: each block plus its card children,
/// since opening a card shows every block nested in it. Mirrors web/src/viewed.ts.
func blockViewIds(_ blocks: [Block]) -> [String] {
    flatten(blocks).map(\.id)
}

extension View {
    /// viewedOnScreen marks `block` viewed once its board row stays on screen for the
    /// board dwell; a changed block or round re-arms the wait.
    func viewedOnScreen(_ block: Block, round: Int, store: ViewedStore) -> some View {
        modifier(ViewedOnScreen(block: block, round: round, store: store))
    }

    /// viewedAfterFocus marks `ids` viewed once the focus step showing them stays up for
    /// the focus dwell.
    func viewedAfterFocus(_ ids: [String], store: ViewedStore) -> some View {
        modifier(ViewedAfterFocus(ids: ids, store: store))
    }
}

private struct ViewedOnScreen: ViewModifier {
    let block: Block
    let round: Int
    let store: ViewedStore

    @State private var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.boardDwell)
    @State private var onScreen = false
    @State private var deadline: ContinuousClock.Instant?

    func body(content: Content) -> some View {
        content
            .onGeometryChange(for: Bool.self) { proxy in
                guard let viewport = proxy.bounds(of: .scrollView) else { return false }
                return isViewedOnScreen(row: CGRect(origin: .zero, size: proxy.size), viewport: viewport)
            } action: { visible in
                onScreen = visible
                deadline = dwell.update(active: visible, at: .now)
            }
            .onChange(of: block) { rearm() }
            .onChange(of: round) { rearm() }
            .task(id: deadline) {
                guard let deadline else { return }
                do {
                    try await Task.sleep(until: deadline, clock: .continuous)
                } catch {
                    return
                }
                if dwell.complete(at: .now) {
                    store.mark(blockViewIds([block]))
                }
            }
    }

    private func rearm() {
        dwell = Dwell(duration: ViewedStore.boardDwell)
        deadline = dwell.update(active: onScreen, at: .now)
    }
}

private struct ViewedAfterFocus: ViewModifier {
    let ids: [String]
    let store: ViewedStore

    @State private var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.focusDwell)

    func body(content: Content) -> some View {
        content.task(id: ids) {
            dwell = Dwell(duration: ViewedStore.focusDwell)
            guard let deadline = dwell.update(active: true, at: .now) else { return }
            do {
                try await Task.sleep(until: deadline, clock: .continuous)
            } catch {
                return
            }
            if dwell.complete(at: .now) {
                store.mark(ids)
            }
        }
    }
}
