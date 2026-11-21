package com.slouchcat.app.mobile.mylists

import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.WatchedRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.browse.media
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.map
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.data.MyListsRepository
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.flow

/**
 * «Merkliste» and «Gesehen» of stage 4 (mockup `/watchlist`, `/watched`): two entries of «Meine
 * Medien» (the watchlist with its number) and two rows of «Mehr» (section «Bibliothek»).
 * [repository]: tests bring a fake.
 */
class MyListsFeature(
    private val repository: (AppGraph) -> MyListsRepository = { it.media.myLists },
) : Feature {
    /** The number of the watchlist of the graph (`:core:media`, one per graph; tests build several). */
    private fun count(graph: AppGraph): LiveCount = graph.media.watchlistCount

    private fun countFlow(graph: AppGraph) = flow {
        val count = count(graph)
        graph.session.server?.let { server -> count.refresh { repository(graph).watchlist(server, 0, 1).map { it.total } } }
        emitAll(count.count)
    }

    override val media = listOf(
        MediaEntries {
            listOf(
                MediaEntry("watchlist", MediaSection.MyMedia, 10, Label.Res(R.string.nav_watchlist), SlouchcatIcon.Bookmark, WatchlistRoute, liveCount = ::countFlow),
                MediaEntry("watched", MediaSection.MyMedia, 20, Label.Res(R.string.nav_watched), SlouchcatIcon.Eye, WatchedRoute),
            )
        },
    )

    override val more = listOf(
        MoreEntry("watchlist", MoreSection.Library, 10, R.string.nav_watchlist, SlouchcatIcon.Bookmark, WatchlistRoute, subtitle = { watchlistSubtitle(it) }),
        MoreEntry("watched", MoreSection.Library, 20, R.string.nav_watched, SlouchcatIcon.Eye, WatchedRoute, subtitle = { stringResource(R.string.app_lists_watchedHint) }),
    )

    @Composable
    private fun watchlistSubtitle(graph: AppGraph): String? {
        val count = remember(graph) { count(graph) }
        LaunchedEffect(graph) { graph.session.server?.let { server -> count.refresh { repository(graph).watchlist(server, 0, 1).map { it.total } } } }
        val n by count.count.collectAsStateWithLifecycle()
        return n?.let { if (it == 0) stringResource(R.string.app_lists_watchlistEmpty) else titleCount(it) }
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        val home = { navigator.openTop(graph.registry.home) }
        composable<WatchlistRoute> {
            val vm = viewModel { WatchlistViewModel(graph.session, repository(graph), count(graph)) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            Messages(vm.messages, snackbar, onUndo = vm::onUndo)
            WatchlistScreen(
                state = state,
                snackbar = snackbar,
                onBack = navigator::back,
                onRetry = vm::retry,
                onMore = vm::onMore,
                onToggleEdit = vm::onToggleEdit,
                onRemove = vm::onRemove,
                onOpen = { navigator.opener(ItemRoute(it)) },
                onHome = home,
            )
        }
        composable<WatchedRoute> {
            val vm = viewModel { WatchedViewModel(graph.session, repository(graph)) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            Messages(vm.messages, snackbar, onUndo = {})
            WatchedScreen(
                state = state,
                snackbar = snackbar,
                onBack = navigator::back,
                onRetry = vm::retry,
                onMore = vm::onMore,
                onToggleEdit = vm::onToggleEdit,
                onAsk = vm::onAsk,
                onConfirm = vm::onConfirm,
                onDismiss = vm::onDismiss,
                onOpen = { navigator.opener(ItemRoute(it)) },
                onHome = home,
            )
        }
    }
}

/** The toasts of a list page; «Rückgängig» after a title left the watchlist. */
@Composable
private fun Messages(messages: kotlinx.coroutines.flow.Flow<ListMessage>, snackbar: SnackbarHostState, onUndo: (String) -> Unit) {
    val resources = LocalResources.current
    LaunchedEffect(messages) {
        messages.collect { message ->
            when (message) {
                is ListMessage.Removed -> {
                    val result = snackbar.showSnackbar(
                        resources.getString(R.string.app_lists_removed, message.title),
                        actionLabel = resources.getString(R.string.app_lists_undo),
                        duration = SnackbarDuration.Long,
                    )
                    if (result == SnackbarResult.ActionPerformed) onUndo(message.id)
                }
                is ListMessage.Unwatched -> snackbar.showSnackbar(resources.getString(R.string.app_lists_unwatched, message.title))
                ListMessage.Failed -> snackbar.showSnackbar(resources.getString(R.string.app_lists_failed))
            }
        }
    }
}
