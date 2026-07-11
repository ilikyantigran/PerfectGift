import SwiftUI
import PerfectGiftKit

/// App entry point. Wires the composition root, the deep-link router, and the APNs
/// delegate, then hands off to `RootView`, which chooses sign-in vs. the main flow.
@main
struct PerfectGiftApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var env = AppEnvironment()
    @StateObject private var router = AppRouter()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(env)
                .environmentObject(env.auth)
                .environmentObject(router)
                .task {
                    appDelegate.configure(env: env, router: router)
                    await env.auth.bootstrap()
                }
                .onOpenURL { url in
                    router.handle(url: url, config: env.config)
                }
        }
    }
}
