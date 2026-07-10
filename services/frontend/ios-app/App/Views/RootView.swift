import SwiftUI
import PerfectGiftKit

/// Chooses the top-level screen:
///  - a shared poll link (universal link) always opens the Subject flow, even signed out,
///  - otherwise signed-in users see the main flow and signed-out users see sign-in.
struct RootView: View {
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var auth: AuthManager
    @EnvironmentObject private var router: AppRouter

    var body: some View {
        Group {
            if let token = router.pendingPollToken {
                // Handed-over phone / deep-linked Subject poll.
                NavigationStack {
                    SubjectPollView(viewModel: env.makeSubjectPollViewModel(token: token))
                        .toolbar {
                            ToolbarItem(placement: .cancellationAction) {
                                Button("Close") { router.pendingPollToken = nil }
                            }
                        }
                }
            } else if auth.isSignedIn {
                MainFlowView()
            } else {
                SignInView(viewModel: env.makeSignInViewModel())
            }
        }
        .animation(.default, value: auth.isSignedIn)
        .animation(.default, value: router.pendingPollToken)
    }
}
