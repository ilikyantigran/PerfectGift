package com.perfectgift.android.ui.generation

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.dto.BudgetBandDto
import com.perfectgift.android.data.remote.dto.HolidayDto
import com.perfectgift.android.data.remote.dto.IdeaDto
import com.perfectgift.android.data.remote.dto.RequestGenerationRequest
import com.perfectgift.android.data.repository.PerfectGiftRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Where the planner flow currently is. Drives which screen the nav host shows. */
enum class GenerationPhase { INPUT, SUBMITTING, GENERATING, IDEAS, FAILED }

data class GenerationUiState(
    val phase: GenerationPhase = GenerationPhase.INPUT,
    // Reference data for the occasion form.
    val holidays: List<HolidayDto> = emptyList(),
    val budgetBands: List<BudgetBandDto> = emptyList(),
    val referenceLoading: Boolean = false,
    // Occasion form.
    val selectedHolidayId: String? = null,
    val budgetBand: String? = null,
    val preferencesText: String = "",
    val pollId: String? = null,
    // Generation progress / results.
    val requestId: String? = null,
    val progress: Int = 0,
    val statusLabel: String = "",
    val ideas: List<IdeaDto> = emptyList(),
    val savedIdeaIds: Set<String> = emptySet(),
    val error: String? = null,
)

/**
 * The single ViewModel behind the occasion → generating → ideas flow (shared across
 * those destinations via the nav-graph back-stack entry). Implements the defining
 * submit-then-observe pattern: [submit] fires POST /v1/generations then observes status
 * on an interval until ideas are ready — never blocking the UI.
 */
class GenerationViewModel(
    private val repository: PerfectGiftRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(GenerationUiState())
    val state: StateFlow<GenerationUiState> = _state.asStateFlow()

    private var observeJob: Job? = null

    // --- Form editing ---
    fun onHolidaySelected(id: String?) = _state.update { it.copy(selectedHolidayId = id) }
    fun onBudgetSelected(band: String?) = _state.update { it.copy(budgetBand = band) }
    fun onPreferencesChange(text: String) = _state.update { it.copy(preferencesText = text) }
    fun attachPoll(pollId: String?) = _state.update { it.copy(pollId = pollId) }

    fun loadReferenceData() {
        if (_state.value.holidays.isNotEmpty() || _state.value.referenceLoading) return
        _state.update { it.copy(referenceLoading = true) }
        viewModelScope.launch {
            val holidays = repository.listHolidays()
            val categories = repository.getCategories()
            _state.update { current ->
                current.copy(
                    referenceLoading = false,
                    holidays = (holidays as? ApiResult.Success)?.data ?: current.holidays,
                    budgetBands = (categories as? ApiResult.Success)?.data?.budgetBands ?: current.budgetBands,
                )
            }
        }
    }

    /** Submit the occasion, then observe generation until ideas are ready. */
    fun submit() {
        val s = _state.value
        val request = RequestGenerationRequest(
            holidayId = s.selectedHolidayId,
            budgetBand = s.budgetBand,
            preferencesText = s.preferencesText.ifBlank { null },
            pollId = s.pollId,
        )
        _state.update { it.copy(phase = GenerationPhase.SUBMITTING, error = null, ideas = emptyList()) }
        viewModelScope.launch {
            when (val accepted = repository.requestGeneration(request)) {
                is ApiResult.Success -> observe(accepted.data.requestId)
                is ApiResult.Failure -> _state.update {
                    it.copy(phase = GenerationPhase.FAILED, error = accepted.error.message)
                }
            }
        }
    }

    private fun observe(requestId: String) {
        observeJob?.cancel()
        _state.update {
            it.copy(phase = GenerationPhase.GENERATING, requestId = requestId, progress = 0, statusLabel = "queued")
        }
        observeJob = viewModelScope.launch {
            repository.observeGeneration(requestId).collect { result ->
                when (result) {
                    is ApiResult.Success -> {
                        val status = result.data
                        when (status.status) {
                            "ready" -> _state.update {
                                it.copy(
                                    phase = GenerationPhase.IDEAS,
                                    progress = 100,
                                    statusLabel = "ready",
                                    ideas = status.ideas.orEmpty().sortedBy { idea -> idea.rank },
                                )
                            }
                            "failed" -> _state.update {
                                it.copy(
                                    phase = GenerationPhase.FAILED,
                                    error = "We couldn't come up with ideas this time. Please try again.",
                                )
                            }
                            else -> _state.update {
                                it.copy(
                                    phase = GenerationPhase.GENERATING,
                                    progress = status.progress,
                                    statusLabel = status.status ?: "running",
                                )
                            }
                        }
                    }
                    is ApiResult.Failure -> _state.update {
                        it.copy(phase = GenerationPhase.FAILED, error = result.error.message)
                    }
                }
            }
        }
    }

    /** Refine/regenerate from the ideas screen; observes the new generation. */
    fun refine(refinement: String) {
        val requestId = _state.value.requestId ?: return
        _state.update { it.copy(phase = GenerationPhase.SUBMITTING, error = null) }
        viewModelScope.launch {
            when (val accepted = repository.refine(requestId, refinement)) {
                is ApiResult.Success -> observe(accepted.data.requestId)
                is ApiResult.Failure -> _state.update {
                    it.copy(phase = GenerationPhase.FAILED, error = accepted.error.message)
                }
            }
        }
    }

    fun saveIdea(ideaId: String) {
        viewModelScope.launch {
            if (repository.saveIdea(ideaId) is ApiResult.Success) {
                _state.update { it.copy(savedIdeaIds = it.savedIdeaIds + ideaId) }
            }
        }
    }

    /** From the failure screen: retry the whole submit. */
    fun retry() = submit()

    /** Reset back to the input form for a brand-new occasion. */
    fun startOver() {
        observeJob?.cancel()
        _state.update {
            it.copy(
                phase = GenerationPhase.INPUT,
                requestId = null,
                progress = 0,
                statusLabel = "",
                ideas = emptyList(),
                error = null,
            )
        }
    }
}
