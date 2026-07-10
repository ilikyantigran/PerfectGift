package com.perfectgift.android.ui.generation

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle

/**
 * The submit-then-observe progress screen. It never freezes: generation runs async on
 * the backend (~3–15 s) and this screen renders live status/progress while polling. On
 * "ready" it hands off to the ideas screen; on failure it offers a graceful retry.
 */
@Composable
fun GeneratingScreen(
    viewModel: GenerationViewModel,
    onIdeasReady: () -> Unit,
    onBackToInput: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(state.phase) {
        if (state.phase == GenerationPhase.IDEAS) onIdeasReady()
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        if (state.phase == GenerationPhase.FAILED) {
            Text(
                state.error ?: "Something went wrong.",
                color = MaterialTheme.colorScheme.error,
                textAlign = TextAlign.Center,
            )
            Spacer(Modifier.height(16.dp))
            Button(onClick = viewModel::retry) { Text("Try again") }
            Spacer(Modifier.height(8.dp))
            OutlinedButton(onClick = onBackToInput) { Text("Edit the occasion") }
        } else {
            CircularProgressIndicator()
            Spacer(Modifier.height(24.dp))
            Text("Dreaming up ideas…", style = MaterialTheme.typography.titleMedium)
            Text(
                "This usually takes a few seconds.",
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(top = 4.dp),
            )
            Spacer(Modifier.height(24.dp))
            if (state.progress in 1..99) {
                @Suppress("DEPRECATION")
                LinearProgressIndicator(
                    progress = state.progress / 100f,
                    modifier = Modifier.fillMaxWidth().height(6.dp),
                )
            } else {
                LinearProgressIndicator(modifier = Modifier.fillMaxWidth().height(6.dp))
            }
            Text(
                state.statusLabel.replaceFirstChar { it.uppercase() },
                style = MaterialTheme.typography.labelMedium,
                modifier = Modifier.padding(top = 8.dp),
            )
        }
    }
}
