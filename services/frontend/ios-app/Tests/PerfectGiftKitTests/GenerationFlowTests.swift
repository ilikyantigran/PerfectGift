import Testing
import Foundation
@testable import PerfectGiftKit

/// The defining pattern: submit → observe (poll) → render ideas when ready.
@Suite @MainActor struct GenerationFlowTests {

    @Test func submitThenObserveReachesReadyWithIdeas() async {
        let fake = FakeAPIClient()
        fake.generationAccepted = .success(GenerationAccepted(requestId: "req_1", status: .queued))
        // Poll returns running twice, then ready with ranked ideas.
        fake.generationQueue = [
            GenerationResult(requestId: "req_1", status: .running, progress: 25),
            GenerationResult(requestId: "req_1", status: .running, progress: 70),
            GenerationResult(requestId: "req_1", status: .ready, progress: 100, ideas: [
                Idea(id: "i1", title: "Hot air balloon", rank: 1),
                Idea(id: "i2", title: "Spa day", rank: 2)
            ])
        ]
        let vm = GenerationViewModel(api: fake, pollInterval: 0, sleeper: instantSleeper)

        await vm.submit(RequestGenerationRequest(holidayId: "h1", tier: .sonnet))
        await waitUntil { vm.phase == .ready }

        #expect(vm.phase == .ready)
        #expect(vm.status == .ready)
        #expect(vm.progress == 100)
        #expect(vm.ideas.map(\.id) == ["i1", "i2"])
        #expect(vm.requestId == "req_1")
        #expect(fake.generationPollCount() >= 3)
    }

    @Test func submitSendsIdempotencyKey() async {
        let fake = FakeAPIClient()
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .ready, ideas: [])]
        let vm = GenerationViewModel(api: fake, pollInterval: 0, sleeper: instantSleeper)

        await vm.submit(RequestGenerationRequest(preferencesText: "cozy"))
        await waitUntil { vm.phase == .ready }

        #expect(fake.generationSubmitCount() == 1)
        #expect(fake.lastIdempotencyKey() != nil)   // an Idempotency-Key must be sent
    }

    @Test func failedStatusSurfacesFailure() async {
        let fake = FakeAPIClient()
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .failed)]
        let vm = GenerationViewModel(api: fake, pollInterval: 0, sleeper: instantSleeper)

        await vm.submit(RequestGenerationRequest(holidayId: "h1"))
        await waitUntil { if case .failed = vm.phase { return true }; return false }

        guard case .failed = vm.phase else { Issue.record("expected failed phase"); return }
        #expect(vm.status == .failed)
    }

    @Test func retryReSubmitsWithFreshIdempotencyKey() async {
        let fake = FakeAPIClient()
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .failed)]
        let vm = GenerationViewModel(api: fake, pollInterval: 0, sleeper: instantSleeper)

        await vm.submit(RequestGenerationRequest(holidayId: "h1"))
        await waitUntil { if case .failed = vm.phase { return true }; return false }
        let firstKey = fake.lastIdempotencyKey()

        // Now let it succeed on retry.
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .ready, ideas: [Idea(id: "i1", title: "Gift")])]
        await vm.retry()
        await waitUntil { vm.phase == .ready }

        #expect(fake.generationSubmitCount() == 2)
        #expect(fake.lastIdempotencyKey() != firstKey)   // retry uses a new idempotency key
    }

    @Test func pushAdvancesToReadyImmediately() async {
        let fake = FakeAPIClient()
        fake.generationAccepted = .success(GenerationAccepted(requestId: "req_1", status: .queued))
        // Stay running on the timed loop; the push-driven poll flips it to ready.
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .running, progress: 10)]
        let vm = GenerationViewModel(api: fake, pollInterval: 999, sleeper: instantSleeper)
        await vm.submit(RequestGenerationRequest(holidayId: "h1"))
        await waitUntil { vm.status == .running }

        // Simulate a push: now the next poll returns ready.
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .ready, ideas: [Idea(id: "i9", title: "Surprise")])]
        await vm.onPushReceived(requestId: "req_1")

        #expect(vm.phase == .ready)
        #expect(vm.ideas.first?.id == "i9")
    }

    @Test func pushForOtherRequestIsIgnored() async {
        let fake = FakeAPIClient()
        fake.generationQueue = [GenerationResult(requestId: "req_1", status: .running)]
        let vm = GenerationViewModel(api: fake, pollInterval: 999, sleeper: instantSleeper)
        await vm.submit(RequestGenerationRequest(holidayId: "h1"))
        await waitUntil { vm.status == .running }
        let pollsBefore = fake.generationPollCount()

        await vm.onPushReceived(requestId: "some_other_request")
        #expect(fake.generationPollCount() == pollsBefore)   // no poll for another request
        vm.cancelObserving()
    }
}
