import SwiftUI
import PerfectGiftKit

/// Chooses the top-level screen:
///  - a shared poll link (universal link) always opens the Subject flow, even signed out,
///  - otherwise signed-in users see the main flow and signed-out users see sign-in.
///
/// Also establishes the app-wide Amore look: forced dark scheme and rose tint.
struct RootView: View {
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var auth: AuthManager
    @EnvironmentObject private var router: AppRouter

    var body: some View {
        Group {
            if let token = router.pendingPollToken {
                // Handed-over phone / deep-linked Subject poll.
                NavigationStack {
                    SubjectPollView(viewModel: env.makeSubjectPollViewModel(token: token),
                                    onClose: { router.pendingPollToken = nil })
                }
            } else if auth.isSignedIn {
                MainFlowView()
            } else {
                SignInView(viewModel: env.makeSignInViewModel())
            }
        }
        .tint(Theme.rose)
        .preferredColorScheme(.dark)
        .animation(.easeInOut(duration: 0.3), value: auth.isSignedIn)
        .animation(.easeInOut(duration: 0.3), value: router.pendingPollToken)
    }
}
