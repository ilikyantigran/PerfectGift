package com.perfectgift.android.ui

import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.CreationExtras
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import com.perfectgift.android.PerfectGiftApp
import com.perfectgift.android.ui.auth.SignInViewModel
import com.perfectgift.android.ui.generation.GenerationViewModel
import com.perfectgift.android.ui.poll.PollViewModel
import com.perfectgift.android.ui.subject.SubjectPollViewModel

/** Bridges Compose's `viewModel()` to the hand-rolled [com.perfectgift.android.di.AppContainer]. */
object AppViewModelProvider {

    val Factory: ViewModelProvider.Factory = viewModelFactory {
        initializer { SignInViewModel(app().container.repository, app().container.session) }
        initializer { GenerationViewModel(app().container.repository) }
        initializer { PollViewModel(app().container.repository) }
        initializer { SubjectPollViewModel(app().container.repository) }
    }

    private fun CreationExtras.app(): PerfectGiftApp =
        (this[APPLICATION_KEY] as PerfectGiftApp)
}
