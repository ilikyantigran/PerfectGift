import SwiftUI
import PerfectGiftKit

/// Renders the ranked ideas, lets the user save favourites, refine (regenerate) and hand
/// off to poll creation to sharpen the next round.
struct IdeasView: View {
    @StateObject private var viewModel: IdeasViewModel
    let onRefine: (String) -> Void
    let onCreatePoll: () -> Void

    @State private var showRefine = false
    @State private var refinementText = ""

    init(viewModel: IdeasViewModel, onRefine: @escaping (String) -> Void, onCreatePoll: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onRefine = onRefine
        self.onCreatePoll = onCreatePoll
    }

    var body: some View {
        List {
            ForEach(viewModel.ideas) { idea in
                IdeaRow(idea: idea, isSaved: viewModel.isSaved(idea)) {
                    Task { await viewModel.toggleSave(idea) }
                }
            }
            Section {
                Button("Refine these ideas") { showRefine = true }
                Button("Create a poll for your partner") { onCreatePoll() }
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Ideas")
        .alert("Refine", isPresented: $showRefine) {
            TextField("e.g. more experiences, under $100", text: $refinementText)
            Button("Regenerate") { onRefine(refinementText); refinementText = "" }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("Tell us what to change and we'll try again.")
        }
    }
}

private struct IdeaRow: View {
    let idea: Idea
    let isSaved: Bool
    let onSave: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                if let rank = idea.rank {
                    Text("#\(rank)")
                        .font(.caption.bold())
                        .foregroundStyle(.tint)
                }
                Text(idea.title).font(.headline)
                Spacer()
                Button(action: onSave) {
                    Image(systemName: isSaved ? "heart.fill" : "heart")
                        .foregroundStyle(isSaved ? .red : .secondary)
                }
                .buttonStyle(.plain)
            }
            if let why = idea.whyItFits {
                Text(why).font(.subheadline).foregroundStyle(.secondary)
            }
            HStack(spacing: 12) {
                if let cost = idea.roughCost {
                    Label(cost, systemImage: "tag").font(.caption)
                }
                if let how = idea.howTo {
                    Label(how, systemImage: "list.bullet").font(.caption).lineLimit(1)
                }
            }
            .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
    }
}
