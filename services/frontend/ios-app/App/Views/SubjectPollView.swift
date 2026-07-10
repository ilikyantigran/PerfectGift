import SwiftUI
import PerfectGiftKit

/// The Subject-side poll rendered natively — used for the handed-over-phone case and for
/// universal-link deep opens. No auth: it fetches the poll by opaque link token and posts
/// answers back through the same token route.
struct SubjectPollView: View {
    @StateObject private var viewModel: SubjectPollViewModel

    init(viewModel: SubjectPollViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        Group {
            switch viewModel.phase {
            case .loading:
                ProgressView("Loading poll…")
            case .ready, .submitting:
                form
            case .submitted:
                ContentUnavailableView("Thanks!", systemImage: "checkmark.circle.fill",
                                       description: Text("Your answers were sent."))
            case let .failed(message):
                ContentUnavailableView("Couldn't load poll", systemImage: "exclamationmark.triangle",
                                       description: Text(message))
            }
        }
        .navigationTitle(viewModel.poll?.title ?? "Poll")
        .task { await viewModel.load() }
    }

    private var form: some View {
        Form {
            ForEach(viewModel.poll?.questions ?? []) { question in
                Section(question.text) {
                    questionInput(question)
                }
            }
            Section {
                Button {
                    Task { await viewModel.submit() }
                } label: {
                    Text("Submit answers").frame(maxWidth: .infinity).fontWeight(.semibold)
                }
                .disabled(viewModel.phase == .submitting)
            }
        }
    }

    @ViewBuilder
    private func questionInput(_ question: Question) -> some View {
        switch question.kind {
        case .text:
            TextField("Your answer",
                      text: Binding(
                        get: { viewModel.textAnswers[question.id] ?? "" },
                        set: { viewModel.textAnswers[question.id] = $0 }),
                      axis: .vertical)
                .lineLimit(2...5)

        case .singleChoice:
            ForEach(question.options ?? [], id: \.self) { option in
                Button {
                    viewModel.singleChoice[question.id] = option
                } label: {
                    HStack {
                        Text(option)
                        Spacer()
                        if viewModel.singleChoice[question.id] == option {
                            Image(systemName: "checkmark").foregroundStyle(.tint)
                        }
                    }
                }
                .buttonStyle(.plain)
            }

        case .multiChoice:
            ForEach(question.options ?? [], id: \.self) { option in
                Button {
                    viewModel.toggleMultiChoice(questionId: question.id, option: option)
                } label: {
                    HStack {
                        Image(systemName: (viewModel.multiChoice[question.id]?.contains(option) ?? false)
                              ? "checkmark.square.fill" : "square")
                        Text(option)
                    }
                }
                .buttonStyle(.plain)
            }

        case .unknown:
            Text("Unsupported question").foregroundStyle(.secondary)
        }
    }
}
