import Foundation

/// Concrete `APIClient` over the gateway REST API. Handles:
///  - `Authorization: Bearer` injection on authenticated routes,
///  - transparent refresh on `401` (single-flight) with exactly one retry,
///  - decoding of the `{ error: { code, message, details } }` envelope into `APIError`.
public final class LiveAPIClient: APIClient {
    private let baseURL: URL
    private let transport: HTTPTransport
    private let tokens: TokenProvider
    private let encoder = JSONCoding.encoder
    private let decoder = JSONCoding.decoder

    public init(baseURL: URL, transport: HTTPTransport = URLSessionTransport(), tokens: TokenProvider) {
        self.baseURL = baseURL
        self.transport = transport
        self.tokens = tokens
    }

    // MARK: - Request description

    private enum Auth { case none, bearer }

    private struct Request {
        var method: String
        var path: String
        var query: [URLQueryItem] = []
        var body: Data?
        var auth: Auth = .bearer
        var headers: [String: String] = [:]
    }

    // MARK: - Core send with refresh-on-401

    private func send<T: Decodable>(_ req: Request, as type: T.Type) async throws -> T {
        let data = try await sendRaw(req)
        return try decode(type, from: data)
    }

    /// Sends a request, refreshing once on 401 for authenticated routes.
    private func sendRaw(_ req: Request, isRetry: Bool = false) async throws -> Data {
        var urlRequest = try buildURLRequest(req)

        if req.auth == .bearer {
            guard let token = await tokens.accessToken() else {
                throw APIError.unauthenticated
            }
            urlRequest.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let (data, response) = try await transport.send(urlRequest)

        if response.statusCode == 401, req.auth == .bearer, !isRetry {
            // Attempt a single transparent refresh, then retry the original request once.
            _ = try await tokens.refresh { [weak self] refreshToken in
                guard let self else { throw APIError.unauthenticated }
                return try await self.performRefresh(refreshToken: refreshToken)
            }
            return try await sendRaw(req, isRetry: true)
        }

        guard (200..<300).contains(response.statusCode) else {
            throw makeError(status: response.statusCode, data: data)
        }
        return data
    }

    /// Refresh call performed OUTSIDE the authed path (public route, uses refresh token).
    private func performRefresh(refreshToken: String) async throws -> TokenPair {
        let body = try encoder.encode(RefreshRequest(refreshToken: refreshToken))
        let req = Request(method: "POST", path: "/v1/auth/refresh", body: body, auth: .none)
        let urlRequest = try buildURLRequest(req)
        let (data, response) = try await transport.send(urlRequest)
        guard (200..<300).contains(response.statusCode) else {
            throw makeError(status: response.statusCode, data: data)
        }
        return try decode(TokenPair.self, from: data)
    }

    // MARK: - Building & decoding

    private func buildURLRequest(_ req: Request) throws -> URLRequest {
        guard var components = URLComponents(url: baseURL.appendingPathComponent(req.path), resolvingAgainstBaseURL: false) else {
            throw APIError.invalidRequest("Bad URL for \(req.path)")
        }
        if !req.query.isEmpty { components.queryItems = req.query }
        guard let url = components.url else {
            throw APIError.invalidRequest("Bad URL components for \(req.path)")
        }
        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = req.method
        urlRequest.setValue("application/json", forHTTPHeaderField: "Accept")
        if req.body != nil {
            urlRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
            urlRequest.httpBody = req.body
        }
        for (k, v) in req.headers { urlRequest.setValue(v, forHTTPHeaderField: k) }
        return urlRequest
    }

    private func decode<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        // 204 / empty body for Void-like responses.
        if data.isEmpty, let empty = OKResponse(ok: true) as? T { return empty }
        do {
            return try decoder.decode(type, from: data)
        } catch {
            throw APIError.decoding("Failed to decode \(T.self): \(error)")
        }
    }

    private func makeError(status: Int, data: Data) -> APIError {
        if let envelope = try? decoder.decode(ErrorEnvelope.self, from: data) {
            return .server(status: status, code: envelope.error.code, message: envelope.error.message, details: envelope.error.details)
        }
        let body = String(data: data, encoding: .utf8) ?? ""
        return .http(status: status, body: body)
    }

    // MARK: - APIClient conformance

    public func signIn(_ request: SignInRequest) async throws -> TokenPair {
        let body = try encoder.encode(request)
        let pair: TokenPair = try await send(Request(method: "POST", path: "/v1/auth/signin", body: body, auth: .none), as: TokenPair.self)
        await tokens.set(pair)
        return pair
    }

    public func refresh(refreshToken: String) async throws -> TokenPair {
        let pair = try await performRefresh(refreshToken: refreshToken)
        await tokens.set(pair)
        return pair
    }

    public func revoke(_ request: RevokeRequest) async throws {
        let body = try encoder.encode(request)
        _ = try await send(Request(method: "POST", path: "/v1/auth/revoke", body: body), as: OKResponse.self)
        await tokens.clear()
    }

    public func me() async throws -> User {
        let resp: MeResponse = try await send(Request(method: "GET", path: "/v1/me"), as: MeResponse.self)
        return resp.user
    }

    public func requestGeneration(_ request: RequestGenerationRequest, idempotencyKey: String?) async throws -> GenerationAccepted {
        let body = try encoder.encode(request)
        var headers: [String: String] = [:]
        if let key = idempotencyKey { headers["Idempotency-Key"] = key }
        return try await send(Request(method: "POST", path: "/v1/generations", body: body, headers: headers), as: GenerationAccepted.self)
    }

    public func generation(id: String) async throws -> GenerationResult {
        try await send(Request(method: "GET", path: "/v1/generations/\(id)"), as: GenerationResult.self)
    }

    public func refine(id: String, refinement: String) async throws -> GenerationAccepted {
        let body = try encoder.encode(RefineRequest(refinement: refinement))
        return try await send(Request(method: "POST", path: "/v1/generations/\(id)/refine", body: body), as: GenerationAccepted.self)
    }

    public func saveIdea(id: String) async throws {
        _ = try await send(Request(method: "POST", path: "/v1/ideas/\(id)/save"), as: OKResponse.self)
    }

    public func createPoll(_ request: CreatePollRequest) async throws -> CreatePollResponse {
        let body = try encoder.encode(request)
        return try await send(Request(method: "POST", path: "/v1/polls", body: body), as: CreatePollResponse.self)
    }

    public func pollResponses(pollId: String) async throws -> [PollResponse] {
        let resp: PollResponsesResponse = try await send(Request(method: "GET", path: "/v1/polls/\(pollId)/responses"), as: PollResponsesResponse.self)
        return resp.responses
    }

    public func pollByToken(_ token: String) async throws -> PollByToken {
        try await send(Request(method: "GET", path: "/v1/polls/token/\(token)", auth: .none), as: PollByToken.self)
    }

    public func submitResponse(token: String, answers: [Answer]) async throws {
        let body = try encoder.encode(SubmitResponseRequest(answers: answers))
        _ = try await send(Request(method: "POST", path: "/v1/polls/token/\(token)/responses", body: body, auth: .none), as: OKResponse.self)
    }

    public func holidays(region: String?, active: Bool?, onOrAfter: String?) async throws -> [Holiday] {
        var query: [URLQueryItem] = []
        if let region { query.append(.init(name: "region", value: region)) }
        if let active { query.append(.init(name: "active", value: active ? "true" : "false")) }
        if let onOrAfter { query.append(.init(name: "on_or_after", value: onOrAfter)) }
        let resp: HolidaysResponse = try await send(Request(method: "GET", path: "/v1/holidays", query: query), as: HolidaysResponse.self)
        return resp.holidays
    }

    public func categories(kind: String?) async throws -> CategoriesResponse {
        var query: [URLQueryItem] = []
        if let kind { query.append(.init(name: "kind", value: kind)) }
        return try await send(Request(method: "GET", path: "/v1/categories", query: query), as: CategoriesResponse.self)
    }

    public func registerDevice(_ request: RegisterDeviceRequest) async throws -> RegisterDeviceResponse {
        let body = try encoder.encode(request)
        return try await send(Request(method: "POST", path: "/v1/devices", body: body), as: RegisterDeviceResponse.self)
    }
}
