package com.perfectgift.android.ui.generation

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.perfectgift.android.data.remote.dto.IdeaDto

/** Ranked ideas list with save/favorite and a refine box that regenerates. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun IdeasScreen(
    viewModel: GenerationViewModel,
    onStartOver: () -> Unit,
    onRefineStarted: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var refineText by remember { mutableStateOf("") }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Surprise ideas") },
                actions = { TextButton(onClick = onStartOver) { Text("New") } },
            )
        },
    ) { padding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding).padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            items(state.ideas, key = { it.id ?: it.title.orEmpty() }) { idea ->
                IdeaCard(
                    idea = idea,
                    saved = idea.id != null && idea.id in state.savedIdeaIds,
                    onSave = { idea.id?.let(viewModel::saveIdea) },
                )
            }

            item {
                Column(Modifier.padding(vertical = 12.dp)) {
                    Text("Not quite right?", style = MaterialTheme.typography.titleSmall)
                    OutlinedTextField(
                        value = refineText,
                        onValueChange = { refineText = it },
                        label = { Text("Refine (e.g. more budget-friendly)") },
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                    )
                    Spacer(Modifier.height(8.dp))
                    TextButton(
                        onClick = {
                            if (refineText.isNotBlank()) {
                                viewModel.refine(refineText.trim())
                                refineText = ""
                                onRefineStarted()
                            }
                        },
                    ) { Text("Regenerate") }
                }
            }
        }
    }
}

@Composable
private fun IdeaCard(idea: IdeaDto, saved: Boolean, onSave: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("#${idea.rank}", style = MaterialTheme.typography.labelLarge)
                Spacer(Modifier.width(8.dp))
                Text(
                    idea.title.orEmpty(),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.weight(1f),
                )
                TextButton(onClick = onSave, enabled = !saved) {
                    Text(if (saved) "Saved" else "Save")
                }
            }
            idea.whyItFits?.let {
                Text(it, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.padding(top = 4.dp))
            }
            idea.roughCost?.let {
                Text("Approx. $it", style = MaterialTheme.typography.labelMedium, modifier = Modifier.padding(top = 8.dp))
            }
            idea.howTo?.let {
                Text(it, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(top = 4.dp))
            }
        }
    }
}
