package com.slouchcat.app.tv.lists

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.browse.CollectionRoute
import com.slouchcat.app.tv.browse.CollectionsRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.ListRoute
import com.slouchcat.app.tv.browse.PlayerRoute
import com.slouchcat.app.tv.browse.WatchedRoute
import com.slouchcat.app.tv.browse.WatchlistRoute
import com.slouchcat.app.tv.browse.media
import com.slouchcat.app.tv.collections.CollectionMessage
import com.slouchcat.app.tv.collections.CollectionViewModel
import com.slouchcat.app.tv.collections.CollectionsViewModel
import com.slouchcat.app.tv.library.LibraryFeature
import com.slouchcat.app.tv.mylists.ListMessage
import com.slouchcat.app.tv.mylists.WatchedViewModel
import com.slouchcat.app.tv.mylists.WatchlistViewModel
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailMedia
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.shell.isRoute
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/**
 * The lists of stage 2b (concept 26, 3.3): «Alle anzeigen» of a start page row, «Merkliste»,
 * «Gesehen», «Sammlungen» and one collection. «Merkliste», «Gesehen» and «Sammlungen» are entries of
 * the section «Medien» of the rail after the libraries: the mockup reaches them only from the start
 * page, but «Gesehen» has no row there (deviation, stage 2b report).
 */
object ListsFeature : TvFeature {
    override val media = listOf(
        RailMedia { _ ->
            listOf(
                entry("watchlist", 0, R.string.watchlist_title, SlouchcatIcon.Bookmark, WatchlistRoute),
                entry("watched", 1, R.string.watched_page_title, SlouchcatIcon.Eye, WatchedRoute),
                RailEntry("collections", RailSection.Media, ORDER + 2, Label.Res(R.string.collection_all_title), SlouchcatIcon.Grid.vector, CollectionsRoute) { entry ->
                    entry.isRoute(CollectionsRoute::class) || entry.isRoute(CollectionRoute::class)
                },
            )
        },
    )

    private fun entry(key: String, at: Int, label: Int, icon: SlouchcatIcon, route: Any) =
        RailEntry(key, RailSection.Media, ORDER + at, Label.Res(label), icon.vector, route) { it.isRoute(route::class) }

    /** After any number of libraries (LibraryFeature counts up from its own order). */
    private const val ORDER = LibraryFeature.ORDER + 1000

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        val item: (() -> ((com.slouchcat.core.media.MediaItem) -> Unit)?) = { if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null }
        val home = { navigator.openerOf("start") }
        composable<ListRoute> { entry ->
            val key = entry.toRoute<ListRoute>().key
            val vm = viewModel { ListViewModel(graph.session, graph.media.home, key) }
            val state by vm.state.collectAsStateWithLifecycle()
            ListScreen(vm.kind, state, item(), vm::retry, vm::onNearEnd, home())
        }
        composable<WatchlistRoute> {
            val vm = viewModel { WatchlistViewModel(graph.session, graph.media.myLists, graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is ListMessage.Removed -> graph.toasts.show(R.string.tv_lists_removed, message.title)
                        is ListMessage.Unwatched -> graph.toasts.show(R.string.tv_lists_unwatched, message.title)
                        ListMessage.Failed -> graph.toasts.show(R.string.app_lists_failed)
                    }
                }
            }
            WatchlistScreen(state, item(), vm::onRemove, vm::onToggleEdit, vm::retry, vm::onMore, home())
        }
        composable<WatchedRoute> {
            val vm = viewModel { WatchedViewModel(graph.session, graph.media.myLists) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is ListMessage.Unwatched -> graph.toasts.show(R.string.tv_lists_unwatched, message.title)
                        is ListMessage.Removed -> graph.toasts.show(R.string.tv_lists_removed, message.title)
                        ListMessage.Failed -> graph.toasts.show(R.string.app_lists_failed)
                    }
                }
            }
            WatchedScreen(state, item(), vm::onAsk, vm::onConfirm, vm::onDismiss, vm::onToggleEdit, vm::retry, vm::onMore, home())
        }
        composable<CollectionsRoute> {
            val vm = viewModel { CollectionsViewModel(graph.session, graph.media.collections, graph.media.collectionsCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            CollectionsScreen(state, images, if (navigator.canOpen(CollectionRoute::class)) ({ navigator.navigate(CollectionRoute(it.id)) }) else null, vm::retry, home())
        }
        composable<CollectionRoute> { entry ->
            val id = entry.toRoute<CollectionRoute>().id
            val vm = viewModel { CollectionViewModel(graph.session, graph.media.collections, id, graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is CollectionMessage.Listed -> graph.toasts.show(R.string.tv_lists_listed, message.count.toString())
                        CollectionMessage.Unlisted -> graph.toasts.show(R.string.app_collections_unlisted)
                        CollectionMessage.Watched -> graph.toasts.show(R.string.app_collections_watchedDone)
                        CollectionMessage.Unwatched -> graph.toasts.show(R.string.app_collections_unwatchedDone)
                        CollectionMessage.Failed -> graph.toasts.show(R.string.app_collections_failed)
                    }
                }
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val actions = remember(vm, navigator) {
                CollectionActions(
                    item = item(),
                    // A series of a collection plays from its own page (its next episode); a film plays itself.
                    play = if (navigator.canOpen(PlayerRoute::class)) ({ navigator.navigate(PlayerRoute(it.id)) }) else null,
                    listAll = vm::onListAll,
                    watched = vm::onWatched,
                    confirm = vm::onConfirm,
                    dismiss = vm::onDismiss,
                    retry = vm::retry,
                )
            }
            CollectionScreen(state, images, actions)
        }
    }
}
