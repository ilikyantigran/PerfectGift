package com.perfectgift.android.ui.poll

import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.perfectgift.android.ui.AppViewModelProvider

/**
 * Owner poll flow: compose questions → create → get a shareable link (returned once).
 * Share the link with the Subject, or hand over the phone to open the native Subject
 * form directly. Also lets the owner pull the Subject's responses.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PollCreateScreen(
    onBack: () -> Unit,
    onOpenAsSubject: (token: String) -> Unit,
    viewModel: PollViewModel = viewModel(factory = AppViewModelProvider.Factory),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Ask them first") },
                actions = { TextButton(onClick = onBack) { Text("Done") } },
            )
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp)
                .verticalScroll(rememberScrollState()),
        ) {
            val created = state.created
            if (created == null) {
                OutlinedTextField(
                    value = state.title,
                    onValueChange = viewModel::onTitleChange,
                    label = { Text("Poll title (optional)") },
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(12.dp))
                Text("Questions", style = MaterialTheme.typography.titleSmall)

                state.questions.forEach { q ->
                    Row(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
                        OutlinedTextField(
                            value = q.text,
                            onValueChange = { viewModel.onQuestionChange(q.localId, it) },
                            label = { Text("Question") },
                            modifier = Modifier.weight(1f),
                        )
                        TextButton(onClick = { viewModel.removeQuestion(q.localId) }) { Text("✕") }
                    }
                }
                TextButton(onClick = viewModel::addQuestion) { Text("+ Add question") }

                state.error?.let {
                    Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(top = 8.dp))
                }

                Spacer(Modifier.height(16.dp))
                Button(
                    onClick = { viewModel.createPoll() },
                    enabled = !state.isSubmitting,
                    modifier = Modifier.fillMaxWidth(),
                ) { Text(if (state.isSubmitting) "Creating…" else "Create poll & get link") }
            } else {
                Card(Modifier.fillMaxWidth()) {
                    Column(Modifier.padding(16.dp)) {
                        Text("Share this link", style = MaterialTheme.typography.titleMedium)
                        Text(
                            created.linkUrl ?: "(no link returned)",
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(vertical = 8.dp),
                        )
                        created.expiresAt?.let {
                            Text("Expires: $it", style = MaterialTheme.typography.labelSmall)
                        }
                        Spacer(Modifier.height(12.dp))
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Button(
                                onClick = {
                                    val link = created.linkUrl ?: return@Button
                                    val send = Intent(Intent.ACTION_SEND).apply {
                                        type = "text/plain"
                                        putExtra(Intent.EXTRA_TEXT, link)
                                    }
                                    context.startActivity(Intent.createChooser(send, "Share poll"))
                                },
                            ) { Text("Share link") }

                            created.linkToken?.let { token ->
                                OutlinedButton(onClick = { onOpenAsSubject(token) }) {
                                    Text("Hand over phone")
                                }
                            }
                        }
                    }
                }

                Spacer(Modifier.height(16.dp))
                OutlinedButton(
                    onClick = viewModel::refreshResponses,
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Check for responses") }

                state.responses.forEach { resp ->
                    Card(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                        Column(Modifier.padding(12.dp)) {
                            Text("Response", style = MaterialTheme.typography.labelMedium)
                            resp.answers.forEach { a ->
                                Text("• ${a.value ?: a.values?.joinToString() ?: ""}")
                            }
                        }
                    }
                }
            }
        }
    }
}
