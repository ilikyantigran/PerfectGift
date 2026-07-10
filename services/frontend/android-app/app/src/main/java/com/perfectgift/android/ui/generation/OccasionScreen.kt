package com.perfectgift.android.ui.generation

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun OccasionScreen(
    viewModel: GenerationViewModel,
    onStartGenerating: () -> Unit,
    onCreatePoll: () -> Unit,
    onSignOut: () -> Unit,
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) { viewModel.loadReferenceData() }
    // Once a submit kicks off, move to the generating screen.
    LaunchedEffect(state.phase) {
        if (state.phase != GenerationPhase.INPUT) onStartGenerating()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Describe the occasion") },
                actions = { TextButton(onClick = onSignOut) { Text("Sign out") } },
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
            val selectedHoliday = state.holidays.firstOrNull { it.id == state.selectedHolidayId }
            SelectField(
                label = "Holiday / occasion",
                selectedLabel = selectedHoliday?.name ?: "Any occasion",
                options = state.holidays,
                optionLabel = { it.name ?: it.id.orEmpty() },
                onSelect = { viewModel.onHolidaySelected(it?.id) },
                allowClear = true,
            )

            Spacer(Modifier.height(12.dp))

            val selectedBand = state.budgetBands.firstOrNull { it.id == state.budgetBand }
            SelectField(
                label = "Budget",
                selectedLabel = selectedBand?.label ?: "No budget set",
                options = state.budgetBands,
                optionLabel = { it.label ?: it.id.orEmpty() },
                onSelect = { viewModel.onBudgetSelected(it?.id) },
                allowClear = true,
            )

            Spacer(Modifier.height(12.dp))

            OutlinedTextField(
                value = state.preferencesText,
                onValueChange = viewModel::onPreferencesChange,
                label = { Text("Their preferences (free-form)") },
                placeholder = { Text("Loves hiking, into indie music, hates clutter…") },
                modifier = Modifier.fillMaxWidth().height(140.dp),
            )

            state.error?.let {
                Text(
                    it,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(top = 8.dp),
                )
            }

            Spacer(Modifier.height(20.dp))
            Button(onClick = viewModel::submit, modifier = Modifier.fillMaxWidth()) {
                Text("Generate ideas")
            }
            Spacer(Modifier.height(8.dp))
            OutlinedButton(onClick = onCreatePoll, modifier = Modifier.fillMaxWidth()) {
                Text("Ask them first (create a poll)")
            }
        }
    }
}

/** Minimal stable-API dropdown select (avoids the experimental exposed dropdown box). */
@Composable
private fun <T> SelectField(
    label: String,
    selectedLabel: String,
    options: List<T>,
    optionLabel: (T) -> String,
    onSelect: (T?) -> Unit,
    allowClear: Boolean,
) {
    var expanded by remember { mutableStateOf(false) }
    Column {
        Text(label, style = MaterialTheme.typography.labelMedium)
        Box {
            OutlinedButton(onClick = { expanded = true }, modifier = Modifier.fillMaxWidth()) {
                Text(selectedLabel)
            }
            DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
                if (allowClear) {
                    DropdownMenuItem(
                        text = { Text("— none —") },
                        onClick = { onSelect(null); expanded = false },
                    )
                }
                options.forEach { option ->
                    DropdownMenuItem(
                        text = { Text(optionLabel(option)) },
                        onClick = { onSelect(option); expanded = false },
                    )
                }
            }
        }
    }
}
