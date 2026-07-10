import Foundation

/// Serializes access to the auth tokens and performs a single-flight refresh so that
/// several concurrent 401s trigger only ONE refresh call. Backed by a `TokenStore` for
/// persistence. The API client consults this actor to attach the bearer token and to
/// recover from a 401.
public actor TokenProvider {
    private let store: TokenStore
    private var cached: StoredTokens?
    /// The in-flight refresh, if any, so concurrent callers await the same result.
    private var inFlightRefresh: Task<StoredTokens, Error>?

    public init(store: TokenStore) {
        self.store = store
        self.cached = store.load()
    }

    /// Current access token, if signed in.
    public func accessToken() -> String? { cached?.accessToken }

    /// Current refresh token, if signed in.
    public func refreshToken() -> String? { cached?.refreshToken }

    public func isSignedIn() -> Bool { cached != nil }

    /// Persist a freshly minted token pair (after sign-in or refresh).
    public func set(_ pair: TokenPair, now: Date = Date()) {
        let tokens = StoredTokens(pair: pair, now: now)
        cached = tokens
        store.save(tokens)
    }

    public func clear() {
        cached = nil
        inFlightRefresh?.cancel()
        inFlightRefresh = nil
        store.clear()
    }

    /// Single-flight refresh. `perform` receives the current refresh token and returns a
    /// new token pair (the caller wires this to the `/v1/auth/refresh` endpoint). If no
    /// refresh token exists, throws `.unauthenticated`.
    public func refresh(perform: @Sendable @escaping (String) async throws -> TokenPair) async throws -> String {
        if let existing = inFlightRefresh {
            return try await existing.value.accessToken
        }
        guard let refreshToken = cached?.refreshToken else {
            throw APIError.unauthenticated
        }

        let task = Task { () throws -> StoredTokens in
            let pair = try await perform(refreshToken)
            let tokens = StoredTokens(pair: pair)
            return tokens
        }
        inFlightRefresh = task

        do {
            let tokens = try await task.value
            cached = tokens
            store.save(tokens)
            inFlightRefresh = nil
            return tokens.accessToken
        } catch {
            inFlightRefresh = nil
            // A failed refresh means the session is dead — force re-auth.
            if case APIError.server(let status, _, _, _) = error, status == 401 {
                clear()
            }
            throw error
        }
    }
}
