import Foundation
import Combine

/// The heart of the app: the **submit-then-observe** flow. Submits a generation,
/// then advances by polling `GET /v1/generations/{id}` on an interval OR immediately
/// when a push arrives, until the status is terminal (`ready`/`failed`).
@MainActor
public final class GenerationViewModel: ObservableObject {

    public enum Phase: Equatable {
        case idle
        case submitting
        case observing          // queued / running
        case ready
        case failed(String)
    }

    @Published public private(set) var phase: Phase = .idle
    @Published public private(set) var status: GenerationStatus = .unknown
    @Published public private(set) var progress: Int = 0
    @Published public private(set) var ideas: [Idea] = []
    @Published public private(set) var requestId: String?

    private let api: APIClient
    /// Poll cadence in seconds and an injectable sleeper make the loop fast & deterministic in tests.
    private let pollInterval: TimeInterval
    private let sleeper: @Sendable (TimeInterval) async throws -> Void
    /// Safety cap so a stuck generation never polls forever.
    private let maxPolls: Int

    private var lastRequest: RequestGenerationRequest?
    private var idempotencyKey: String?
    private var observeTask: Task<Void, Never>?

    public init(
        api: APIClient,
        pollInterval: TimeInterval = 2.0,
        maxPolls: Int = 60,
        sleeper: @escaping @Sendable (TimeInterval) async throws -> Void = { seconds in
            try await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
        }
    ) {
        self.api = api
        self.pollInterval = pollInterval
        self.maxPolls = maxPolls
        self.sleeper = sleeper
    }

    // MARK: - Submit

    /// Submit a new generation and begin observing. Reuses one Idempotency-Key so an
    /// automatic retry of the same submit is de-duplicated by the backend.
    public func submit(_ request: RequestGenerationRequest) async {
        lastRequest = request
        idempotencyKey = UUID().uuidString
        await submitCurrent()
    }

    /// Retry after a failure — resubmits the last request with a fresh idempotency key.
    public func retry() async {
        guard lastRequest != nil else { return }
        idempotencyKey = UUID().uuidString
        await submitCurrent()
    }

    private func submitCurrent() async {
        guard let request = lastRequest else { return }
        cancelObserving()
        phase = .submitting
        status = .queued
        progress = 0
        ideas = []
        do {
            let accepted = try await api.requestGeneration(request, idempotencyKey: idempotencyKey)
            requestId = accepted.requestId
            status = accepted.status ?? .queued
            phase = .observing
            startObserving()
        } catch {
            fail(with: error)
        }
    }

    // MARK: - Observe

    /// Begin the polling loop. Idempotent — a running loop is not duplicated.
    public func startObserving() {
        guard observeTask == nil, let id = requestId else { return }
        observeTask = Task { [weak self] in
            guard let self else { return }
            var polls = 0
            while !Task.isCancelled {
                let done = await self.pollOnce(id: id)
                if done { break }
                polls += 1
                if polls >= self.maxPolls {
                    self.timeOut()
                    break
                }
                try? await self.sleeper(self.pollInterval)
            }
            await MainActor.run { self.observeTask = nil }
        }
    }

    /// Poll status once. Returns true when the generation reached a terminal state.
    @discardableResult
    public func pollOnce() async -> Bool {
        guard let id = requestId else { return true }
        return await pollOnce(id: id)
    }

    private func pollOnce(id: String) async -> Bool {
        do {
            let result = try await api.generation(id: id)
            status = result.status
            if let p = result.progress { progress = p }
            switch result.status {
            case .ready:
                ideas = result.rankedIdeas
                progress = 100
                phase = .ready
                return true
            case .failed:
                phase = .failed("Generation failed. Please try again.")
                return true
            case .queued, .running, .unknown:
                phase = .observing
                return false
            }
        } catch {
            // A transient poll error shouldn't kill the flow; surface only if persistent.
            if case APIError.unauthenticated = error {
                fail(with: error)
                return true
            }
            return false
        }
    }

    /// Called from the AppDelegate when a "ideas ready" push arrives for this request.
    public func onPushReceived(requestId pushId: String) async {
        guard pushId == requestId else { return }
        _ = await pollOnce()
    }

    // MARK: - Refine

    public func refine(_ refinement: String) async {
        guard let id = requestId else { return }
        cancelObserving()
        phase = .submitting
        ideas = []
        do {
            let accepted = try await api.refine(id: id, refinement: refinement)
            requestId = accepted.requestId
            status = accepted.status ?? .queued
            phase = .observing
            startObserving()
        } catch {
            fail(with: error)
        }
    }

    // MARK: - Lifecycle

    public func cancelObserving() {
        observeTask?.cancel()
        observeTask = nil
    }

    private func timeOut() {
        phase = .failed("This is taking longer than expected. Please try again.")
    }

    private func fail(with error: Error) {
        let message = (error as? APIError)?.userMessage ?? error.localizedDescription
        phase = .failed(message)
    }
}
