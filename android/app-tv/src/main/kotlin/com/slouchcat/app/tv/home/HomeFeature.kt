package com.slouchcat.app.tv.home

import android.content.Context
import android.provider.Settings
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.tv.browse.CollectionRoute
import com.slouchcat.app.tv.browse.CollectionsRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.LibraryRoute
import com.slouchcat.app.tv.browse.ListRoute
import com.slouchcat.app.tv.browse.player
import com.slouchcat.app.tv.player.Playbacks
import com.slouchcat.app.tv.browse.SearchRoute
import com.slouchcat.app.tv.browse.WatchlistRoute
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.browse.home.HomeMessage
import com.slouchcat.core.browse.home.HomeViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable

/** «Start» (mockup `/`). */
@Serializable
data object HomeRoute

/**
 * «Start» (concept 26, 3.3, stage 2b): the first entry of the rail (Top, order 10), where the app
 * lands after the profile choice, and the page with the stage and the rows of `getHome`.
 */
object HomeFeature : TvFeature {
    override val rail = listOf(
        RailEntry("start", RailSection.Top, 10, Label.Res(R.string.nav_home), SlouchcatIcon.Home.vector, HomeRoute),
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<HomeRoute> {
            val vm = viewModel { HomeViewModel(graph.session, graph.media.home, graph.media.items, graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            // Back from the player (stage 3): the rows again, quietly (progress, «Weiterschauen»).
            // After the last progress of the player reached the server (at most 2 s; review m-6).
            val resumed = rememberCoroutineScope()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { resumed.launch { Playbacks.settled(graph); vm.onShown() } }
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is HomeMessage.Watchlist -> graph.toasts.show(if (message.added) R.string.app_home_watchlistAdded else R.string.app_home_watchlistRemoved, message.title)
                        HomeMessage.WatchlistFailed -> graph.toasts.show(R.string.app_home_watchlistFailed)
                    }
                }
            }
            val links = remember(navigator) {
                HomeLinks(
                    item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null,
                    // Audiobooks and episodes («Weiterhören») go to the audio player of stage 4.
                    play = navigator.player()?.let { play -> { item -> play(item.id, item.kind) } },
                    showAll = { row ->
                        val list = row.list
                        val library = row.libraryId
                        when {
                            list != null -> navigator.opener(ListRoute(list.value))
                            row.picks -> navigator.opener(ListRoute(ListRoute.PICKS))
                            row.key == HomeRowKey.LIBRARY && library != null -> navigator.opener(LibraryRoute(library, LibrarySort.ADDED.value))
                            row.key == HomeRowKey.WATCHLIST -> navigator.opener(WatchlistRoute)
                            else -> null
                        }
                    },
                    collection = if (navigator.canOpen(CollectionRoute::class)) ({ navigator.navigate(CollectionRoute(it.id)) }) else null,
                    collections = navigator.opener(CollectionsRoute),
                    genre = if (navigator.canOpen(SearchRoute::class)) ({ navigator.openTarget("search", SearchRoute(genre = it)) }) else null,
                    libraries = navigator.openLibraries,
                )
            }
            HomeScreen(
                state = state,
                links = links,
                onRetry = vm::retry,
                onRowEnd = vm::onRowEnd,
                onWatchlist = vm::onWatchlist,
                rotate = motionAllowed(LocalContext.current),
                onStage = vm::onStage,
            )
        }
    }
}

/** Whether the stage may move on by itself: not when the system turned animations off. */
private fun motionAllowed(context: Context): Boolean {
    // With a screen reader the content under the focus must not change by itself (code review L-6).
    val reader = (context.getSystemService(Context.ACCESSIBILITY_SERVICE) as? android.view.accessibility.AccessibilityManager)?.isTouchExplorationEnabled == true
    return !reader && Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) > 0f
}
