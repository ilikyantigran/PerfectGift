import SwiftUI
import PerfectGiftKit

/// Occasion input: pick a holiday, a budget band, free-form preferences and a model tier,
/// then submit to start a generation. Reference data loads from the Catalog endpoints.
struct OccasionInputView: View {
    @StateObject private var viewModel: OccasionInputViewModel
    /// Called with the assembled request when the user taps "Find surprises".
    let onSubmit: (RequestGenerationRequest) -> Void

    init(viewModel: OccasionInputViewModel, onSubmit: @escaping (RequestGenerationRequest) -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onSubmit = onSubmit
    }

    var body: some View {
        Form {
            Section("Occasion") {
                Picker("Holiday", selection: $viewModel.selectedHolidayId) {
                    Text("None").tag(String?.none)
                    ForEach(viewModel.holidays) { holiday in
                        Text(holiday.name).tag(Optional(holiday.id))
                    }
                }
            }

            Section("Budget") {
                if viewModel.budgetBands.isEmpty {
                    Text("Any budget").foregroundStyle(.secondary)
                } else {
                    Picker("Budget", selection: $viewModel.selectedBudgetBandId) {
                        Text("Any").tag(String?.none)
                        ForEach(viewModel.budgetBands) { band in
                            Text(band.label).tag(Optional(band.id))
                        }
                    }
                    .pickerStyle(.segmented)
                }
            }

            Section("Preferences") {
                TextField(
                    "e.g. loves hiking, hates clutter, into pottery…",
                    text: $viewModel.preferencesText,
                    axis: .vertical
                )
                .lineLimit(3...6)
            }

            Section("Model") {
                Picker("Tier", selection: $viewModel.tier) {
                    ForEach(ModelTier.allCases, id: \.self) { tier in
                        Text(tier.displayName).tag(tier)
                    }
                }
                .pickerStyle(.segmented)
            }

            if let error = viewModel.errorMessage {
                Section { Text(error).foregroundStyle(.red).font(.footnote) }
            }

            Section {
                Button {
                    onSubmit(viewModel.makeRequest())
                } label: {
                    Text("Find surprises")
                        .frame(maxWidth: .infinity)
                        .fontWeight(.semibold)
                }
                .disabled(!viewModel.canSubmit)
            }
        }
        .task { await viewModel.loadReferenceData() }
        .overlay {
            if viewModel.isLoadingReference && viewModel.holidays.isEmpty {
                ProgressView("Loading…")
            }
        }
    }
}
