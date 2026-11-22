package com.slouchcat.app.tv.navigation

import android.os.SystemClock
import androidx.activity.compose.BackHandler
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.LifecycleStartEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.profiles.ProfilesScreen
import com.slouchcat.app.tv.profiles.ProfilesViewModel
import com.slouchcat.app.tv.setup.HandSetupScreen
import com.slouchcat.app.tv.setup.HandSetupViewModel
import com.slouchcat.app.tv.setup.PairScreen
import com.slouchcat.app.tv.setup.PairViewModel
import com.slouchcat.app.tv.setup.RelayMenu
import com.slouchcat.app.tv.setup.SetupScreen
import com.slouchcat.app.tv.shell.ENTRY_ROUTES
import com.slouchcat.app.tv.shell.HandSetupRoute
import com.slouchcat.app.tv.shell.LIBRARIES_MENU
import com.slouchcat.app.tv.shell.LibrariesMenu
import com.slouchcat.app.tv.shell.librariesMenuEntry
import com.slouchcat.app.tv.shell.railLibraryCapacity
import com.slouchcat.app.tv.shell.PairRoute
import com.slouchcat.app.tv.shell.ProfilesRoute
import com.slouchcat.app.tv.shell.SetupRoute
import com.slouchcat.app.tv.shell.TvNavigator
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.shell.TvShell
import com.slouchcat.app.tv.shell.entryOf
import com.slouchcat.app.tv.shell.isRoute
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvScreenHost
import com.slouchcat.app.tv.ui.TvTheme
import com.slouchcat.app.tv.ui.TvToastHost
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.SessionEvent
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/**
 * The navigation of the TV app: the entry pages (setup, profile choice) and the pages of every
 * feature in one NavHost, framed by [TvShell]. The NavHost belongs to one session epoch (sign-in,
 * profile choice, sign-out each start one, like the phone app): with a new epoch every page and its
 * ViewModel go, and the first page follows the session again.
 *
 * Back (concept 26 3.4, BAUKASTEN «Fokus-Engine»): menus close first (their own handlers), then the
 * page (the NavHost); on the first page of an epoch the focus goes into the rail first (K26 stage 0,
 * point 6), then «Zum Verlassen nochmals Zurück drücken», and a second Back within 3 s calls [onExit].
 */
@Composable
fun TvNavHost(graph: TvGraph, onExit: () -> Unit, clock: () -> Long = SystemClock::elapsedRealtime) {
    val state by graph.session.state.collectAsStateWithLifecycle()
    val epoch by graph.session.epoch.collectAsStateWithLifecycle()
    val profile = (state as? SessionState.Ready)?.profile
    TvTheme(accent = profile?.accent) {
        if (state == SessionState.Loading) {
            Box(Modifier.fillMaxSize().background(SlouchcatTheme.colors.bg))
            return@TvTheme
        }
        val scope = rememberCoroutineScope()
        var tooOld by rememberSaveable { mutableStateOf(false) }
        LaunchedEffect(graph.sessionEvents) {
            graph.sessionEvents.events.collect { reported ->
                when (reported.event) {
                    SessionEvent.AppTooOld -> tooOld = true
                    // A 401 of a request of an earlier epoch changes nothing.
                    SessionEvent.SignedOut -> if (reported.epoch == graph.session.epoch.value) {
                        scope.launch {
                            graph.sessionActions.sessionLost()
                            graph.toasts.show(R.string.tv_shell_signedOut)
                        }
                    }
                }
            }
        }
        TvScreenHost {
            key(epoch) {
                val nav = rememberNavController()
                val start = remember { startOf(state, graph) }
                val selected = rememberSaveable { mutableStateOf<String?>(null) }
                val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
                val all = if (profile == null) emptyList() else (libraries as? Loadable.Ready)?.value.orEmpty().let(graph.registry::mediaEntries)
                // As many libraries as fit (code review M-3), then «Alle Bibliotheken …» with the menu.
                val capacity = railLibraryCapacity(with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() })
                val media = if (profile == null || (all.isEmpty() && graph.registry.lists.isEmpty())) emptyList() else all.take(capacity) + librariesMenuEntry()
                val currentMedia by rememberUpdatedState(all)
                var librariesMenu by rememberSaveable { mutableStateOf(false) }
                var menuClosed by remember { mutableIntStateOf(0) }
                val navigator = remember(nav) { TvNavigator(nav, graph.registry, selected, { currentMedia }, openLibraries = { librariesMenu = true }) }
                val app = remember(navigator) { TvScope(graph, navigator) }
                val current by nav.currentBackStackEntryAsState()
                // Libraries count as well (review M-3): a library opened from Start is marked.
                val here = graph.registry.entryOf(current, all)?.key
                LaunchedEffect(here) { if (here != null) selected.value = here }
                val page = current
                val fullScreen = page == null || (ENTRY_ROUTES + graph.registry.fullScreen).any { page.isRoute(it) }
                val showRail = !fullScreen && profile != null
                val railFocus = remember { FocusRequester() }
                var railOpen by remember { mutableStateOf(false) }
                var armed by remember { mutableStateOf<Long?>(null) }
                // Registered before the NavHost: every page's and menu's own handler comes first.
                BackHandler(enabled = page != null && nav.previousBackStackEntry == null) {
                    when {
                        showRail && !railOpen -> runCatching { railFocus.requestFocus() }
                        armed?.let { clock() - it < TvMotion.EXIT_MS } == true -> {
                            armed = null
                            onExit()
                        }
                        else -> {
                            armed = clock()
                            graph.toasts.show(R.string.tv_shell_exitHint)
                        }
                    }
                }
                // The menu «Bibliotheken» (mockup `ACT.libs`): from «Alle Bibliotheken …» and the empty start page.
                if (librariesMenu && profile != null) {
                    LibrariesMenu(
                        libraries = all,
                        lists = graph.registry.lists,
                        onOpen = { entry ->
                            librariesMenu = false
                            navigator.openRail(entry)
                        },
                        onDismiss = {
                            librariesMenu = false
                            menuClosed++
                        },
                    )
                }
                // Back from the menu: the focus goes to the rail (code review M-4: never nowhere).
                LaunchedEffect(menuClosed) {
                    if (menuClosed == 0) return@LaunchedEffect
                    withFrameNanos { }
                    runCatching { railFocus.requestFocus() }
                }
                TvShell(
                    showRail = showRail,
                    rail = graph.registry.rail,
                    media = media,
                    // A library beyond the rail and «Meine Listen» mark «Alle Bibliotheken …».
                    selected = selected.value?.takeIf { key -> (graph.registry.rail + media).any { it.key == key } } ?: LIBRARIES_MENU.takeIf { selected.value != null },
                    profile = profile,
                    onEntry = { entry -> if (entry.key == LIBRARIES_MENU) librariesMenu = true else navigator.openRail(entry) },
                    onProfile = navigator::toProfiles,
                    railFocus = railFocus,
                    railOpen = railOpen,
                    onRailOpen = { railOpen = it },
                    // Stage 5: the unread notifications on «Einstellungen» (a feature brings the number).
                    badge = { entry -> graph.registry.badges[entry.key]?.let { b -> b.count(graph)?.takeIf { it > 0 }?.let { n -> n to b.label(n) } } },
                ) {
                    // No page transitions: a page is there at once and takes its start focus (mockup).
                    NavHost(
                        navController = nav,
                        startDestination = start,
                        modifier = Modifier.fillMaxSize(),
                        enterTransition = { EnterTransition.None },
                        exitTransition = { ExitTransition.None },
                        popEnterTransition = { EnterTransition.None },
                        popExitTransition = { ExitTransition.None },
                    ) {
                        setupDestinations(graph, navigator)
                        profilesDestination(graph, navigator)
                        for (feature in graph.registry.features) {
                            with(feature) { register(app) }
                        }
                    }
                }
            }
            TvToastHost(graph.toasts)
            if (tooOld) AppTooOldDialog(onClose = { tooOld = false })
        }
    }
}

/** Where an epoch opens: the setup, the profile choice or the first entry of the rail. */
private fun startOf(state: SessionState, graph: TvGraph): Any = when (state) {
    SessionState.Loading, SessionState.NoServer, is SessionState.SignedOut -> SetupRoute
    is SessionState.ChooseProfile -> ProfilesRoute
    is SessionState.Ready -> graph.registry.home.route
}

private fun NavGraphBuilder.setupDestinations(graph: TvGraph, navigator: TvNavigator) {
    composable<SetupRoute> {
        SetupScreen(onPhone = { navigator.toSetup(PairRoute) }, onHand = { navigator.toSetup(HandSetupRoute) })
    }
    composable<PairRoute> {
        val vm = viewModel {
            PairViewModel(
                relay = { graph.relaySettings.relay.first() },
                flowFor = graph::pairing,
                probe = graph.serverProbe,
                servers = graph.serverStore,
                session = graph.session,
                device = graph.device,
                systemVersion = graph.systemVersion,
                cleanup = graph.scope,
            )
        }
        // The pairing lives only while the page is seen (review M-1): the entry stops when another
        // page comes on top (way by hand), on Back, on Home and with the app in the background.
        LifecycleStartEffect(vm) {
            vm.onVisible()
            onStopOrDispose { vm.onHidden() }
        }
        val state by vm.state.collectAsStateWithLifecycle()
        val relay by graph.relaySettings.relay.collectAsStateWithLifecycle(com.slouchcat.app.tv.common.RelaySettings.DEFAULT)
        var relayMenu by rememberSaveable { mutableStateOf(false) }
        val scope = rememberCoroutineScope()
        PairScreen(
            state = state,
            onNewCode = vm::onNewCode,
            onHand = { navigator.navigate(HandSetupRoute) },
            onRelay = { relayMenu = true },
            onContinue = vm::onContinue,
            onUndo = { vm.onUndo { graph.toasts.show(R.string.tv_pair_undone) } },
            onInput = vm::onInput,
        )
        if (relayMenu) {
            RelayMenu(
                current = relay,
                onSave = { address ->
                    relayMenu = false
                    scope.launch {
                        graph.relaySettings.set(address)
                        vm.onNewCode()
                    }
                },
                onDismiss = { relayMenu = false },
            )
        }
    }
    composable<HandSetupRoute> {
        val vm = viewModel {
            HandSetupViewModel(
                saved = createSavedStateHandle(),
                probe = graph.serverProbe,
                resolver = graph.hostResolver,
                servers = graph.serverStore,
                session = graph.session,
                auth = graph.auth,
                device = graph.device,
                // No message «Verbunden mit …»: the profile choice says it at its foot (K-9), and a
                // message would cover exactly that line.
            )
        }
        val state by vm.state.collectAsStateWithLifecycle()
        HandSetupScreen(
            state = state,
            onAddress = vm::onAddress,
            onConsent = vm::onConsent,
            onNext = vm::onNext,
            onEmail = vm::onEmail,
            onPassword = vm::onPassword,
            onTogglePassword = vm::onTogglePassword,
            onSignIn = vm::onSignIn,
            onBack = { vm.onBack() },
        )
    }
}

private fun NavGraphBuilder.profilesDestination(graph: TvGraph, navigator: TvNavigator) {
    composable<ProfilesRoute> {
        val vm = viewModel { ProfilesViewModel(graph.session, graph.auth, graph.sessionActions) }
        val state by vm.state.collectAsStateWithLifecycle()
        val manage = graph.registry.manageProfiles
        // A chosen profile and a sign-out start a new epoch: the NavHost opens the next page.
        ProfilesScreen(
            state = state,
            onPick = vm::onPick,
            onDigit = vm::onDigit,
            onDelete = vm::onDelete,
            onSubmitPin = vm::onSubmitPin,
            onCancelPin = vm::onCancelPin,
            onRetry = vm::retry,
            onSignOut = vm::onSignOut,
            // Only with a profile in use (code review m-2): before the first choice the page could
            // only read, and «Profil wechseln» there stacked a second choice.
            onManage = manage?.takeIf { graph.session.state.collectAsStateWithLifecycle().value is SessionState.Ready }?.let { route -> { navigator.navigate(route) } },
        )
    }
}

/** «Bitte die App aktualisieren» (426 `app_too_old` anywhere). */
@Composable
private fun AppTooOldDialog(onClose: () -> Unit) {
    val close = remember { FocusRequester() }
    TvDialog(stringResource(R.string.app_update_title), onDismiss = onClose, initialFocus = close) {
        androidx.tv.material3.Text(stringResource(R.string.app_update_body), style = TvType.large, color = SlouchcatTheme.colors.tx2)
        TvActions {
            TvButton(stringResource(R.string.app_update_close), onClick = onClose, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(close))
        }
    }
}

