package com.perfectgift.android.data.auth

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.runBlocking

/**
 * Single source of truth for the current session. Keeps the tokens in memory (so the
 * OkHttp interceptor/authenticator can read them synchronously on network threads) and
 * mirrors them to [TokenStore] (DataStore) for persistence across launches.
 */
class SessionManager(private val store: TokenStore) {

    private val _state = MutableStateFlow<AuthTokens?>(null)
    val state: StateFlow<AuthTokens?> = _state.asStateFlow()

    val isSignedIn: Boolean get() = _state.value != null

    /** Load any persisted tokens into memory. Call once at startup. */
    suspend fun bootstrap() {
        _state.value = store.current()
    }

    /** Synchronous read for interceptors running on OkHttp threads. */
    fun currentAccessToken(): String? = _state.value?.accessToken

    fun currentRefreshToken(): String? = _state.value?.refreshToken

    suspend fun onSignedIn(access: String, refresh: String) {
        val tokens = AuthTokens(access, refresh)
        _state.value = tokens
        store.save(tokens)
    }

    /** Used by the authenticator (network thread) after a successful refresh. */
    fun updateBlocking(access: String, refresh: String) {
        val tokens = AuthTokens(access, refresh)
        _state.value = tokens
        runBlocking { store.save(tokens) }
    }

    suspend fun signOut() {
        _state.value = null
        store.clear()
    }

    /** Sign-out from a network thread (e.g. refresh failed → force re-login). */
    fun signOutBlocking() {
        _state.value = null
        runBlocking { store.clear() }
    }
}
