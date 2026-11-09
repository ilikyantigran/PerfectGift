import SwiftUI
import AuthenticationServices
import PerfectGiftKit

/// Sign-in / sign-up screen (Amore "Registration" design). Sign in with Apple / Google with
/// an email/password fallback. All paths funnel through `SignInViewModel` → `AuthManager`.
/// The Sign Up / Sign In tabs are cosmetic — the backend upserts on the same email path.
struct SignInView: View {
    @StateObject private var viewModel: SignInViewModel
    @State private var isLogin = false

    init(viewModel: SignInViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        ZStack {
            ScreenBackground()
            ScrollView(showsIndicators: false) {
                VStack(spacing: 0) {
                    logo
                    form
                }
                .padding(.bottom, 32)
            }
            if viewModel.isWorking {
                Color.black.opacity(0.3).ignoresSafeArea()
                ProgressView().tint(Theme.cream)
            }
        }
    }

    // MARK: Logo

    private var logo: some View {
        VStack(spacing: 0) {
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .fill(LinearGradient(colors: [Theme.rose.opacity(0.25), Theme.peach.opacity(0.15)],
                                     startPoint: .topLeading, endPoint: .bottomTrailing))
                .frame(width: 72, height: 72)
                .overlay(RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .strokeBorder(Theme.rose.opacity(0.3), lineWidth: 1))
                .overlay(Image(systemName: "heart.fill").font(.system(size: 32))
                    .foregroundStyle(Theme.warmGradient))
                .shadow(color: Theme.rose.opacity(0.2), radius: 20, y: 12)
                .padding(.bottom, 16)

            Text("Amore").font(.display(32, weight: .bold)).foregroundStyle(Theme.cream)
            Text("Make love unforgettable").font(.amoreBody(13)).foregroundStyle(Theme.muted).padding(.top, 4)
        }
        .padding(.top, 48)
        .padding(.bottom, 32)
    }

    // MARK: Form

    private var form: some View {
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                tab("Sign Up", active: !isLogin) { isLogin = false }
                tab("Sign In", active: isLogin) { isLogin = true }
            }
            .padding(4)
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
            .padding(.bottom, 24)

            VStack(spacing: 12) {
                labeledField("Email") {
                    TextField("", text: $viewModel.email,
                              prompt: Text("your@email.com").foregroundColor(Theme.muted.opacity(0.6)))
                        .textContentType(.emailAddress)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                }
                labeledField("Password") {
                    SecureField("", text: $viewModel.password,
                                prompt: Text("••••••••").foregroundColor(Theme.muted.opacity(0.6)))
                        .textContentType(.password)
                }
            }
            .padding(.bottom, 16)

            if isLogin {
                HStack {
                    Spacer()
                    Button("Forgot password?") {}
                        .font(.amoreBody(12)).foregroundStyle(Theme.rose)
                }
                .padding(.bottom, 16)
            }

            PrimaryButton(title: isLogin ? "Sign In" : "Create Account",
                          enabled: viewModel.canSubmitEmail) {
                Task { await viewModel.signInWithEmail() }
            }
            .padding(.bottom, 24)

            if let message = viewModel.errorMessage {
                Text(message).font(.amoreBody(12)).foregroundStyle(Theme.rose)
                    .multilineTextAlignment(.center).padding(.bottom, 16)
            }

            // Divider
            HStack(spacing: 12) {
                Rectangle().fill(Theme.border).frame(height: 1)
                Text("or continue with").font(.amoreBody(12)).foregroundStyle(Theme.muted).fixedSize()
                Rectangle().fill(Theme.border).frame(height: 1)
            }
            .padding(.bottom, 20)

            VStack(spacing: 10) {
                SignInWithAppleButton(.signIn) { request in
                    request.requestedScopes = [.email, .fullName]
                } onCompletion: { handleApple($0) }
                    .signInWithAppleButtonStyle(.white)
                    .frame(height: 48)
                    .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))

                // Google Sign-In yields an ID token from its SDK (wired at integration time).
                socialOutline(title: "Continue with Google", symbol: "g.circle.fill", tint: Color(hex: 0x4285F4)) {
                    Task { await viewModel.signInWithGoogle(idToken: "") }
                }
                .disabled(true)
                .opacity(0.5)
            }
        }
        .padding(.horizontal, 28)
    }

    // MARK: Pieces

    private func tab(_ label: String, active: Bool, action: @escaping () -> Void) -> some View {
        Button(action: { withAnimation(.easeOut(duration: 0.2)) { action() } }) {
            Text(label)
                .font(.amoreBody(13, weight: .medium))
                .foregroundStyle(active ? Theme.cream : Theme.muted)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background {
                    if active {
                        RoundedRectangle(cornerRadius: 14, style: .continuous)
                            .fill(Theme.ctaGradient)
                            .shadow(color: Theme.rose.opacity(0.3), radius: 8, y: 4)
                    }
                }
        }
    }

    private func labeledField<Content: View>(_ label: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            FieldLabel(text: label)
            content()
                .font(.amoreBody(14))
                .foregroundStyle(Theme.cream)
                .tint(Theme.rose)
                .padding(.horizontal, 16).padding(.vertical, 14)
                .surfaceCard(cornerRadius: 16)
        }
    }

    private func socialOutline(title: String, symbol: String, tint: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: symbol).font(.system(size: 16)).foregroundStyle(tint)
                Text(title).font(.amoreBody(13)).foregroundStyle(Theme.cream)
            }
            .frame(maxWidth: .infinity).padding(.vertical, 13)
            .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
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
