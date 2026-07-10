import SwiftUI
import PerfectGiftKit

/// The submit-then-observe screen. Kicks off the generation on appear, shows friendly
/// progress while queued/running, advances on a push, and swaps to the ideas list when
/// ready. On failure it offers a graceful "Try again" — never a dead end.
struct GeneratingView: View {
    @StateObject private var viewModel: GenerationViewModel
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var router: AppRouter
    let request: RequestGenerationRequest
    let onCreatePoll: (String?) -> Void

    init(viewModel: GenerationViewModel,
         request: RequestGenerationRequest,
         onCreatePoll: @escaping (String?) -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.request = request
        self.onCreatePoll = onCreatePoll
    }

    var body: some View {
        Group {
            switch viewModel.phase {
            case .idle, .submitting, .observing:
                progress
            case .ready:
                IdeasView(
                    viewModel: env.makeIdeasViewModel(ideas: viewModel.ideas),
                    onRefine: { text in Task { await viewModel.refine(text) } },
                    onCreatePoll: { onCreatePoll(viewModel.requestId) }
                )
            case let .failed(message):
                failure(message)
            }
        }
        .navigationTitle("Surprises")
        .task { await viewModel.submit(request) }
        .onDisappear { viewModel.cancelObserving() }
        // Advance immediately if a push says these ideas are ready.
        .onChange(of: router.readyGenerationRequestId) { _, newValue in
            if let id = newValue { Task { await viewModel.onPushReceived(requestId: id) } }
        }
    }

    private var progress: some View {
        VStack(spacing: 20) {
            ProgressView(value: Double(viewModel.progress), total: 100)
                .progressViewStyle(.linear)
                .padding(.horizontal, 40)
            Image(systemName: "sparkles")
                .font(.system(size: 44))
                .foregroundStyle(.tint)
                .symbolEffect(.pulse)
            Text(statusText)
                .font(.headline)
            Text("This usually takes a few seconds.")
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
        .padding()
    }

    private var statusText: String {
        switch viewModel.status {
        case .queued:  return "Warming up…"
        case .running: return "Dreaming up ideas…"
        default:       return "Working…"
        }
    }

    private func failure(_ message: String) -> some View {
        ContentUnavailableView {
            Label("Couldn't generate ideas", systemImage: "exclamationmark.triangle")
        } description: {
            Text(message)
        } actions: {
            Button("Try again") { Task { await viewModel.retry() } }
                .buttonStyle(.borderedProminent)
        }
    }
}
