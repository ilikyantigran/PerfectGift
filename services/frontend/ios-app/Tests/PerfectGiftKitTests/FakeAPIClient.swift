import Foundation
@testable import PerfectGiftKit

/// A programmable in-memory `APIClient` for unit tests. Each endpoint reads from a
/// configurable stub (a value, an error, or — for generation polling — a queue of
/// results) and records the calls made so tests can assert on them. No network.
final class FakeAPIClient: APIClient, @unchecked Sendable {
    private let lock = NSLock()

    // Stubbed outcomes
    var signInResult: Result<TokenPair, Error> = .failure(APIError.unauthenticated)
    var refreshResult: Result<TokenPair, Error> = .failure(APIError.unauthenticated)
    var meResult: Result<User, Error> = .success(User(id: "u1", email: "a@b.co"))
    var generationAccepted: Result<GenerationAccepted, Error> = .success(GenerationAccepted(requestId: "req_1", status: .queued))
    /// Queue of statuses returned by successive `generation(id:)` polls. Last one repeats.
    var generationQueue: [GenerationResult] = []
    var refineAccepted: Result<GenerationAccepted, Error> = .success(GenerationAccepted(requestId: "req_2", status: .queued))
    var saveIdeaError: Error?
    var createPollResult: Result<CreatePollResponse, Error> = .success(CreatePollResponse(pollId: "p1", linkToken: "tok", linkUrl: "https://perfectgift.app/p/tok"))
    var pollResponsesResult: Result<[PollResponse], Error> = .success([])
    var pollByTokenResult: Result<PollByToken, Error> = .success(PollByToken(pollId: "p1", title: "T", questions: []))
    var submitResponseError: Error?
    var holidaysResult: Result<[Holiday], Error> = .success([])
    var categoriesResult: Result<CategoriesResponse, Error> = .success(CategoriesResponse())
    var registerDeviceResult: Result<RegisterDeviceResponse, Error> = .success(RegisterDeviceResponse(deviceId: "d1"))

    // Call recording
    private(set) var signInCalls: [SignInRequest] = []
    private(set) var generationSubmits: [(RequestGenerationRequest, String?)] = []
    private(set) var generationPolls: [String] = []
    private(set) var savedIdeaIds: [String] = []
    private(set) var submittedAnswers: [[Answer]] = []
    private(set) var revokeCalls = 0

    private func sync<T>(_ body: () -> T) -> T { lock.lock(); defer { lock.unlock() }; return body() }

    func signIn(_ request: SignInRequest) async throws -> TokenPair {
        sync { signInCalls.append(request) }
        return try signInResult.get()
    }

    func refresh(refreshToken: String) async throws -> TokenPair {
        try refreshResult.get()
    }

    func revoke(_ request: RevokeRequest) async throws {
        sync { revokeCalls += 1 }
    }

    func me() async throws -> User { try meResult.get() }

    func requestGeneration(_ request: RequestGenerationRequest, idempotencyKey: String?) async throws -> GenerationAccepted {
        sync { generationSubmits.append((request, idempotencyKey)) }
        return try generationAccepted.get()
    }

    func generation(id: String) async throws -> GenerationResult {
        let index = sync { () -> Int in
            generationPolls.append(id)
            return generationPolls.count - 1
        }
        guard !generationQueue.isEmpty else {
            return GenerationResult(requestId: id, status: .running)
        }
        let i = min(index, generationQueue.count - 1)
        return generationQueue[i]
    }

    func refine(id: String, refinement: String) async throws -> GenerationAccepted {
        try refineAccepted.get()
    }

    func saveIdea(id: String) async throws {
        if let e = saveIdeaError { throw e }
        sync { savedIdeaIds.append(id) }
    }

    func createPoll(_ request: CreatePollRequest) async throws -> CreatePollResponse {
        try createPollResult.get()
    }

    func pollResponses(pollId: String) async throws -> [PollResponse] {
        try pollResponsesResult.get()
    }

    func pollByToken(_ token: String) async throws -> PollByToken {
        try pollByTokenResult.get()
    }

    func submitResponse(token: String, answers: [Answer]) async throws {
        if let e = submitResponseError { throw e }
        sync { submittedAnswers.append(answers) }
    }

    func holidays(region: String?, active: Bool?, onOrAfter: String?) async throws -> [Holiday] {
        try holidaysResult.get()
    }

    func categories(kind: String?) async throws -> CategoriesResponse {
        try categoriesResult.get()
    }

    func registerDevice(_ request: RegisterDeviceRequest) async throws -> RegisterDeviceResponse {
        try registerDeviceResult.get()
    }

    // Test helpers
    func generationPollCount() -> Int { sync { generationPolls.count } }
    func generationSubmitCount() -> Int { sync { generationSubmits.count } }
    func lastIdempotencyKey() -> String? { sync { generationSubmits.last?.1 } }
}

/// An instantly-returning sleeper so the generation poll loop runs without real delay.
let instantSleeper: @Sendable (TimeInterval) async throws -> Void = { _ in
    await Task.yield()
}
