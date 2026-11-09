import SwiftUI
import PerfectGiftKit

/// Renders the ranked ideas (Amore "Result" design): accent-tinted cards with a cost pill,
/// the rationale, and a "how to" line — plus save (heart), refine (regenerate) and hand off
/// to poll creation. Shown inside `GeneratingView`, which already draws the background.
struct IdeasView: View {
    @StateObject private var viewModel: IdeasViewModel
    let onBack: () -> Void
    let onRefine: (String) -> Void
    let onCreatePoll: () -> Void

    @State private var showRefine = false
    @State private var refinementText = ""

    init(viewModel: IdeasViewModel, onBack: @escaping () -> Void,
         onRefine: @escaping (String) -> Void, onCreatePoll: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onBack = onBack
        self.onRefine = onRefine
        self.onCreatePoll = onCreatePoll
    }

    // Accent + emoji cycle for the cards (design uses tinted tiles with an emoji).
    private let accents: [Color] = [Theme.rose, Theme.peach, Color(hex: 0xc49ed4), Color(hex: 0x64b4a0)]
    private let emojis = ["✨", "🎁", "🌹", "💫"]

    var body: some View {
        ScrollView(showsIndicators: false) {
            VStack(alignment: .leading, spacing: 0) {
                header
                subtitle
                VStack(spacing: 14) {
                    ForEach(Array(viewModel.ideas.enumerated()), id: \.element.id) { index, idea in
                        card(idea, accent: accents[index % accents.count], emoji: emojis[index % emojis.count])
                    }
                }
                .padding(.horizontal, 24)

                VStack(spacing: 10) {
                    SecondaryButton(title: "Generate more ideas", icon: "arrow.clockwise") { showRefine = true }
                    PrimaryButton(title: "Create a poll for your partner", icon: "list.bullet.rectangle") { onCreatePoll() }
                }
                .padding(.horizontal, 24).padding(.top, 20)
            }
            .padding(.bottom, 32)
        }
        .alert("Refine", isPresented: $showRefine) {
            TextField("e.g. more experiences, under $100", text: $refinementText)
            Button("Regenerate") { onRefine(refinementText); refinementText = "" }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("Tell us what to change and we'll try again.")
        }
    }

    private var header: some View {
        HStack(spacing: 12) {
            BackButton(action: onBack)
            VStack(alignment: .leading, spacing: 2) {
                Text("YOUR SURPRISE").font(.amoreBody(11)).tracking(1.1).foregroundStyle(Theme.muted)
                Text("\(viewModel.ideas.count) Perfect Ideas").font(.display(20, weight: .semibold)).foregroundStyle(Theme.cream)
            }
            Spacer()
        }
        .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 4)
    }

    private var subtitle: some View {
        Text("Crafted just for her. Tap the heart to save an idea.")
            .font(.amoreBody(13)).foregroundStyle(Theme.muted).lineSpacing(3)
            .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 20)
    }

    private func card(_ idea: Idea, accent: Color, emoji: String) -> some View {
        let saved = viewModel.isSaved(idea)
        return VStack(alignment: .leading, spacing: 0) {
            // Header row
            HStack(alignment: .top, spacing: 12) {
                RoundedRectangle(cornerRadius: 14, style: .continuous).fill(accent.opacity(0.12))
                    .frame(width: 44, height: 44)
                    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(accent.opacity(0.35), lineWidth: 1))
                    .overlay(Text(emoji).font(.system(size: 22)))
                VStack(alignment: .leading, spacing: 6) {
                    Text(idea.title).font(.display(17, weight: .semibold)).foregroundStyle(Theme.cream)
                        .fixedSize(horizontal: false, vertical: true)
                    if let cost = idea.roughCost {
                        Text(cost).font(.amoreBody(12, weight: .medium)).foregroundStyle(accent)
                            .padding(.horizontal, 10).padding(.vertical, 3)
                            .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(accent.opacity(0.15)))
                            .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(accent.opacity(0.4), lineWidth: 1))
                    }
                }
                Spacer(minLength: 0)
                Button(action: { Task { await viewModel.toggleSave(idea) } }) {
                    Image(systemName: saved ? "heart.fill" : "heart")
                        .font(.system(size: 18)).foregroundStyle(saved ? Theme.rose : Theme.muted)
                }
            }
            .padding(18)

            Rectangle().fill(Theme.border.opacity(0.6)).frame(height: 1)

            if let why = idea.whyItFits {
                Text(why).font(.amoreBody(13)).foregroundStyle(Theme.softText).lineSpacing(4)
                    .padding(.horizontal, 18).padding(.vertical, 14)
            }

            HStack(spacing: 8) {
                if let rank = idea.rank { tag("#\(rank)") }
                if let how = idea.howTo { tag(how, lineLimit: 1) }
            }
            .padding(.horizontal, 18).padding(.bottom, 16)
        }
        .background(RoundedRectangle(cornerRadius: 22, style: .continuous)
            .fill(LinearGradient(colors: [accent.opacity(0.15), accent.opacity(0.05)],
                                 startPoint: .topLeading, endPoint: .bottomTrailing)))
        .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
    }

    private func tag(_ text: String, lineLimit: Int? = nil) -> some View {
        Text(text.uppercased()).font(.amoreBody(11, weight: .medium)).tracking(0.6)
            .foregroundStyle(Theme.muted).lineLimit(lineLimit)
            .padding(.horizontal, 10).padding(.vertical, 4)
            .background(RoundedRectangle(cornerRadius: 8, style: .continuous).fill(Theme.surface))
            .overlay(RoundedRectangle(cornerRadius: 8, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
    }
}
