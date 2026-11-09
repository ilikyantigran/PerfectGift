import SwiftUI
import PerfectGiftKit

/// Owner-side poll creation and sharing (Amore-themed). Creates a poll, then surfaces the
/// share link (and a native ShareLink) plus a live view of any Subject responses.
struct PollCreateView: View {
    @StateObject private var viewModel: PollCreateViewModel
    let onBack: () -> Void

    init(viewModel: PollCreateViewModel, onBack: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onBack = onBack
    }

    var body: some View {
        ZStack {
            ScreenBackground()
            ScrollView(showsIndicators: false) {
                VStack(alignment: .leading, spacing: 16) {
                    header
                    if let created = viewModel.created {
                        shareCard(created)
                        responsesCard
                    } else {
                        editor
                    }
                    if let error = viewModel.errorMessage {
                        Text(error).font(.amoreBody(12)).foregroundStyle(Theme.rose).padding(.horizontal, 24)
                    }
                }
                .padding(.bottom, 32)
            }
            if viewModel.isWorking {
                Color.black.opacity(0.3).ignoresSafeArea()
                ProgressView().tint(Theme.cream)
            }
        }
    }

    private var header: some View {
        HStack(spacing: 12) {
            BackButton(action: onBack)
            VStack(alignment: .leading, spacing: 2) {
                Text("PARTNER POLL").font(.amoreBody(11)).tracking(1.1).foregroundStyle(Theme.muted)
                Text("Create a poll").font(.display(20, weight: .semibold)).foregroundStyle(Theme.cream)
            }
            Spacer()
        }
        .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 4)
    }

    // MARK: Editor

    private var editor: some View {
        VStack(alignment: .leading, spacing: 16) {
            VStack(alignment: .leading, spacing: 8) {
                FieldLabel(text: "Title")
                TextField("", text: $viewModel.title,
                          prompt: Text("Poll title").foregroundColor(Theme.muted.opacity(0.6)))
                    .font(.amoreBody(14)).foregroundStyle(Theme.cream).tint(Theme.rose)
                    .padding(.horizontal, 16).padding(.vertical, 14).surfaceCard(cornerRadius: 16)
            }

            VStack(alignment: .leading, spacing: 8) {
                FieldLabel(text: "Questions your partner will answer")
                VStack(spacing: 10) {
                    ForEach(viewModel.questions) { q in
                        HStack(spacing: 12) {
                            Image(systemName: icon(for: q.type)).font(.system(size: 16)).foregroundStyle(Theme.rose)
                                .frame(width: 36, height: 36)
                                .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Theme.rose.opacity(0.12)))
                            VStack(alignment: .leading, spacing: 2) {
                                Text(q.prompt).font(.amoreBody(14, weight: .medium)).foregroundStyle(Theme.cream)
                                    .fixedSize(horizontal: false, vertical: true)
                                Text(typeLabel(q.type)).font(.amoreBody(12)).foregroundStyle(Theme.muted)
                            }
                            Spacer(minLength: 0)
                        }
                        .padding(14)
                        .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.surface))
                        .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
                    }
                }
                Text("A ready-made set for now — editable custom questions are coming soon.")
                    .font(.amoreBody(11)).foregroundStyle(Theme.muted)
            }

            PrimaryButton(title: "Create & get link", icon: "link") { Task { await viewModel.createPoll() } }
        }
        .padding(.horizontal, 24)
    }

    // MARK: Share + responses

    private func shareCard(_ created: CreatePollResponse) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            FieldLabel(text: "Share this link")
            if let url = viewModel.shareURL {
                ShareLink(item: url) {
                    HStack(spacing: 10) {
                        Image(systemName: "square.and.arrow.up").font(.system(size: 16, weight: .semibold))
                        Text("Share poll link").font(.amoreBody(15, weight: .semibold))
                    }
                    .foregroundStyle(Theme.cream).frame(maxWidth: .infinity).padding(.vertical, 16)
                    .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Theme.ctaGradient)
                        .shadow(color: Theme.rose.opacity(0.4), radius: 18, y: 10))
                }
                Text(url.absoluteString).font(.amoreBody(12)).foregroundStyle(Theme.muted).textSelection(.enabled)
            } else {
                Text("Link token: \(created.linkToken)").font(.amoreBody(12)).foregroundStyle(Theme.softText).textSelection(.enabled)
            }
            if let expires = created.expiresAt {
                Text("Expires \(expires)").font(.amoreBody(11)).foregroundStyle(Theme.muted)
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 22, style: .continuous).fill(Theme.surface))
        .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
        .padding(.horizontal, 24)
    }

    private var responsesCard: some View {
        VStack(alignment: .leading, spacing: 12) {
            FieldLabel(text: "Responses")
            if viewModel.responses.isEmpty {
                Text("No responses yet.").font(.amoreBody(13)).foregroundStyle(Theme.muted)
            } else {
                ForEach(viewModel.responses) { response in
                    VStack(alignment: .leading, spacing: 2) {
                        Text("\(response.answers.count) answers").font(.amoreBody(14)).foregroundStyle(Theme.cream)
                        if let at = response.submittedAt {
                            Text(at).font(.amoreBody(11)).foregroundStyle(Theme.muted)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            SecondaryButton(title: "Refresh", icon: "arrow.clockwise") { Task { await viewModel.loadResponses() } }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 22, style: .continuous).fill(Theme.surface))
        .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
        .padding(.horizontal, 24)
    }

    private func icon(for type: QuestionKind) -> String {
        switch type {
        case .text: return "text.alignleft"
        case .singleChoice: return "circle.circle"
        case .multiChoice: return "checklist"
        case .unknown: return "questionmark"
        }
    }

    private func typeLabel(_ type: QuestionKind) -> String {
        switch type {
        case .text: return "Free text"
        case .singleChoice: return "Single choice"
        case .multiChoice: return "Multiple choice"
        case .unknown: return "—"
        }
    }
}
