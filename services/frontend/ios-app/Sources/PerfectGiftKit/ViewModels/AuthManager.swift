import Foundation
import Combine

/// App-wide session state. Owns sign-in/out and exposes whether the user is signed in.
/// Observed by the root view to switch between the sign-in screen and the main flow.
@MainActor
public final class AuthManager: ObservableObject {
    @Published public private(set) var currentUser: User?
    @Published public private(set) var isSignedIn: Bool
    @Published public private(set) var isWorking: Bool = false
    @Published public var errorMessage: String?

    private let api: APIClient
    private let tokens: TokenProvider

    public init(api: APIClient, tokens: TokenProvider, initiallySignedIn: Bool = false) {
        self.api = api
        self.tokens = tokens
        self.isSignedIn = initiallySignedIn
    }

    /// Determine session state from persisted tokens at launch (and load the profile).
    public func bootstrap() async {
        let signedIn = await tokens.isSignedIn()
        isSignedIn = signedIn
        guard signedIn else { return }
        await loadProfile()
    }

    public func signIn(_ request: SignInRequest) async {
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        do {
            let pair = try await api.signIn(request)
            currentUser = pair.user
            isSignedIn = true
            if currentUser == nil { await loadProfile() }
        } catch {
            isSignedIn = false
            errorMessage = (error as? APIError)?.userMessage ?? error.localizedDescription
        }
    }

    public func loadProfile() async {
        do {
            currentUser = try await api.me()
        } catch {
            // A hard 401 here means the refresh also failed → sign the user out.
            if case APIError.unauthenticated = error {
                await forceSignOut()
            }
        }
    }

    public func signOut() async {
        isWorking = true
        defer { isWorking = false }
        let refreshToken = await tokens.refreshToken()
        do {
            try await api.revoke(RevokeRequest(refreshToken: refreshToken))
        } catch {
            // Even if revoke fails server-side, clear locally.
        }
        await forceSignOut()
    }

    private func forceSignOut() async {
        await tokens.clear()
        currentUser = nil
        isSignedIn = false
    }
}
