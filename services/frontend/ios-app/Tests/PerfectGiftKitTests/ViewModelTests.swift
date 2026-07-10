import Testing
import Foundation
@testable import PerfectGiftKit

/// Auth, occasion-input, ideas, poll-create and subject-poll view models.
@Suite @MainActor struct ViewModelTests {

    // MARK: Sign in

    @Test func signInSuccessSetsSignedIn() async {
        let fake = FakeAPIClient()
        fake.signInResult = .success(TokenPair(accessToken: "a", refreshToken: "r", user: User(id: "u1", email: "a@b.co")))
        let auth = AuthManager(api: fake, tokens: TokenProvider(store: InMemoryTokenStore()))
        let vm = SignInViewModel(auth: auth)
        vm.email = "a@b.co"
        vm.password = "secret1"
        #expect(vm.canSubmitEmail)

        await vm.signInWithEmail()

        #expect(auth.isSignedIn)
        #expect(auth.currentUser?.id == "u1")
        #expect(fake.signInCalls.first?.provider == .email)
        #expect(vm.errorMessage == nil)
    }

    @Test func signInFailureSurfacesMessage() async {
        let fake = FakeAPIClient()
        fake.signInResult = .failure(APIError.server(status: 401, code: "invalid_credentials", message: "Wrong password", details: nil))
        let auth = AuthManager(api: fake, tokens: TokenProvider(store: InMemoryTokenStore()))
        let vm = SignInViewModel(auth: auth)
        vm.email = "a@b.co"; vm.password = "secret1"

        await vm.signInWithEmail()

        #expect(!auth.isSignedIn)
        #expect(vm.errorMessage == "Wrong password")
    }

    @Test func appleSignInUsesProviderApple() async {
        let fake = FakeAPIClient()
        fake.signInResult = .success(TokenPair(accessToken: "a", refreshToken: "r", user: User(id: "u1")))
        let auth = AuthManager(api: fake, tokens: TokenProvider(store: InMemoryTokenStore()))
        let vm = SignInViewModel(auth: auth)
        await vm.signInWithApple(idToken: "apple_id_token")
        #expect(fake.signInCalls.first?.provider == .apple)
        #expect(fake.signInCalls.first?.idToken == "apple_id_token")
    }

    // MARK: Occasion input

    @Test func occasionLoadsReferenceAndBuildsRequest() async {
        let fake = FakeAPIClient()
        fake.holidaysResult = .success([Holiday(id: "h_valentines", name: "Valentine's Day")])
        fake.categoriesResult = .success(CategoriesResponse(
            categories: [Category(id: "c1", name: "Experiences")],
            budgetBands: [BudgetBand(id: "b_mid", label: "$50–150", min: 50, max: 150, currency: "USD")]
        ))
        let vm = OccasionInputViewModel(api: fake)
        await vm.loadReferenceData()

        #expect(vm.holidays.first?.id == "h_valentines")
        #expect(vm.budgetBands.first?.id == "b_mid")

        vm.selectedHolidayId = "h_valentines"
        vm.selectedBudgetBandId = "b_mid"
        vm.preferencesText = "  loves hiking  "
        vm.tier = .opus
        #expect(vm.canSubmit)

        let req = vm.makeRequest()
        #expect(req.holidayId == "h_valentines")
        #expect(req.budgetBand == "b_mid")
        #expect(req.preferencesText == "loves hiking")   // trimmed
        #expect(req.tier == .opus)
    }

    @Test func occasionCannotSubmitWhenEmpty() {
        let vm = OccasionInputViewModel(api: FakeAPIClient())
        #expect(!vm.canSubmit)
        vm.preferencesText = "anything"
        #expect(vm.canSubmit)
    }

    // MARK: Ideas save

    @Test func toggleSaveOptimisticAndCalls() async {
        let fake = FakeAPIClient()
        let idea = Idea(id: "i1", title: "Gift")
        let vm = IdeasViewModel(api: fake, ideas: [idea])
        await vm.toggleSave(idea)
        #expect(vm.isSaved(idea))
        #expect(fake.savedIdeaIds == ["i1"])
    }

    @Test func toggleSaveRollsBackOnError() async {
        let fake = FakeAPIClient()
        fake.saveIdeaError = APIError.server(status: 500, code: "internal", message: "boom", details: nil)
        let idea = Idea(id: "i1", title: "Gift")
        let vm = IdeasViewModel(api: fake, ideas: [idea])
        await vm.toggleSave(idea)
        #expect(!vm.isSaved(idea))    // failed save rolls back the optimistic insert
        #expect(vm.errorMessage == "boom")
    }

    // MARK: Poll create

    @Test func createPollProducesShareURL() async {
        let fake = FakeAPIClient()
        fake.createPollResult = .success(CreatePollResponse(pollId: "p1", linkToken: "tok", linkUrl: "https://perfectgift.app/p/tok"))
        let vm = PollCreateViewModel(api: fake, surpriseRequestId: "req_1")
        await vm.createPoll()
        #expect(vm.created?.pollId == "p1")
        #expect(vm.shareURL?.absoluteString == "https://perfectgift.app/p/tok")
    }

    // MARK: Subject poll

    @Test func subjectPollBuildsAnswersAndSubmits() async {
        let fake = FakeAPIClient()
        fake.pollByTokenResult = .success(PollByToken(pollId: "p1", title: "T", questions: [
            Question(id: "q_notes", text: "Avoid?", kind: .text),
            Question(id: "q_budget", text: "Budget?", kind: .singleChoice, options: ["Low", "High"]),
            Question(id: "q_vibe", text: "Vibe?", kind: .multiChoice, options: ["Cozy", "Bold"])
        ]))
        let vm = SubjectPollViewModel(api: fake, token: "tok")
        await vm.load()
        #expect(vm.phase == .ready)

        vm.textAnswers["q_notes"] = "No chocolate"
        vm.singleChoice["q_budget"] = "High"
        vm.toggleMultiChoice(questionId: "q_vibe", option: "Cozy")
        vm.toggleMultiChoice(questionId: "q_vibe", option: "Bold")
        vm.toggleMultiChoice(questionId: "q_vibe", option: "Bold") // toggle off

        let answers = vm.buildAnswers()
        #expect(answers.count == 3)
        #expect(answers.first(where: { $0.questionId == "q_notes" })?.value == "No chocolate")
        #expect(answers.first(where: { $0.questionId == "q_budget" })?.value == "High")
        #expect(answers.first(where: { $0.questionId == "q_vibe" })?.values == ["Cozy"])

        await vm.submit()
        #expect(vm.phase == .submitted)
        #expect(fake.submittedAnswers.count == 1)
    }

    @Test func subjectPollLoadFailure() async {
        let fake = FakeAPIClient()
        fake.pollByTokenResult = .failure(APIError.server(status: 404, code: "not_found", message: "Poll expired", details: nil))
        let vm = SubjectPollViewModel(api: fake, token: "bad")
        await vm.load()
        guard case let .failed(msg) = vm.phase else { Issue.record("expected failed"); return }
        #expect(msg == "Poll expired")
    }
}
