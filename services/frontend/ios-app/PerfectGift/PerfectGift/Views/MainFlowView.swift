import SwiftUI
import PerfectGiftKit

/// The signed-in planner flow: occasion input → generating → ideas, plus poll creation.
/// A single `NavigationStack` with a typed path drives the submit-then-observe journey.
struct MainFlowView: View {
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var auth: AuthManager
    @EnvironmentObject private var router: AppRouter
    @State private var path: [Route] = []

    enum Route: Hashable {
        case generating(RequestGenerationRequest)
        case createPoll(surpriseRequestId: String?)
    }

    var body: some View {
        NavigationStack(path: $path) {
            OccasionInputView(viewModel: env.makeOccasionViewModel()) { request in
                path.append(.generating(request))
            }
            .navigationTitle("New surprise")
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Menu {
                        Button("Create a poll") { path.append(.createPoll(surpriseRequestId: nil)) }
                        Button("Sign out", role: .destructive) { Task { await auth.signOut() } }
                    } label: {
                        Image(systemName: "person.crop.circle")
                    }
                }
            }
            .navigationDestination(for: Route.self) { route in
                switch route {
                case let .generating(request):
                    GeneratingView(
                        viewModel: env.makeGenerationViewModel(),
                        request: request,
                        onCreatePoll: { requestId in path.append(.createPoll(surpriseRequestId: requestId)) }
                    )
                case let .createPoll(requestId):
                    PollCreateView(viewModel: env.makePollCreateViewModel(surpriseRequestId: requestId))
                }
            }
        }
    }
}
