package com.perfectgift.android.data.auth

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

/** The only thing persisted off-memory: the auth tokens. Backed by DataStore. */
data class AuthTokens(
    val accessToken: String,
    val refreshToken: String,
)

interface TokenStore {
    val tokens: Flow<AuthTokens?>
    suspend fun current(): AuthTokens?
    suspend fun save(tokens: AuthTokens)
    suspend fun clear()
}

private val Context.authDataStore: DataStore<Preferences> by preferencesDataStore(name = "auth")

class DataStoreTokenStore(private val context: Context) : TokenStore {

    private object Keys {
        val ACCESS = stringPreferencesKey("access_token")
        val REFRESH = stringPreferencesKey("refresh_token")
    }

    override val tokens: Flow<AuthTokens?> = context.authDataStore.data.map { prefs ->
        val access = prefs[Keys.ACCESS]
        val refresh = prefs[Keys.REFRESH]
        if (!access.isNullOrEmpty() && !refresh.isNullOrEmpty()) {
            AuthTokens(access, refresh)
        } else {
            null
        }
    }

    override suspend fun current(): AuthTokens? = tokens.first()

    override suspend fun save(tokens: AuthTokens) {
        context.authDataStore.edit { prefs ->
            prefs[Keys.ACCESS] = tokens.accessToken
            prefs[Keys.REFRESH] = tokens.refreshToken
        }
    }

    override suspend fun clear() {
        context.authDataStore.edit { it.clear() }
    }
}
