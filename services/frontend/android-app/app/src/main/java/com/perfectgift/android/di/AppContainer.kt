package com.perfectgift.android.di

import android.content.Context
import com.perfectgift.android.BuildConfig
import com.perfectgift.android.data.auth.DataStoreTokenStore
import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.NetworkModule
import com.perfectgift.android.data.repository.PerfectGiftRepository
import com.perfectgift.android.data.repository.PerfectGiftRepositoryImpl

/**
 * Hand-rolled dependency container (a full DI framework isn't warranted for a client
 * this size). Constructed once in [com.perfectgift.android.PerfectGiftApp] and reached
 * from ViewModels via a factory.
 */
class AppContainer(context: Context) {

    private val appContext = context.applicationContext

    val session: SessionManager = SessionManager(DataStoreTokenStore(appContext))

    private val gson = NetworkModule.gson()

    private val api = NetworkModule.create(
        baseUrl = BuildConfig.GATEWAY_BASE_URL,
        session = session,
        debug = BuildConfig.DEBUG,
    )

    val repository: PerfectGiftRepository = PerfectGiftRepositoryImpl(api, session, gson)
}
