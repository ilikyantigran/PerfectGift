package com.perfectgift.android

import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.NetworkModule
import com.perfectgift.android.data.remote.dto.GenerationStatus
import com.perfectgift.android.data.remote.dto.IdeaDto
import com.perfectgift.android.data.repository.PerfectGiftRepositoryImpl
import com.perfectgift.android.ui.generation.GenerationPhase
import com.perfectgift.android.ui.generation.GenerationViewModel
import com.perfectgift.android.util.FakeGatewayApi
import com.perfectgift.android.util.FakeTokenStore
import com.perfectgift.android.util.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class GenerationFlowTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val gson = NetworkModule.gson()

    private fun repo(api: FakeGatewayApi) =
        PerfectGiftRepositoryImpl(api, SessionManager(FakeTokenStore()), gson)

    @Test
    fun `submit then observe advances queued to ready and renders ranked ideas`() =
        runTest(mainDispatcherRule.dispatcher.scheduler) {
            val api = FakeGatewayApi().apply {
                generationScript = ArrayDeque(
                    listOf(
                        GenerationStatus(requestId = "req-1", status = "queued", progress = 0),
                        GenerationStatus(requestId = "req-1", status = "running", progress = 60),
                        GenerationStatus(
                            requestId = "req-1",
                            status = "ready",
                            progress = 100,
                            ideas = listOf(
                                IdeaDto(id = "b", title = "Second", rank = 2),
                                IdeaDto(id = "a", title = "First", rank = 1),
                            ),
                        ),
                    ),
                )
            }
            val vm = GenerationViewModel(repo(api))

            vm.submit()
            advanceUntilIdle()

            val state = vm.state.value
            assertEquals(GenerationPhase.IDEAS, state.phase)
            assertEquals(2, state.ideas.size)
            // Sorted by rank ascending.
            assertEquals("First", state.ideas.first().title)
            assertEquals(100, state.progress)
            // Idempotency-Key must be sent on submit so a retry is safe (SERVICE.md §3).
            assertFalse(api.lastIdempotencyKey.isNullOrBlank())
        }

    @Test
    fun `failed generation surfaces a graceful error, never a dead end`() =
        runTest(mainDispatcherRule.dispatcher.scheduler) {
            val api = FakeGatewayApi().apply {
                generationScript = ArrayDeque(
                    listOf(GenerationStatus(requestId = "req-1", status = "failed")),
                )
            }
            val vm = GenerationViewModel(repo(api))

            vm.submit()
            advanceUntilIdle()

            assertEquals(GenerationPhase.FAILED, vm.state.value.phase)
            assertTrue(vm.state.value.error?.isNotBlank() == true)
        }

    @Test
    fun `observeGeneration emits each status and terminates on ready`() =
        runTest(mainDispatcherRule.dispatcher.scheduler) {
            val api = FakeGatewayApi().apply {
                generationScript = ArrayDeque(
                    listOf(
                        GenerationStatus(status = "running", progress = 30),
                        GenerationStatus(status = "ready", progress = 100, ideas = emptyList()),
                    ),
                )
            }
            val emissions = mutableListOf<String?>()
            repo(api).observeGeneration("req-1").collect { result ->
                if (result is ApiResult.Success) emissions.add(result.data.status)
            }
            assertEquals(listOf("running", "ready"), emissions)
        }
}
