import SwiftUI
import AuthenticationServices
import PerfectGiftKit

/// Sign-in screen: Sign in with Apple / Google (native SDKs supply an ID token) with an
/// email/password fallback. All paths funnel through `SignInViewModel` → `AuthManager`.
struct SignInView: View {
    @StateObject private var viewModel: SignInViewModel

    init(viewModel: SignInViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        VStack(spacing: 24) {
            Spacer()
            VStack(spacing: 8) {
                Image(systemName: "gift.fill")
                    .font(.system(size: 56))
                    .foregroundStyle(.tint)
                Text("PerfectGift")
                    .font(.largeTitle.bold())
                Text("Describe an occasion. Get ranked surprise ideas.")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
            }

            VStack(spacing: 12) {
                SignInWithAppleButton(.signIn) { request in
                    request.requestedScopes = [.email, .fullName]
                } onCompletion: { result in
                    handleApple(result)
                }
                .signInWithAppleButtonStyle(.black)
                .frame(height: 48)

                Button {
                    // The Google Sign-In SDK presents its own flow; on success it yields
                    // an ID token which we forward. Wired at integration time.
                    Task { await viewModel.signInWithGoogle(idToken: "") }
                } label: {
                    Label("Continue with Google", systemImage: "g.circle")
                        .frame(maxWidth: .infinity, minHeight: 48)
                }
                .buttonStyle(.bordered)
                .disabled(true) // enabled once the Google SDK is configured
            }

            emailFallback

            if let message = viewModel.errorMessage {
                Text(message)
                    .font(.footnote)
                    .foregroundStyle(.red)
                    .multilineTextAlignment(.center)
            }
            Spacer()
        }
        .padding(24)
        .overlay { if viewModel.isWorking { ProgressView() } }
    }

    private var emailFallback: some View {
        VStack(spacing: 12) {
            Divider().overlay(Text("or").font(.caption).foregroundStyle(.secondary).padding(.horizontal, 8))
            TextField("Email", text: $viewModel.email)
                .textContentType(.emailAddress)
                .keyboardType(.emailAddress)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .textFieldStyle(.roundedBorder)
            SecureField("Password", text: $viewModel.password)
                .textContentType(.password)
                .textFieldStyle(.roundedBorder)
            Button("Sign in with email") {
                Task { await viewModel.signInWithEmail() }
            }
            .buttonStyle(.borderedProminent)
            .frame(maxWidth: .infinity)
            .disabled(!viewModel.canSubmitEmail)
        }
    }

    private func handleApple(_ result: Result<ASAuthorization, Error>) {
        switch result {
        case let .success(auth):
            if let credential = auth.credential as? ASAuthorizationAppleIDCredential,
               let tokenData = credential.identityToken,
               let idToken = String(data: tokenData, encoding: .utf8) {
                Task { await viewModel.signInWithApple(idToken: idToken) }
            }
        case let .failure(error):
            viewModel.errorMessage = error.localizedDescription
        }
    }
}
