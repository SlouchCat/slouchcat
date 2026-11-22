package com.slouchcat.app.mobile.home

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.LibraryRoute
import com.slouchcat.app.mobile.browse.ListRoute
import com.slouchcat.app.mobile.browse.NotificationsRoute
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.browse.SEARCH_TARGET
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.library.LibraryActions
import com.slouchcat.app.mobile.library.LibraryScreen
import com.slouchcat.app.mobile.lists.ListScreen
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.browse.home.HomeMessage
import com.slouchcat.core.browse.home.HomeViewModel
import com.slouchcat.core.browse.library.LibraryViewModel
import com.slouchcat.core.browse.lists.ListViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/** The start page, the first target of the bar («Start»). */
@Serializable
data object HomeRoute

/**
 * Stage 4, first part (concept 27, 3.1): «Start» with its rows, the libraries of the profile in the
 * media choice («Medien»), a library with filters and order, and the lists behind the rows
 * («Alle anzeigen», «User-Empfehlungen»). Links to pages of other features open only once those
 * features are registered (`Navigator.opener`).
 */
object HomeFeature : Feature {
    override val destinations = listOf(TopDestination("start", 10, HomeRoute, R.string.nav_home, SlouchcatIcon.Home))

    override val media = listOf(
        MediaEntries { libraries ->
            libraries.mapIndexed { index, library ->
                MediaEntry(
                    key = "library-${library.id}",
                    section = MediaSection.Media,
                    order = 10 + index,
                    label = Label.Plain(library.name),
                    icon = libraryIcon(library.kind, library.contents.isNotEmpty() && library.contents.all { it == RootContent.SERIES }),
                    route = LibraryRoute(library.id),
                    count = library.itemCount,
                    isCurrent = { entry -> entry.destination.hasRoute(LibraryRoute::class) && entry.toRoute<LibraryRoute>().id == library.id },
                )
            }
        },
    )

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<HomeRoute> {
            val vm = viewModel { HomeViewModel(graph.session, graph.media.home, graph.media.items, graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    is HomeMessage.Watchlist -> getString(if (message.added) R.string.app_home_watchlistAdded else R.string.app_home_watchlistRemoved, message.title)
                    HomeMessage.WatchlistFailed -> getString(R.string.app_home_watchlistFailed)
                }
            }
            val links = remember(navigator) {
                HomeLinks(
                    item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null,
                    play = if (navigator.canOpen(PlayerRoute::class)) ({ navigator.navigate(com.slouchcat.app.mobile.browse.playRoute(it.id, it.kind)) }) else null,
                    showAll = { row ->
                        val list = row.list
                        val libraryId = row.libraryId
                        when {
                            list != null -> navigator.opener(ListRoute(list.value))
                            row.picks -> navigator.opener(ListRoute(ListRoute.PICKS))
                            row.key == HomeRowKey.LIBRARY && libraryId != null -> navigator.opener(LibraryRoute(libraryId, LibrarySort.ADDED.value))
                            row.key == HomeRowKey.WATCHLIST -> navigator.opener(WatchlistRoute)
                            else -> null
                        }
                    },
                    collection = if (navigator.canOpen(CollectionRoute::class)) ({ navigator.navigate(CollectionRoute(it.id)) }) else null,
                    collections = navigator.opener(CollectionsRoute),
                    genre = if (navigator.canOpen(SearchRoute::class)) ({ navigator.openTarget(SEARCH_TARGET, SearchRoute(genre = it)) }) else null,
                    library = { navigator.navigate(LibraryRoute(it.id)) },
                    notifications = navigator.opener(NotificationsRoute),
                    profiles = navigator::toProfiles,
                )
            }
            HomeScreen(
                state = state,
                links = links,
                onRetry = vm::retry,
                onRefresh = vm::refresh,
                onRowEnd = vm::onRowEnd,
                onWatchlist = vm::onWatchlist,
                snackbar = snackbar,
                rotate = motionAllowed(LocalContext.current),
                onStage = vm::onStage,
            )
        }
        composable<LibraryRoute> { entry ->
            val route = entry.toRoute<LibraryRoute>()
            LibraryPage(app, route.id, route.sort)
        }
        composable<ListRoute> { entry ->
            val key = entry.toRoute<ListRoute>().key
            val vm = viewModel { ListViewModel(graph.session, graph.media.home, key) }
            val state by vm.state.collectAsStateWithLifecycle()
            ListScreen(
                kind = vm.kind,
                state = state,
                onBack = navigator::back,
                onRetry = vm::retry,
                onNearEnd = vm::onNearEnd,
                onItem = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null,
            )
        }
    }
}

/**
 * The page of one library (`listLibraryItems`, mockup `/library/:key`): from the media choice, from
 * a row of the start page and from «Hörbücher» and «Podcasts» of «Mehr» (stage 7, `AudioLibraryRoute`).
 */
@Composable
internal fun LibraryPage(app: AppScope, id: Long, sort: String?) {
    val graph = app.graph
    val navigator = app.navigator
    val vm = viewModel {
        LibraryViewModel(
            graph.session,
            graph.media.library,
            id,
            graph.librariesSource.libraries,
            initialSort = LibrarySort.entries.firstOrNull { it.value == sort },
        )
    }
    val state by vm.state.collectAsStateWithLifecycle()
    val actions = remember(vm, navigator) {
        LibraryActions(
            back = navigator::back,
            retry = vm::retry,
            nearEnd = vm::onNearEnd,
            sheet = vm::onSheet,
            sort = vm::onSort,
            all = vm::onAll,
            unseen = vm::onUnseen,
            genre = vm::onGenre,
            decade = vm::onDecade,
            content = vm::onContent,
            rating = vm::onRating,
            view = vm::onView,
            item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null,
            search = navigator.openerOf(SEARCH_TARGET),
        )
    }
    LibraryScreen(state, actions)
}

/**
 * Whether the stage may move on by itself: not when the system turned animations off (a
 * setting for reduced motion, also what the emulator flows set).
 */
private fun motionAllowed(context: android.content.Context): Boolean =
    android.provider.Settings.Global.getFloat(context.contentResolver, android.provider.Settings.Global.ANIMATOR_DURATION_SCALE, 1f) > 0f
