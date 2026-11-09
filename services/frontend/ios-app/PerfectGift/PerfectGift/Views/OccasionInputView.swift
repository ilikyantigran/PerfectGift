import SwiftUI
import PerfectGiftKit

/// Occasion input (Amore "Main" design): greeting header + avatar, a hero line, an occasion
/// dropdown, a budget grid, free-form preferences and a model tier, then "Generate Magic".
/// Reference data (holidays, budget bands) loads from the Catalog endpoints.
struct OccasionInputView: View {
    @StateObject private var viewModel: OccasionInputViewModel
    let onSubmit: (RequestGenerationRequest) -> Void
    let onProfile: () -> Void

    @State private var showPicker = false

    init(viewModel: OccasionInputViewModel,
         onSubmit: @escaping (RequestGenerationRequest) -> Void,
         onProfile: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onSubmit = onSubmit
        self.onProfile = onProfile
    }

    private var selectedHolidayName: String? {
        viewModel.holidays.first { $0.id == viewModel.selectedHolidayId }?.name
    }

    var body: some View {
        ZStack {
            ScreenBackground()
            ScrollView(showsIndicators: false) {
                VStack(alignment: .leading, spacing: 0) {
                    header
                    hero
                    form
                }
                .padding(.bottom, 40)
            }
            if viewModel.isLoadingReference && viewModel.holidays.isEmpty {
                ProgressView().tint(Theme.cream)
            }
        }
        .task { await viewModel.loadReferenceData() }
    }

    // MARK: Header

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 1) {
                Text("Good evening,").font(.amoreBody(12)).foregroundStyle(Theme.muted)
                Text(greetingName).font(.display(20, weight: .semibold)).foregroundStyle(Theme.cream)
            }
            Spacer()
            Button(action: onProfile) {
                Circle()
                    .fill(LinearGradient(colors: [Theme.rose.opacity(0.3), Theme.peach.opacity(0.2)],
                                         startPoint: .topLeading, endPoint: .bottomTrailing))
                    .frame(width: 40, height: 40)
                    .overlay(Circle().strokeBorder(Theme.rose.opacity(0.35), lineWidth: 1))
                    .overlay(Text(avatarInitial).font(.amoreBody(15, weight: .semibold)).foregroundStyle(Theme.cream))
                    .shadow(color: Theme.rose.opacity(0.2), radius: 8, y: 4)
            }
        }
        .padding(.horizontal, 24).padding(.top, 12).padding(.bottom, 20)
    }

    private var greetingName: String { "there" }
    private var avatarInitial: String { "A" }

    private var hero: some View {
        VStack(alignment: .leading, spacing: 8) {
            (Text("Plan the perfect\n").foregroundColor(Theme.cream)
             + Text("surprise").foregroundColor(Theme.rose).italic()
             + Text(" for her").foregroundColor(Theme.cream))
                .font(.display(30, weight: .bold))
                .lineSpacing(4)
            Text("Tell us about the occasion and we'll craft something she'll never forget.")
                .font(.amoreBody(13)).foregroundStyle(Theme.muted).lineSpacing(3)
        }
        .padding(.horizontal, 24).padding(.bottom, 28)
    }

    // MARK: Form

    private var form: some View {
        VStack(alignment: .leading, spacing: 16) {
            occasionField
            budgetField
            preferencesField
            tierField

            if let error = viewModel.errorMessage {
                Text(error).font(.amoreBody(12)).foregroundStyle(Theme.rose)
            }

            PrimaryButton(title: "Generate Magic", icon: "sparkles", enabled: viewModel.canSubmit) {
                onSubmit(viewModel.makeRequest())
            }
            .padding(.top, 4)
        }
        .padding(.horizontal, 24)
    }

    private var occasionField: some View {
        VStack(alignment: .leading, spacing: 8) {
            FieldLabel(text: "Occasion")
            VStack(spacing: 4) {
                Button(action: { withAnimation(.easeOut(duration: 0.2)) { showPicker.toggle() } }) {
                    HStack {
                        Text(selectedHolidayName ?? "Select occasion")
                            .font(.amoreBody(14))
                            .foregroundStyle(selectedHolidayName == nil ? Theme.muted.opacity(0.5) : Theme.cream)
                        Spacer()
                        Image(systemName: "chevron.down")
                            .font(.system(size: 13, weight: .medium)).foregroundStyle(Theme.muted)
                            .rotationEffect(.degrees(showPicker ? 180 : 0))
                    }
                    .padding(.horizontal, 16).padding(.vertical, 15)
                    .surfaceCard(active: showPicker)
                }

                if showPicker {
                    VStack(spacing: 0) {
                        pickerRow(name: "None", id: nil)
                        ForEach(viewModel.holidays) { holiday in
                            Divider().overlay(Theme.border.opacity(0.5))
                            pickerRow(name: holiday.name, id: holiday.id)
                        }
                    }
                    .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(Theme.elevated))
                    .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Theme.border, lineWidth: 1))
                    .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
                    .shadow(color: .black.opacity(0.5), radius: 20, y: 16)
                }
            }
        }
    }

    private func pickerRow(name: String, id: String?) -> some View {
        let selected = viewModel.selectedHolidayId == id
        return Button(action: {
            viewModel.selectedHolidayId = id
            withAnimation(.easeOut(duration: 0.2)) { showPicker = false }
        }) {
            HStack {
                Text(name).font(.amoreBody(14)).foregroundStyle(selected ? Theme.rose : Theme.cream)
                Spacer()
            }
            .padding(.horizontal, 16).padding(.vertical, 12)
            .background(selected ? Theme.rose.opacity(0.1) : .clear)
        }
    }

    private var budgetField: some View {
        VStack(alignment: .leading, spacing: 8) {
            FieldLabel(text: "Budget")
            if viewModel.budgetBands.isEmpty {
                Text("Any budget").font(.amoreBody(13)).foregroundStyle(Theme.muted)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 16).padding(.vertical, 14).surfaceCard(cornerRadius: 14)
            } else {
                LazyVGrid(columns: [GridItem(.flexible(), spacing: 8), GridItem(.flexible(), spacing: 8)], spacing: 8) {
                    ForEach(viewModel.budgetBands) { band in
                        let selected = band.id == viewModel.selectedBudgetBandId
                        Button(action: {
                            withAnimation(.easeOut(duration: 0.15)) {
                                viewModel.selectedBudgetBandId = selected ? nil : band.id
                            }
                        }) {
                            Text(band.label)
                                .font(.amoreBody(13, weight: selected ? .medium : .regular))
                                .foregroundStyle(selected ? Theme.rose : Theme.muted)
                                .frame(maxWidth: .infinity).padding(.vertical, 12)
                                .background(RoundedRectangle(cornerRadius: 14, style: .continuous)
                                    .fill(selected ? Theme.rose.opacity(0.15) : Theme.surface))
                                .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous)
                                    .strokeBorder(selected ? Theme.rose.opacity(0.5) : Theme.border, lineWidth: 1))
                        }
                    }
                }
            }
        }
    }

    private var preferencesField: some View {
        VStack(alignment: .leading, spacing: 8) {
            FieldLabel(text: "Preferences  (optional)")
            AmoreTextEditor(placeholder: "She loves Italian food, hates heights, her favorite scent is vanilla...",
                            text: $viewModel.preferencesText)
        }
    }

    private var tierField: some View {
        VStack(alignment: .leading, spacing: 8) {
            FieldLabel(text: "Model")
            HStack(spacing: 8) {
                ForEach(ModelTier.allCases, id: \.self) { tier in
                    let selected = viewModel.tier == tier
                    Button(action: { withAnimation(.easeOut(duration: 0.15)) { viewModel.tier = tier } }) {
                        Text(tier.displayName)
                            .font(.amoreBody(13, weight: selected ? .medium : .regular))
                            .foregroundStyle(selected ? Theme.rose : Theme.muted)
                            .frame(maxWidth: .infinity).padding(.vertical, 12)
                            .background(RoundedRectangle(cornerRadius: 14, style: .continuous)
                                .fill(selected ? Theme.rose.opacity(0.15) : Theme.surface))
                            .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous)
                                .strokeBorder(selected ? Theme.rose.opacity(0.5) : Theme.border, lineWidth: 1))
                    }
                }
            }
        }
    }
}
