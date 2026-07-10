import Foundation
import SwiftUI
import PerfectGiftKit

/// Cross-cutting navigation intents raised by universal links and push notifications.
/// Views observe this to present the right screen (e.g. a shared poll token deep-links
/// straight into the Subject flow; an "ideas ready" push surfaces the request id).
@MainActor
final class AppRouter: ObservableObject {
    /// A link token to open in the Subject poll flow (from a universal link).
    @Published var pendingPollToken: String?
    /// A generation request id whose ideas just became ready (from a push).
    @Published var readyGenerationRequestId: String?

    /// Parse an incoming universal link. Supports paths like `/p/{token}` or
    /// `/polls/token/{token}`. Returns true if it was handled.
    @discardableResult
    func handle(url: URL, config: AppConfig) -> Bool {
        guard url.host == config.universalLinkHost || url.host == nil else { return false }
        let parts = url.pathComponents.filter { $0 != "/" }
        if let idx = parts.firstIndex(of: "p"), idx + 1 < parts.count {
            pendingPollToken = parts[idx + 1]
            return true
        }
        if let idx = parts.firstIndex(of: "token"), idx + 1 < parts.count {
            pendingPollToken = parts[idx + 1]
            return true
        }
        return false
    }

    /// Parse an APNs payload for a "ideas ready" signal carrying the request id.
    func handlePush(userInfo: [AnyHashable: Any]) {
        if let requestId = userInfo["request_id"] as? String {
            readyGenerationRequestId = requestId
        }
    }
}
