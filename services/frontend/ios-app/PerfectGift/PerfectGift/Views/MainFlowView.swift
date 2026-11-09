import SwiftUI
import PerfectGiftKit

/// The signed-in planner flow: occasion input → generating → ideas, plus profile & poll
/// creation. A single `NavigationStack` with a typed path drives the submit-then-observe
/// journey. System nav chrome is hidden — every screen draws its own Amore header.
struct MainFlowView: View {
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var auth: AuthManager
    @EnvironmentObject private var router: AppRouter
    @State private var path: [Route] = []

    enum Route: Hashable {
        case generating(RequestGenerationRequest)
        case createPoll(surpriseRequestId: String?)
        case profile
    }

    var body: some View {
        NavigationStack(path: $path) {
            OccasionInputView(
                viewModel: env.makeOccasionViewModel(),
                onSubmit: { request in path.append(.generating(request)) },
                onProfile: { path.append(.profile) }
            )
            .navigationBarBackButtonHidden(true)
            .toolbar(.hidden, for: .navigationBar)
            .navigationDestination(for: Route.self) { route in
                destination(for: route)
                    .navigationBarBackButtonHidden(true)
                    .toolbar(.hidden, for: .navigationBar)
            }
        }
    }

    @ViewBuilder
    private func destination(for route: Route) -> some View {
        switch route {
        case let .generating(request):
            GeneratingView(
                viewModel: env.makeGenerationViewModel(),
                request: request,
                onBack: { pop() },
                onCreatePoll: { requestId in path.append(.createPoll(surpriseRequestId: requestId)) }
            )
        case let .createPoll(requestId):
            PollCreateView(viewModel: env.makePollCreateViewModel(surpriseRequestId: requestId),
                           onBack: { pop() })
        case .profile:
            ProfileView(
                email: auth.currentUser?.email,
                displayName: auth.currentUser?.displayName,
                onGeneratePoll: { path.append(.createPoll(surpriseRequestId: nil)) },
                onSignOut: { Task { await auth.signOut() } },
                onBack: { pop() }
            )
        }
    }

    private func pop() { if !path.isEmpty { path.removeLast() } }
}
