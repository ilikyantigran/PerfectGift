package com.perfectgift.android.ui.poll

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.dto.CreatePollResponse
import com.perfectgift.android.data.remote.dto.PollResponseDto
import com.perfectgift.android.data.remote.dto.QuestionDto
import com.perfectgift.android.data.repository.PerfectGiftRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.UUID

/** A single free-text question row while composing a poll. */
data class DraftQuestion(
    val localId: String = UUID.randomUUID().toString(),
    val text: String = "",
)

data class PollUiState(
    val title: String = "",
    val questions: List<DraftQuestion> = listOf(DraftQuestion()),
    val isSubmitting: Boolean = false,
    val created: CreatePollResponse? = null,
    val responses: List<PollResponseDto> = emptyList(),
    val error: String? = null,
)

/** Owner-side poll creation, share link, and reading the Subject's responses. */
class PollViewModel(
    private val repository: PerfectGiftRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(PollUiState())
    val state: StateFlow<PollUiState> = _state.asStateFlow()

    fun onTitleChange(value: String) = _state.update { it.copy(title = value) }

    fun onQuestionChange(localId: String, text: String) = _state.update { s ->
        s.copy(questions = s.questions.map { if (it.localId == localId) it.copy(text = text) else it })
    }

    fun addQuestion() = _state.update { it.copy(questions = it.questions + DraftQuestion()) }

    fun removeQuestion(localId: String) = _state.update { s ->
        s.copy(questions = s.questions.filterNot { it.localId == localId }.ifEmpty { listOf(DraftQuestion()) })
    }

    fun createPoll(surpriseRequestId: String? = null) {
        val s = _state.value
        val questions = s.questions
            .filter { it.text.isNotBlank() }
            .map { QuestionDto(text = it.text.trim(), kind = "text") }
        if (questions.isEmpty()) {
            _state.update { it.copy(error = "Add at least one question.") }
            return
        }
        _state.update { it.copy(isSubmitting = true, error = null) }
        viewModelScope.launch {
            when (val result = repository.createPoll(s.title.trim().ifBlank { "A quick question" }, questions, surpriseRequestId)) {
                is ApiResult.Success -> _state.update { it.copy(isSubmitting = false, created = result.data) }
                is ApiResult.Failure -> _state.update { it.copy(isSubmitting = false, error = result.error.message) }
            }
        }
    }

    /** Owner polls for the Subject's answers (e.g. after a "poll done" push). */
    fun refreshResponses() {
        val pollId = _state.value.created?.pollId ?: return
        viewModelScope.launch {
            when (val result = repository.getPollResponses(pollId)) {
                is ApiResult.Success -> _state.update { it.copy(responses = result.data) }
                is ApiResult.Failure -> _state.update { it.copy(error = result.error.message) }
            }
        }
    }
}
