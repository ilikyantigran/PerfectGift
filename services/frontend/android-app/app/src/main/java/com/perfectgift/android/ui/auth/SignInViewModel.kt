package com.perfectgift.android.ui.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.dto.AuthProvider
import com.perfectgift.android.data.repository.PerfectGiftRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SignInUiState(
    val email: String = "",
    val password: String = "",
    val isSubmitting: Boolean = false,
    val error: String? = null,
    val signedIn: Boolean = false,
)

class SignInViewModel(
    private val repository: PerfectGiftRepository,
    private val session: SessionManager,
) : ViewModel() {

    private val _state = MutableStateFlow(SignInUiState(signedIn = session.isSignedIn))
    val state: StateFlow<SignInUiState> = _state.asStateFlow()

    fun onEmailChange(value: String) = _state.update { it.copy(email = value, error = null) }
    fun onPasswordChange(value: String) = _state.update { it.copy(password = value, error = null) }

    /** Email + password fallback sign-in. */
    fun signInWithEmail() {
        val s = _state.value
        if (s.email.isBlank() || s.password.isBlank()) {
            _state.update { it.copy(error = "Enter your email and password.") }
            return
        }
        submit { repository.signIn(AuthProvider.EMAIL, email = s.email.trim(), password = s.password) }
    }

    /** Google sign-in: the Compose layer obtains the Google ID token via Credential Manager. */
    fun signInWithGoogle(idToken: String) =
        submit { repository.signIn(AuthProvider.GOOGLE, idToken = idToken) }

    /** Sign in with Apple: pass the Apple-issued identity token. */
    fun signInWithApple(idToken: String) =
        submit { repository.signIn(AuthProvider.APPLE, idToken = idToken) }

    private fun submit(call: suspend () -> ApiResult<*>) {
        _state.update { it.copy(isSubmitting = true, error = null) }
        viewModelScope.launch {
            when (val result = call()) {
                is ApiResult.Success -> _state.update { it.copy(isSubmitting = false, signedIn = true) }
                is ApiResult.Failure -> _state.update {
                    it.copy(isSubmitting = false, error = result.error.message)
                }
            }
        }
    }
}
