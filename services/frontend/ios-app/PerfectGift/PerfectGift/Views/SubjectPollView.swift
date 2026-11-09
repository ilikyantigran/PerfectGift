import SwiftUI
import PerfectGiftKit

/// The Subject-side poll rendered natively (Amore "Poll" design): one question per step with
/// a progress bar, radio/checkbox options and a free-text step. Used for the handed-over-phone
/// case and universal-link deep opens. No auth — it fetches by opaque link token.
struct SubjectPollView: View {
    @StateObject private var viewModel: SubjectPollViewModel
    let onClose: () -> Void
    @State private var step = 0

    init(viewModel: SubjectPollViewModel, onClose: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onClose = onClose
    }

    private var questions: [Question] { viewModel.poll?.questions ?? [] }

    var body: some View {
        ZStack {
            ScreenBackground()
            switch viewModel.phase {
            case .loading:
                ProgressView().tint(Theme.cream)
            case .ready, .submitting:
                if questions.isEmpty { emptyState } else { poll }
            case .submitted:
                statusState(icon: "checkmark.circle.fill", title: "Thanks!",
                            message: "Your answers were sent.", tint: Theme.rose)
            case let .failed(message):
                statusState(icon: "exclamationmark.triangle", title: "Couldn't load poll",
                            message: message, tint: Theme.peach)
            }
        }
        .task { await viewModel.load() }
    }

    // MARK: Poll stepper

    private var poll: some View {
        let q = questions[min(step, questions.count - 1)]
        let isLast = step == questions.count - 1
        return VStack(alignment: .leading, spacing: 0) {
            header
            progress
            ScrollView(showsIndicators: false) {
                VStack(alignment: .leading, spacing: 24) {
                    Text(q.prompt).font(.display(22, weight: .semibold)).foregroundStyle(Theme.cream)
                        .lineSpacing(3).fixedSize(horizontal: false, vertical: true)
                    questionInput(q)
                }
                .padding(.horizontal, 24).padding(.top, 4)
            }
            PrimaryButton(title: isLast ? "Submit Poll" : "Next Question", enabled: canAdvance(q)) {
                if isLast { Task { await viewModel.submit() } }
                else { withAnimation(.easeInOut(duration: 0.2)) { step += 1 } }
            }
            .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 8)
        }
    }

    private var header: some View {
        HStack(spacing: 12) {
            BackButton { if step == 0 { onClose() } else { withAnimation { step -= 1 } } }
            VStack(alignment: .leading, spacing: 2) {
                Text("PARTNER POLL").font(.amoreBody(11)).tracking(1.1).foregroundStyle(Theme.muted)
                Text(viewModel.poll?.title ?? "Tell us about you")
                    .font(.display(18, weight: .semibold)).foregroundStyle(Theme.cream).lineLimit(1)
            }
            Spacer()
        }
        .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 16)
    }

    private var progress: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 6) {
                ForEach(0..<questions.count, id: \.self) { i in
                    Capsule().fill(i <= step ? Theme.rose : Theme.border).frame(height: 3)
                }
            }
            Text("Question \(step + 1) of \(questions.count)").font(.amoreBody(12)).foregroundStyle(Theme.muted)
        }
        .padding(.horizontal, 24).padding(.bottom, 24)
    }

    // MARK: Inputs

    @ViewBuilder
    private func questionInput(_ q: Question) -> some View {
        switch q.type {
        case .text:
            AmoreTextEditor(placeholder: "A dream destination, a favorite memory, something you always wanted to try...",
                            text: Binding(get: { viewModel.textAnswers[q.id] ?? "" },
                                          set: { viewModel.textAnswers[q.id] = $0 }),
                            height: 120)
        case .singleChoice:
            VStack(spacing: 10) {
                ForEach(q.options ?? []) { opt in
                    optionRow(label: opt.label, selected: viewModel.singleChoice[q.id] == opt.id, isMulti: false) {
                        viewModel.singleChoice[q.id] = opt.id
                    }
                }
            }
        case .multiChoice:
            VStack(spacing: 10) {
                ForEach(q.options ?? []) { opt in
                    let selected = viewModel.multiChoice[q.id]?.contains(opt.id) ?? false
                    optionRow(label: opt.label, selected: selected, isMulti: true) {
                        viewModel.toggleMultiChoice(questionId: q.id, option: opt.id)
                    }
                }
            }
        case .unknown:
            Text("Unsupported question").font(.amoreBody(13)).foregroundStyle(Theme.muted)
        }
    }

    private func optionRow(label: String, selected: Bool, isMulti: Bool, action: @escaping () -> Void) -> some View {
        Button(action: { withAnimation(.easeOut(duration: 0.15)) { action() } }) {
            HStack(spacing: 12) {
                ZStack {
                    RoundedRectangle(cornerRadius: isMulti ? 6 : 10, style: .continuous)
                        .strokeBorder(selected ? Theme.rose : Theme.border, lineWidth: 2)
                        .background(RoundedRectangle(cornerRadius: isMulti ? 6 : 10, style: .continuous)
                            .fill(selected ? Theme.rose : .clear))
                        .frame(width: 20, height: 20)
                    if selected {
                        Image(systemName: "checkmark").font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.canvas)
                    }
                }
                Text(label).font(.amoreBody(14, weight: selected ? .medium : .regular))
                    .foregroundStyle(selected ? Theme.cream : Theme.muted)
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 16).padding(.vertical, 15)
            .background(RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(selected ? Theme.rose.opacity(0.15) : Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous)
                .strokeBorder(selected ? Theme.rose.opacity(0.5) : Theme.border, lineWidth: 1))
        }
    }

    private func canAdvance(_ q: Question) -> Bool {
        switch q.type {
        case .text: return true
        case .singleChoice: return viewModel.singleChoice[q.id] != nil
        case .multiChoice: return !(viewModel.multiChoice[q.id]?.isEmpty ?? true)
        case .unknown: return true
        }
    }

    // MARK: States

    private var emptyState: some View {
        statusState(icon: "tray", title: viewModel.poll?.title ?? "Poll",
                    message: "This poll has no questions.", tint: Theme.muted)
    }

    private func statusState(icon: String, title: String, message: String, tint: Color) -> some View {
        VStack(spacing: 16) {
            Image(systemName: icon).font(.system(size: 48)).foregroundStyle(tint)
            Text(title).font(.display(22, weight: .semibold)).foregroundStyle(Theme.cream)
            Text(message).font(.amoreBody(13)).foregroundStyle(Theme.muted).multilineTextAlignment(.center)
            SecondaryButton(title: "Close") { onClose() }.padding(.horizontal, 60).padding(.top, 8)
        }
        .padding(32)
    }
}
