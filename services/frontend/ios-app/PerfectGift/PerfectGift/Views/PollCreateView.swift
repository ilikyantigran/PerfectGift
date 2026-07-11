import SwiftUI
import PerfectGiftKit

/// Owner-side poll creation and sharing. Creates a poll, then surfaces the share link
/// (and a native ShareLink) plus a live view of any Subject responses.
struct PollCreateView: View {
    @StateObject private var viewModel: PollCreateViewModel

    init(viewModel: PollCreateViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        Form {
            if let created = viewModel.created {
                shareSection(created)
                responsesSection
            } else {
                editorSection
            }
            if let error = viewModel.errorMessage {
                Section { Text(error).foregroundStyle(.red).font(.footnote) }
            }
        }
        .navigationTitle("Poll")
        .overlay { if viewModel.isWorking { ProgressView() } }
    }

    private var editorSection: some View {
        Group {
            Section("Title") {
                TextField("Poll title", text: $viewModel.title)
            }
            Section {
                ForEach(viewModel.questions) { q in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(q.prompt).font(.subheadline)
                        Text(typeLabel(q.type)).font(.caption).foregroundStyle(.secondary)
                    }
                }
            } header: {
                Text("Questions your partner will answer")
            } footer: {
                Text("A ready-made set for now — editable custom questions are coming soon.")
            }
            Section {
                Button {
                    Task { await viewModel.createPoll() }
                } label: {
                    Text("Create & get link").frame(maxWidth: .infinity).fontWeight(.semibold)
                }
            }
        }
    }

    private func shareSection(_ created: CreatePollResponse) -> some View {
        Section("Share this link") {
            if let url = viewModel.shareURL {
                ShareLink(item: url) {
                    Label("Share poll link", systemImage: "square.and.arrow.up")
                }
                Text(url.absoluteString).font(.caption).foregroundStyle(.secondary).textSelection(.enabled)
            } else {
                Text("Link token: \(created.linkToken)").font(.caption).textSelection(.enabled)
            }
            if let expires = created.expiresAt {
                Text("Expires \(expires)").font(.caption2).foregroundStyle(.secondary)
            }
        }
    }

    private var responsesSection: some View {
        Section("Responses") {
            if viewModel.responses.isEmpty {
                Text("No responses yet.").foregroundStyle(.secondary)
            } else {
                ForEach(viewModel.responses) { response in
                    VStack(alignment: .leading) {
                        Text("\(response.answers.count) answers")
                        if let at = response.submittedAt {
                            Text(at).font(.caption2).foregroundStyle(.secondary)
                        }
                    }
                }
            }
            Button("Refresh") { Task { await viewModel.loadResponses() } }
        }
    }

    private func typeLabel(_ type: QuestionKind) -> String {
        switch type {
        case .text:         return "Free text"
        case .singleChoice: return "Single choice"
        case .multiChoice:  return "Multiple choice"
        case .unknown:      return "—"
        }
    }
}
