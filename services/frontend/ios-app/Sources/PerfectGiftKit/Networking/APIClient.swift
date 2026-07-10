import Foundation

/// The full set of gateway calls the app makes. View models depend on this protocol
/// (never the concrete client), so they can be unit-tested with a fake.
public protocol APIClient: Sendable {
    // MARK: Auth
    func signIn(_ request: SignInRequest) async throws -> TokenPair
    func refresh(refreshToken: String) async throws -> TokenPair
    func revoke(_ request: RevokeRequest) async throws
    func me() async throws -> User

    // MARK: Generations (submit-then-observe)
    func requestGeneration(_ request: RequestGenerationRequest, idempotencyKey: String?) async throws -> GenerationAccepted
    func generation(id: String) async throws -> GenerationResult
    func refine(id: String, refinement: String) async throws -> GenerationAccepted
    func saveIdea(id: String) async throws

    // MARK: Polls (owner)
    func createPoll(_ request: CreatePollRequest) async throws -> CreatePollResponse
    func pollResponses(pollId: String) async throws -> [PollResponse]

    // MARK: Polls (anonymous Subject, by link token)
    func pollByToken(_ token: String) async throws -> PollByToken
    func submitResponse(token: String, answers: [Answer]) async throws

    // MARK: Reference data
    func holidays(region: String?, active: Bool?, onOrAfter: String?) async throws -> [Holiday]
    func categories(kind: String?) async throws -> CategoriesResponse

    // MARK: Push
    func registerDevice(_ request: RegisterDeviceRequest) async throws -> RegisterDeviceResponse
}

// Convenience overloads with sensible defaults.
public extension APIClient {
    func requestGeneration(_ request: RequestGenerationRequest) async throws -> GenerationAccepted {
        try await requestGeneration(request, idempotencyKey: UUID().uuidString)
    }
    func holidays() async throws -> [Holiday] {
        try await holidays(region: nil, active: true, onOrAfter: nil)
    }
    func categories() async throws -> CategoriesResponse {
        try await categories(kind: nil)
    }
}
