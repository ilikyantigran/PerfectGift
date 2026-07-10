package com.perfectgift.android.util

import com.perfectgift.android.data.auth.AuthTokens
import com.perfectgift.android.data.auth.TokenStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.first

/** In-memory [TokenStore] for tests (no Android DataStore). */
class FakeTokenStore(initial: AuthTokens? = null) : TokenStore {
    private val flow = MutableStateFlow(initial)
    override val tokens = flow
    override suspend fun current(): AuthTokens? = flow.first()
    override suspend fun save(tokens: AuthTokens) { flow.value = tokens }
    override suspend fun clear() { flow.value = null }
}
