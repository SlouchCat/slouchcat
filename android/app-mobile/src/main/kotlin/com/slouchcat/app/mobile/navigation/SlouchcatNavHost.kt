package com.slouchcat.app.mobile.navigation

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalResources
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
import com.slouchcat.app.mobile.shell.ShellBadge
import com.slouchcat.app.mobile.shell.SignInRoute
import com.slouchcat.app.mobile.shell.frame
import com.slouchcat.app.mobile.shell.targetOf
import com.slouchcat.app.mobile.signin.SignInScreen
import com.slouchcat.app.mobile.signin.SignInViewModel
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch

/**
 * The navigation of the app: the entry pages (address, sign-in, profile choice) and the pages of
 * every feature in one NavHost, framed by [AppShell].
 *
 * The NavHost and its controller belong to one session epoch (sign-in, profile choice, sign-out
 * each start one; code review B-1): with a new epoch every target's saved pages and their
 * ViewModels go, and the first page follows the session again. Within an epoch the NavHost stays
 * where it is (B-2). An in-app jump ([AppGraph.links]) waits until a profile is chosen.
 */
@Composable
fun SlouchcatNavHost(graph: AppGraph) {
    val state by graph.session.state.collectAsStateWithLifecycle()
    val epoch by graph.session.epoch.collectAsStateWithLifecycle()
    if (state == SessionState.Loading) {
        Box(Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg))
        return
    }
    val profile = (state as? SessionState.Ready)?.profile
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    val resources = LocalResources.current
    val notice by graph.notices.pending.collectAsStateWithLifecycle()
    LaunchedEffect(notice) {
        val id = notice ?: return@LaunchedEffect
        val text = resources.getString(id)
        // Outside this effect: consuming the notice restarts it, which would close the snackbar at once.
        scope.launch { snackbar.showSnackbar(text) }
        graph.notices.consume()
    }

    SlouchcatTheme(accent = profile?.accent) {
        SessionEventHandler(
            graph.sessionEvents,
            // A 401 of a request of an earlier epoch (another account, another profile) changes nothing.
            onSignedOut = { at -> if (at == graph.session.epoch.value) scope.launch { graph.sessionActions.sessionLost() } },
        )
        key(epoch) {
            val nav = rememberNavController()
            val start = remember { startOf(state, graph) }
            val selected = rememberSaveable { mutableStateOf<String?>(null) }
            val navigator = remember(nav) { Navigator(nav, graph.registry, selected) }
            val app = remember(navigator) { AppScope(graph, navigator) }
            val current by nav.currentBackStackEntryAsState()
            val here = graph.registry.targetOf(current)?.key
            LaunchedEffect(here) { if (here != null) selected.value = here }
            // An in-app jump waits for a chosen profile, then builds its stack.
            val link by graph.links.pending.collectAsStateWithLifecycle()
            LaunchedEffect(link, state) {
                val waiting = link ?: return@LaunchedEffect
                if (state is SessionState.Ready && current != null) {
                    graph.links.consume()
                    navigator.open(waiting)
                }
            }
            val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
            val media = if (profile == null) emptyList() else (libraries as? Loadable.Ready)?.value.orEmpty().let(graph.registry::mediaEntries)
            val badges = buildMap<String, ShellBadge> {
                for (destination in graph.registry.destinations) {
                    val flow = remember(destination.key) { destination.badge?.invoke(graph) ?: flowOf(null) }
                    val badge by flow.collectAsStateWithLifecycle(null)
                    badge?.let { put(destination.key, it) }
                }
            }
            AppShell(
                frame = graph.registry.frame(current, selected.value, media).copy(badges = if (profile == null) emptyMap() else badges),
                profile = profile,
                onTarget = navigator::openTop,
                onMedia = navigator::openMedia,
                onSwitchProfile = navigator::toProfiles,
                snackbar = snackbar,
                mediaCount = { entry ->
                    val live = remember(entry.key) { entry.liveCount?.invoke(graph) ?: flowOf(entry.count) }
                    val count by live.collectAsStateWithLifecycle(entry.count)
                    count
                },
                accessory = graph.registry.accessory?.let { accessory -> { if (profile != null) accessory(app) } },
            ) {
                NavHost(navController = nav, startDestination = start, modifier = Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
                    setupDestination(graph, navigator)
                    signInDestination(graph, navigator)
                    profilesDestination(graph)
                    for (feature in graph.registry.features) {
                        with(feature) { register(app) }
                    }
                }
            }
        }
    }
}

/** Where an epoch opens: the address, the sign-in, the profile choice or the first target. */
private fun startOf(state: SessionState, graph: AppGraph): Any = when (state) {
    SessionState.Loading, SessionState.NoServer -> SetupRoute
    // «Server wechseln» signs out and asks for the address again (mockup `ACT.suLogout`).
    is SessionState.SignedOut -> if (graph.entry.takeSetupNext()) SetupRoute else SignInRoute()
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
            onOwnNetwork = vm::onOwnNetwork,
            onAcceptKey = vm::onAcceptKey,
            onDismissKey = vm::onDismissKey,
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
                device = graph.device,
                keys = graph.keyConfirmation,
            )
        }
        val state by vm.state.collectAsStateWithLifecycle()
        // The page after a sign-in follows from the new session epoch (SlouchcatNavHost).
        SignInScreen(
            state = state,
            onEmailChange = vm::onEmailChange,
            onPasswordChange = vm::onPasswordChange,
            onTogglePassword = vm::onTogglePassword,
            onSubmit = vm::onSubmit,
            onChangeServer = navigator::toSetup,
            onCheckKey = vm::onCheckKey,
            onOwnNetwork = vm::onOwnNetwork,
            onConfirmKey = vm::onConfirmKey,
            onDismissKey = vm::onDismissKey,
        )
    }
}

private fun NavGraphBuilder.profilesDestination(graph: AppGraph) {
    composable<ProfilesRoute> {
        val vm = viewModel { ProfilesViewModel(graph.session, graph.auth, graph.sessionActions) }
        val state by vm.state.collectAsStateWithLifecycle()
        // A chosen profile and a sign-out start a new epoch: SlouchcatNavHost opens the next page.
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
