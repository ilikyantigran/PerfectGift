import UIKit
import UserNotifications
import PerfectGiftKit

/// UIKit application delegate, bridged into SwiftUI, solely to own APNs registration and
/// push delivery — the two things SwiftUI's App lifecycle can't do alone.
final class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    private weak var env: AppEnvironment?
    private weak var router: AppRouter?

    @MainActor
    func configure(env: AppEnvironment, router: AppRouter) {
        self.env = env
        self.router = router
        UNUserNotificationCenter.current().delegate = self
        requestPushAuthorization()
    }

    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        true
    }

    private func requestPushAuthorization() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .badge, .sound]) { granted, _ in
            guard granted else { return }
            DispatchQueue.main.async {
                UIApplication.shared.registerForRemoteNotifications()
            }
        }
    }

    // MARK: APNs token

    func application(_ application: UIApplication,
                     didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String
        Task { @MainActor in
            await env?.push.register(deviceToken: deviceToken, appVersion: version)
        }
    }

    func application(_ application: UIApplication,
                     didFailToRegisterForRemoteNotificationsWithError error: Error) {
        // Best-effort per spec; nothing to do but log.
    }

    // MARK: Foreground presentation + taps

    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                willPresent notification: UNNotification) async -> UNNotificationPresentationOptions {
        await MainActor.run { router?.handlePush(userInfo: notification.request.content.userInfo) }
        return [.banner, .sound]
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                didReceive response: UNNotificationResponse) async {
        await MainActor.run { router?.handlePush(userInfo: response.notification.request.content.userInfo) }
    }
}
