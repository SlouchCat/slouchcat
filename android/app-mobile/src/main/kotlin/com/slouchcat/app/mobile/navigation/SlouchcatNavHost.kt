package com.slouchcat.app.mobile.navigation

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.SessionEventHandler
import com.slouchcat.app.mobile.profiles.ProfilesScreen
import com.slouchcat.app.mobile.profiles.ProfilesViewModel
import com.slouchcat.app.mobile.setup.SetupScreen
import com.slouchcat.app.mobile.setup.SetupViewModel
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.AppShell
import com.slouchcat.app.mobile.shell.Navigator
import com.slouchcat.app.mobile.shell.ProfilesRoute
import com.slouchcat.app.mobile.shell.SetupRoute
import com.slouchcat.app.mobile.shell.SignInRoute
import com.slouchcat.app.mobile.shell.frame
import com.slouchcat.app.mobile.shell.targetOf
import com.slouchcat.app.mobile.signin.SignInScreen
import com.slouchcat.app.mobile.signin.SignInViewModel
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.launch

/**
 * The navigation of the app: the entry pages (address, sign-in, profile choice) and the pages of
 * every feature in one NavHost, framed by [AppShell]. The first page follows the stored session.
 */
@Composable
fun SlouchcatNavHost(graph: AppGraph) {
    val state by graph.session.state.collectAsStateWithLifecycle()
    if (state == SessionState.Loading) {
        Box(Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg))
        return
    }
    val nav = rememberNavController()
    val start = remember { startOf(state, graph) }
    val navigator = remember(nav) { Navigator(nav, graph.registry) }
    val app = remember(navigator) { AppScope(graph, navigator) }
    val scope = rememberCoroutineScope()
    val profile = (state as? SessionState.Ready)?.profile

    SlouchcatTheme(accent = profile?.accent) {
        SessionEventHandler(
            graph.sessionEvents,
            onSignedOut = {
                scope.launch {
                    graph.sessionActions.sessionLost()
                    navigator.toSignIn()
                }
            },
            onConfirmKey = { navigator.toSignIn(confirmKey = true) },
        )
        val current by nav.currentBackStackEntryAsState()
        // The target a page counts to: the last target page shown (or tapped).
        var lastTarget by rememberSaveable { mutableStateOf<String?>(null) }
        val here = graph.registry.targetOf(current)?.key
        LaunchedEffect(here) { if (here != null) lastTarget = here }
        val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
        val media = (libraries as? Loadable.Ready)?.value.orEmpty().let(graph.registry::mediaEntries)
            .takeIf { profile != null }.orEmpty()
        AppShell(
            frame = graph.registry.frame(current, lastTarget, media),
            profile = profile,
            onTarget = {
                lastTarget = it.key
                navigator.openTop(it)
            },
            onMedia = { navigator.navigate(it.route) },
            onSwitchProfile = navigator::toProfiles,
        ) {
            NavHost(navController = nav, startDestination = start, modifier = Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
                setupDestination(graph, navigator)
                signInDestination(graph, navigator)
                profilesDestination(graph, navigator)
                for (feature in graph.registry.features) {
                    with(feature) { register(app) }
                }
            }
        }
    }
}

/** Where the app opens: the address, the sign-in, the profile choice or the first target. */
private fun startOf(state: SessionState, graph: AppGraph): Any = when (state) {
    SessionState.Loading, SessionState.NoServer -> SetupRoute
    is SessionState.SignedOut -> SignInRoute()
    is SessionState.ChooseProfile -> ProfilesRoute
    is SessionState.Ready -> graph.registry.home.route
}

/** One destination per page: the ViewModel from [AppGraph], the state, the screen. */
private fun NavGraphBuilder.setupDestination(graph: AppGraph, navigator: Navigator) {
    composable<SetupRoute> {
        val vm = viewModel { SetupViewModel(createSavedStateHandle(), graph.serverProbe, graph.hostResolver, graph.serverStore) }
        val state by vm.state.collectAsStateWithLifecycle()
        LaunchedEffect(vm) { vm.done.collect { navigator.toSignIn() } }
        SetupScreen(
            state = state,
            onAddressChange = vm::onAddressChange,
            onConnect = vm::onConnect,
            onConfirmInsecure = vm::onConfirmInsecure,
            onDismissInsecure = vm::onDismissInsecure,
            onChangeAddress = vm::onChangeAddress,
            onNext = vm::onNext,
        )
    }
}

private fun NavGraphBuilder.signInDestination(graph: AppGraph, navigator: Navigator) {
    composable<SignInRoute> { entry ->
        val route = Navigator.signIn(entry)
        val vm = viewModel {
            SignInViewModel(
                saved = createSavedStateHandle(),
                confirmKey = route.confirmKey,
                session = graph.session,
                auth = graph.auth,
                probe = graph.serverProbe,
                servers = graph.serverStore,
                device = graph.device,
                keys = graph.keyConfirmation,
            )
        }
        val state by vm.state.collectAsStateWithLifecycle()
        LaunchedEffect(vm) { vm.done.collect { chosen -> if (chosen) navigator.toHome() else navigator.toProfilesFirst() } }
        SignInScreen(
            state = state,
            onEmailChange = vm::onEmailChange,
            onPasswordChange = vm::onPasswordChange,
            onTogglePassword = vm::onTogglePassword,
            onSubmit = vm::onSubmit,
            onChangeServer = navigator::toSetup,
        )
    }
}

private fun NavGraphBuilder.profilesDestination(graph: AppGraph, navigator: Navigator) {
    composable<ProfilesRoute> {
        val vm = viewModel { ProfilesViewModel(graph.session, graph.auth, graph.sessionActions) }
        val state by vm.state.collectAsStateWithLifecycle()
        LaunchedEffect(vm) { vm.done.collect { navigator.toHome() } }
        LaunchedEffect(vm) { vm.signedOut.collect { navigator.toSignIn() } }
        ProfilesScreen(
            state = state,
            onPick = vm::onPick,
            onDigit = vm::onDigit,
            onDelete = vm::onDelete,
            onSubmitPin = vm::onSubmitPin,
            onCancelPin = vm::onCancelPin,
            onRetry = vm::retry,
            onSignOut = vm::onSignOut,
        )
    }
}
