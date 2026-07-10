package com.perfectgift.android.ui.nav

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.navigation
import androidx.navigation.navArgument
import com.perfectgift.android.ui.AppViewModelProvider
import com.perfectgift.android.ui.auth.SignInScreen
import com.perfectgift.android.ui.generation.GeneratingScreen
import com.perfectgift.android.ui.generation.GenerationViewModel
import com.perfectgift.android.ui.generation.IdeasScreen
import com.perfectgift.android.ui.generation.OccasionScreen
import com.perfectgift.android.ui.poll.PollCreateScreen
import com.perfectgift.android.ui.subject.SubjectPollScreen

/**
 * The whole app's navigation. The occasion → generating → ideas screens live in a nested
 * graph and share one [GenerationViewModel] (scoped to the graph's back-stack entry) so
 * the submit-then-observe state survives the hops between them.
 */
@Composable
fun PerfectGiftNavHost(
    navController: NavHostController,
    startSignedIn: Boolean,
    onSignOut: () -> Unit,
) {
    NavHost(
        navController = navController,
        startDestination = if (startSignedIn) Routes.GENERATION_GRAPH else Routes.SIGN_IN,
    ) {
        composable(Routes.SIGN_IN) {
            SignInScreen(
                onSignedIn = {
                    navController.navigate(Routes.GENERATION_GRAPH) {
                        popUpTo(Routes.SIGN_IN) { inclusive = true }
                    }
                },
            )
        }

        navigation(startDestination = Routes.OCCASION, route = Routes.GENERATION_GRAPH) {
            composable(Routes.OCCASION) { entry ->
                OccasionScreen(
                    viewModel = entry.sharedGenerationVm(navController),
                    onStartGenerating = { navController.navigate(Routes.GENERATING) },
                    onCreatePoll = { navController.navigate(Routes.POLL_CREATE) },
                    onSignOut = {
                        onSignOut()
                        navController.navigate(Routes.SIGN_IN) {
                            popUpTo(Routes.GENERATION_GRAPH) { inclusive = true }
                        }
                    },
                )
            }

            composable(Routes.GENERATING) { entry ->
                GeneratingScreen(
                    viewModel = entry.sharedGenerationVm(navController),
                    onIdeasReady = {
                        navController.navigate(Routes.IDEAS) {
                            popUpTo(Routes.GENERATING) { inclusive = true }
                        }
                    },
                    onBackToInput = {
                        navController.navigate(Routes.OCCASION) {
                            popUpTo(Routes.OCCASION) { inclusive = true }
                        }
                    },
                )
            }

            composable(Routes.IDEAS) { entry ->
                val vm = entry.sharedGenerationVm(navController)
                IdeasScreen(
                    viewModel = vm,
                    onStartOver = {
                        vm.startOver()
                        navController.navigate(Routes.OCCASION) {
                            popUpTo(Routes.GENERATION_GRAPH) { inclusive = false }
                            launchSingleTop = true
                        }
                    },
                    onRefineStarted = {
                        navController.navigate(Routes.GENERATING) {
                            popUpTo(Routes.IDEAS) { inclusive = true }
                        }
                    },
                )
            }
        }

        composable(Routes.POLL_CREATE) {
            PollCreateScreen(
                onBack = { navController.popBackStack() },
                onOpenAsSubject = { token -> navController.navigate(Routes.subject(token)) },
            )
        }

        composable(
            route = Routes.SUBJECT_ROUTE,
            arguments = listOf(navArgument(Routes.SUBJECT_ARG_TOKEN) { type = NavType.StringType }),
        ) { backStackEntry ->
            val token = backStackEntry.arguments?.getString(Routes.SUBJECT_ARG_TOKEN).orEmpty()
            SubjectPollScreen(
                token = token,
                onDone = {
                    if (!navController.popBackStack()) {
                        navController.navigate(Routes.GENERATION_GRAPH)
                    }
                },
            )
        }
    }
}

/** Obtain the [GenerationViewModel] scoped to the generation nested graph. */
@Composable
private fun NavBackStackEntry.sharedGenerationVm(navController: NavHostController): GenerationViewModel {
    val parentEntry = remember(this) { navController.getBackStackEntry(Routes.GENERATION_GRAPH) }
    return viewModel(viewModelStoreOwner = parentEntry, factory = AppViewModelProvider.Factory)
}
