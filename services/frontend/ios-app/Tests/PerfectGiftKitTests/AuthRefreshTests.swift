import Testing
import Foundation
@testable import PerfectGiftKit

/// Exercises `LiveAPIClient`'s transparent refresh-on-401 against a scripted transport.
@Suite struct AuthRefreshTests {

    private func makeClient(_ stubs: [FakeTransport.Stub], tokens: StoredTokens) -> (LiveAPIClient, FakeTransport, TokenProvider) {
        let transport = FakeTransport(stubs)
        let provider = TokenProvider(store: InMemoryTokenStore(tokens))
        let client = LiveAPIClient(baseURL: URL(string: "http://localhost:8080")!, transport: transport, tokens: provider)
        return (client, transport, provider)
    }

    @Test func refreshesOnceOn401ThenRetriesWithNewToken() async throws {
        let stubs: [FakeTransport.Stub] = [
            // 1: authed GET /v1/me → 401 (access token expired)
            .init(status: 401, body: json(#"{"error":{"code":"unauthorized","message":"expired"}}"#)) { req in
                #expect(req.value(forHTTPHeaderField: "Authorization") == "Bearer old_access")
            },
            // 2: POST /v1/auth/refresh → 200 with a new pair
            .init(status: 200, body: json(#"{"access_token":"new_access","refresh_token":"new_refresh","expires_in":3600,"user":{"id":"u1"}}"#)) { req in
                #expect(req.url?.path == "/v1/auth/refresh")
            },
            // 3: retry GET /v1/me → 200, must carry the NEW access token
            .init(status: 200, body: json(#"{"user":{"id":"u1","email":"a@b.co"}}"#)) { req in
                #expect(req.value(forHTTPHeaderField: "Authorization") == "Bearer new_access")
            }
        ]
        let (client, transport, provider) = makeClient(stubs, tokens: StoredTokens(accessToken: "old_access", refreshToken: "old_refresh"))

        let user = try await client.me()

        #expect(user.id == "u1")
        #expect(transport.requestCount() == 3)              // original + refresh + retry
        #expect(transport.path(at: 1) == "/v1/auth/refresh")
        let newAccess = await provider.accessToken()
        #expect(newAccess == "new_access")                  // refreshed token persisted
    }

    @Test func failedRefreshClearsSessionAndThrows() async {
        let stubs: [FakeTransport.Stub] = [
            .init(status: 401, body: json(#"{"error":{"code":"unauthorized","message":"expired"}}"#)),
            .init(status: 401, body: json(#"{"error":{"code":"unauthorized","message":"refresh expired"}}"#))
        ]
        let (client, _, provider) = makeClient(stubs, tokens: StoredTokens(accessToken: "old", refreshToken: "bad"))

        await #expect(throws: (any Error).self) {
            _ = try await client.me()
        }
        let signedIn = await provider.isSignedIn()
        #expect(!signedIn)   // a failed refresh clears the session
    }

    @Test func successfulCallDoesNotRefresh() async throws {
        let stubs: [FakeTransport.Stub] = [
            .init(status: 200, body: json(#"{"user":{"id":"u1"}}"#))
        ]
        let (client, transport, _) = makeClient(stubs, tokens: StoredTokens(accessToken: "good", refreshToken: "r"))
        _ = try await client.me()
        #expect(transport.requestCount() == 1)   // a 200 must not trigger a refresh
    }

    @Test func errorEnvelopeBecomesServerAPIError() async {
        let stubs: [FakeTransport.Stub] = [
            .init(status: 404, body: json(#"{"error":{"code":"not_found","message":"No such generation"}}"#))
        ]
        let (client, _, _) = makeClient(stubs, tokens: StoredTokens(accessToken: "good", refreshToken: "r"))
        do {
            _ = try await client.generation(id: "missing")
            Issue.record("expected error")
        } catch let error as APIError {
            guard case let .server(status, code, message, _) = error else {
                Issue.record("expected .server, got \(error)"); return
            }
            #expect(status == 404)
            #expect(code == "not_found")
            #expect(message == "No such generation")
        } catch {
            Issue.record("expected APIError, got \(error)")
        }
    }

    @Test func signInPersistsTokens() async throws {
        let stubs: [FakeTransport.Stub] = [
            .init(status: 200, body: json(#"{"access_token":"a","refresh_token":"r","expires_in":3600,"user":{"id":"u1"}}"#)) { req in
                #expect(req.url?.path == "/v1/auth/signin")
                #expect(req.value(forHTTPHeaderField: "Authorization") == nil)   // public route
            }
        ]
        let (client, _, provider) = makeClient(stubs, tokens: StoredTokens(accessToken: "", refreshToken: ""))
        await provider.clear()
        let pair = try await client.signIn(.email("a@b.co", password: "secret1"))
        #expect(pair.accessToken == "a")
        let stored = await provider.accessToken()
        #expect(stored == "a")
    }
}
