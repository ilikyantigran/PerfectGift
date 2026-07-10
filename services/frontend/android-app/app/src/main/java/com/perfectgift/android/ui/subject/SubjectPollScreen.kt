package com.perfectgift.android.ui.subject

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.perfectgift.android.ui.AppViewModelProvider
import com.perfectgift.android.ui.common.ErrorState
import com.perfectgift.android.ui.common.LoadingState

/**
 * The Subject flow, rendered natively on a handed-over phone or via an App Link. Fetches
 * the poll by its opaque link token (no auth), shows the questions, submits answers.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SubjectPollScreen(
    token: String,
    onDone: () -> Unit,
    viewModel: SubjectPollViewModel = viewModel(factory = AppViewModelProvider.Factory),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(token) { viewModel.load(token) }

    Scaffold(
        topBar = { TopAppBar(title = { Text(state.title.ifBlank { "A quick question" }) }) },
    ) { padding ->
        when {
            state.loading -> LoadingState("Loading…", Modifier.padding(padding))
            state.error != null && state.questions.isEmpty() ->
                ErrorState(state.error!!, modifier = Modifier.padding(padding))
            state.submitted -> Column(
                modifier = Modifier.fillMaxSize().padding(padding).padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center,
            ) {
                Text("Thanks! Your answers were sent.", textAlign = TextAlign.Center)
                Spacer(Modifier.height(16.dp))
                Button(onClick = onDone) { Text("Done") }
            }
            else -> Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .padding(16.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                state.questions.forEach { q ->
                    val id = q.id ?: return@forEach
                    Text(q.text.orEmpty(), style = MaterialTheme.typography.titleSmall)
                    OutlinedTextField(
                        value = state.answers[id].orEmpty(),
                        onValueChange = { viewModel.onAnswerChange(id, it) },
                        modifier = Modifier.fillMaxWidth().padding(top = 4.dp, bottom = 12.dp),
                    )
                }
                state.error?.let {
                    Text(it, color = MaterialTheme.colorScheme.error)
                }
                Spacer(Modifier.height(8.dp))
                Button(
                    onClick = viewModel::submit,
                    enabled = !state.isSubmitting,
                    modifier = Modifier.fillMaxWidth(),
                ) { Text(if (state.isSubmitting) "Sending…" else "Submit") }
            }
        }
    }
}
