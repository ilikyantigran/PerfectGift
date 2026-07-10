import Foundation
import Combine

/// Drives the sign-in screen. Apple/Google provide an ID token from the native SDK
/// (the View obtains it); email is the fallback. All paths funnel through `AuthManager`.
@MainActor
public final class SignInViewModel: ObservableObject {
    @Published public var email: String = ""
    @Published public var password: String = ""
    @Published public private(set) var isWorking = false
    @Published public var errorMessage: String?

    private let auth: AuthManager

    public init(auth: AuthManager) {
        self.auth = auth
    }

    public var canSubmitEmail: Bool {
        email.contains("@") && password.count >= 6 && !isWorking
    }

    public func signInWithEmail() async {
        guard canSubmitEmail else { return }
        await run(.email(email, password: password))
    }

    /// Called by the View after Sign in with Apple yields an identity token.
    public func signInWithApple(idToken: String) async {
        await run(.apple(idToken: idToken))
    }

    /// Called by the View after Google Sign-In yields an ID token.
    public func signInWithGoogle(idToken: String) async {
        await run(.google(idToken: idToken))
    }

    private func run(_ request: SignInRequest) async {
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        await auth.signIn(request)
        errorMessage = auth.errorMessage
    }
}
