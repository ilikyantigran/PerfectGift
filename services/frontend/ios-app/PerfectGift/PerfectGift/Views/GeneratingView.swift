import SwiftUI
import PerfectGiftKit

/// The submit-then-observe screen (Amore "Loader" design): an animated pulsing heart with a
/// spinning ring and floating particles while queued/running, advancing on a push, then
/// swapping to the ideas list when ready. On failure it offers a graceful "Try again".
struct GeneratingView: View {
    @StateObject private var viewModel: GenerationViewModel
    @EnvironmentObject private var env: AppEnvironment
    @EnvironmentObject private var router: AppRouter
    let request: RequestGenerationRequest
    let onBack: () -> Void
    let onCreatePoll: (String?) -> Void

    init(viewModel: GenerationViewModel,
         request: RequestGenerationRequest,
         onBack: @escaping () -> Void,
         onCreatePoll: @escaping (String?) -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.request = request
        self.onBack = onBack
        self.onCreatePoll = onCreatePoll
    }

    var body: some View {
        ZStack {
            ScreenBackground()
            switch viewModel.phase {
            case .idle, .submitting, .observing:
                loader
            case .ready:
                IdeasView(
                    viewModel: env.makeIdeasViewModel(ideas: viewModel.ideas),
                    onBack: onBack,
                    onRefine: { text in Task { await viewModel.refine(text) } },
                    onCreatePoll: { onCreatePoll(viewModel.requestId) }
                )
            case let .failed(message):
                failure(message)
            }
        }
        .task { await viewModel.submit(request) }
        .onDisappear { viewModel.cancelObserving() }
        .onChange(of: router.readyGenerationRequestId) { _, newValue in
            if let id = newValue { Task { await viewModel.onPushReceived(requestId: id) } }
        }
    }

    // MARK: Loader

    private var loader: some View {
        VStack(spacing: 0) {
            HeartLoader()
                .frame(width: 120, height: 120)
                .padding(.bottom, 40)

            (Text("Crafting your\n").foregroundColor(Theme.cream)
             + Text("perfect surprise").foregroundColor(Theme.rose).italic())
                .font(.display(26, weight: .bold))
                .multilineTextAlignment(.center)
                .lineSpacing(3)
                .padding(.bottom, 8)

            Text(subtitle).font(.amoreBody(13)).foregroundStyle(Theme.muted).padding(.bottom, 32)

            // Status chip
            HStack(spacing: 10) {
                Circle().fill(Theme.rose).frame(width: 8, height: 8)
                    .shadow(color: Theme.rose.opacity(0.8), radius: 4)
                Text(statusText).font(.amoreBody(13)).foregroundStyle(Theme.cream)
            }
            .padding(.horizontal, 20).padding(.vertical, 12)
            .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))

            // Progress dots reflect real generation progress (0–100).
            HStack(spacing: 6) {
                ForEach(0..<4, id: \.self) { i in
                    Capsule()
                        .fill(i <= activeDot ? Theme.rose : Theme.border)
                        .frame(width: i == activeDot ? 20 : 6, height: 6)
                        .animation(.easeOut(duration: 0.4), value: activeDot)
                }
            }
            .padding(.top, 24)
        }
        .padding(.horizontal, 32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var activeDot: Int { min(3, viewModel.progress / 25) }

    private var subtitle: String { "Making it special just for her" }

    private var statusText: String {
        switch viewModel.status {
        case .queued:  return "Analyzing your preferences..."
        case .running: return "Curating the perfect ideas..."
        default:       return "Adding the finishing touches..."
        }
    }

    private func failure(_ message: String) -> some View {
        VStack(spacing: 16) {
            Image(systemName: "exclamationmark.triangle").font(.system(size: 44)).foregroundStyle(Theme.rose)
            Text("Couldn't generate ideas").font(.display(20, weight: .semibold)).foregroundStyle(Theme.cream)
            Text(message).font(.amoreBody(13)).foregroundStyle(Theme.muted)
                .multilineTextAlignment(.center).padding(.horizontal, 24)
            PrimaryButton(title: "Try again") { Task { await viewModel.retry() } }
                .padding(.horizontal, 48).padding(.top, 8)
            SecondaryButton(title: "Go back", icon: "chevron.left", action: onBack)
                .padding(.horizontal, 48)
        }
        .padding(32)
    }
}

// MARK: - Animated heart

/// Spinning gradient ring + pulsing heart + floating particles.
private struct HeartLoader: View {
    @State private var spin = false
    @State private var pulse = false
    @State private var floatUp = false

    var body: some View {
        ZStack {
            RadialGradient(colors: [Theme.rose.opacity(0.2), .clear], center: .center, startRadius: 0, endRadius: 120)
                .frame(width: 240, height: 240)

            Circle().strokeBorder(Theme.rose.opacity(0.15), lineWidth: 2).frame(width: 108, height: 108)
            Circle()
                .trim(from: 0, to: 0.3)
                .stroke(Theme.warmGradient, style: StrokeStyle(lineWidth: 2, lineCap: .round))
                .frame(width: 108, height: 108)
                .rotationEffect(.degrees(spin ? 360 : 0))
                .animation(.linear(duration: 4).repeatForever(autoreverses: false), value: spin)

            // Floating particles
            ForEach(0..<8, id: \.self) { i in
                Circle()
                    .fill(i % 2 == 0 ? Theme.rose : Theme.peach)
                    .frame(width: CGFloat(4 + (i % 4) * 2))
                    .offset(x: CGFloat(i * 11 - 40), y: floatUp ? -50 : 10)
                    .opacity(floatUp ? 0 : 0.7)
                    .animation(.easeOut(duration: 1.6 + Double(i) * 0.18).repeatForever(autoreverses: false)
                        .delay(Double(i) * 0.2), value: floatUp)
            }

            Image(systemName: "heart.fill")
                .font(.system(size: 50))
                .foregroundStyle(Theme.warmGradient)
                .scaleEffect(pulse ? 1.12 : 1)
                .animation(.easeInOut(duration: 1.4).repeatForever(autoreverses: true), value: pulse)
        }
        .onAppear { spin = true; pulse = true; floatUp = true }
    }
}
