package com.perfectgift.android.ui.subject

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.dto.AnswerDto
import com.perfectgift.android.data.remote.dto.QuestionDto
import com.perfectgift.android.data.repository.PerfectGiftRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SubjectPollUiState(
    val loading: Boolean = true,
    val title: String = "",
    val questions: List<QuestionDto> = emptyList(),
    // questionId -> answer value.
    val answers: Map<String, String> = emptyMap(),
    val isSubmitting: Boolean = false,
    val submitted: Boolean = false,
    val error: String? = null,
)

/**
 * The handed-over-phone / App-Link Subject flow: fetch a poll by its opaque link token
 * (no auth), render questions natively, submit answers. Uses the anonymous token routes.
 */
class SubjectPollViewModel(
    private val repository: PerfectGiftRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(SubjectPollUiState())
    val state: StateFlow<SubjectPollUiState> = _state.asStateFlow()

    private var token: String = ""

    fun load(linkToken: String) {
        token = linkToken
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            when (val result = repository.getPollByToken(linkToken)) {
                is ApiResult.Success -> _state.update {
                    it.copy(
                        loading = false,
                        title = result.data.title.orEmpty(),
                        questions = result.data.questions,
                    )
                }
                is ApiResult.Failure -> _state.update {
                    it.copy(loading = false, error = result.error.message)
                }
            }
        }
    }

    fun onAnswerChange(questionId: String, value: String) = _state.update {
        it.copy(answers = it.answers + (questionId to value))
    }

    fun submit() {
        val s = _state.value
        val answers = s.questions.mapNotNull { q ->
            val id = q.id ?: return@mapNotNull null
            val value = s.answers[id]?.takeIf { it.isNotBlank() } ?: return@mapNotNull null
            AnswerDto(questionId = id, value = value)
        }
        if (answers.isEmpty()) {
            _state.update { it.copy(error = "Please answer at least one question.") }
            return
        }
        _state.update { it.copy(isSubmitting = true, error = null) }
        viewModelScope.launch {
            when (val result = repository.submitPollResponse(token, answers)) {
                is ApiResult.Success -> _state.update { it.copy(isSubmitting = false, submitted = true) }
                is ApiResult.Failure -> _state.update { it.copy(isSubmitting = false, error = result.error.message) }
            }
        }
    }
}
