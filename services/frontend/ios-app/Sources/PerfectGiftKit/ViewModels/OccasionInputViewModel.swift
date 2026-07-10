import Foundation
import Combine

/// Drives the occasion-input screen: loads reference data (holidays, budget bands),
/// holds the form state, and builds the `RequestGenerationRequest` to submit.
@MainActor
public final class OccasionInputViewModel: ObservableObject {
    @Published public private(set) var holidays: [Holiday] = []
    @Published public private(set) var budgetBands: [BudgetBand] = []
    @Published public private(set) var isLoadingReference = false
    @Published public var errorMessage: String?

    // Form state
    @Published public var selectedHolidayId: String?
    @Published public var selectedBudgetBandId: String?
    @Published public var preferencesText: String = ""
    @Published public var tier: ModelTier = .sonnet
    /// Optional poll to sharpen the generation (set after a poll is created/answered).
    @Published public var pollId: String?

    private let api: APIClient

    public init(api: APIClient) {
        self.api = api
    }

    public var canSubmit: Bool {
        selectedHolidayId != nil || !preferencesText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    public func loadReferenceData() async {
        isLoadingReference = true
        errorMessage = nil
        defer { isLoadingReference = false }
        do {
            async let holidays = api.holidays()
            async let categories = api.categories()
            self.holidays = try await holidays
            self.budgetBands = try await categories.budgetBands
        } catch {
            errorMessage = (error as? APIError)?.userMessage ?? error.localizedDescription
        }
    }

    /// Builds the request from current form state. `budget_band` sends the band's id.
    public func makeRequest() -> RequestGenerationRequest {
        let prefs = preferencesText.trimmingCharacters(in: .whitespacesAndNewlines)
        return RequestGenerationRequest(
            holidayId: selectedHolidayId,
            budgetBand: selectedBudgetBandId,
            preferencesText: prefs.isEmpty ? nil : prefs,
            pollId: pollId,
            tier: tier
        )
    }
}
