package com.perfectgift.android.ui.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Divider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.perfectgift.android.ui.AppViewModelProvider

/**
 * Sign-in screen. Email + password fallback is wired end-to-end. The Google / Apple
 * buttons are placeholders for the native token flows (Credential Manager for Google,
 * the Apple web flow for Apple) — once a provider ID token is obtained, it is handed to
 * [SignInViewModel.signInWithGoogle] / [SignInViewModel.signInWithApple].
 */
@Composable
fun SignInScreen(
    onSignedIn: () -> Unit,
    viewModel: SignInViewModel = viewModel(factory = AppViewModelProvider.Factory),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(state.signedIn) {
        if (state.signedIn) onSignedIn()
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("PerfectGift", style = MaterialTheme.typography.headlineMedium)
        Text(
            "Plan the perfect surprise.",
            style = MaterialTheme.typography.bodyMedium,
            modifier = Modifier.padding(top = 4.dp, bottom = 32.dp),
        )

        OutlinedButton(
            onClick = { /* TODO: launch Credential Manager → viewModel.signInWithGoogle(idToken) */ },
            modifier = Modifier.fillMaxWidth(),
            enabled = !state.isSubmitting,
        ) { Text("Continue with Google") }

        Spacer(Modifier.height(8.dp))

        OutlinedButton(
            onClick = { /* TODO: launch Apple sign-in → viewModel.signInWithApple(idToken) */ },
            modifier = Modifier.fillMaxWidth(),
            enabled = !state.isSubmitting,
        ) { Text("Continue with Apple") }

        Divider(Modifier.padding(vertical = 24.dp))

        OutlinedTextField(
            value = state.email,
            onValueChange = viewModel::onEmailChange,
            label = { Text("Email") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(
            value = state.password,
            onValueChange = viewModel::onPasswordChange,
            label = { Text("Password") },
            singleLine = true,
            visualTransformation = PasswordVisualTransformation(),
            modifier = Modifier.fillMaxWidth(),
        )

        state.error?.let {
            Text(
                it,
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.padding(top = 8.dp),
            )
        }

        Spacer(Modifier.height(16.dp))
        Button(
            onClick = viewModel::signInWithEmail,
            enabled = !state.isSubmitting,
            modifier = Modifier.fillMaxWidth(),
        ) {
            if (state.isSubmitting) {
                CircularProgressIndicator(modifier = Modifier.height(20.dp))
            } else {
                Text("Sign in")
            }
        }
    }
}
