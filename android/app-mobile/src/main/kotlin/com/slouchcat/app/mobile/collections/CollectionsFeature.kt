package com.slouchcat.app.mobile.collections

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.res.pluralStringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.map
import com.slouchcat.core.browse.collections.CollectionMessage
import com.slouchcat.core.browse.collections.CollectionViewModel
import com.slouchcat.core.browse.collections.CollectionsViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.CollectionsRepository
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.flow

/**
 * «Sammlungen» of stage 4 (mockup `/collections`, `/collection/:id`): the entry of «Meine Medien»
 * with the number, a row of «Mehr» and the two pages; in-app address `collection/<id>`.
 * [repository]: tests bring a fake.
 */
class CollectionsFeature(
    private val repository: (AppGraph) -> CollectionsRepository = { it.media.collections },
) : Feature {
    /** The number of collections of the graph (`:core:media`). */
    private fun count(graph: AppGraph): LiveCount = graph.media.collectionsCount

    private fun countFlow(graph: AppGraph) = flow {
        val count = count(graph)
        graph.session.server?.let { server -> count.refresh { repository(graph).collections(server).map { it.size } } }
        emitAll(count.count)
    }

    override val media = listOf(
        MediaEntries {
            listOf(MediaEntry("collections", MediaSection.MyMedia, 30, Label.Res(R.string.nav_collections), SlouchcatIcon.Grid, CollectionsRoute, liveCount = ::countFlow))
        },
    )

    override val more = listOf(
        MoreEntry("collections", MoreSection.Library, 30, R.string.nav_collections, SlouchcatIcon.Grid, CollectionsRoute, subtitle = { subtitle(it) }),
    )

    override fun link(address: String): AppLink? =
        address.removePrefix("/").takeIf { it.startsWith("collection/") }?.removePrefix("collection/")?.takeIf { it.isNotBlank() && '/' !in it }
            ?.let { AppLink("start", listOf(CollectionRoute(it))) }

    @Composable
    private fun subtitle(graph: AppGraph): String? {
        val count = remember(graph) { count(graph) }
        LaunchedEffect(graph) { graph.session.server?.let { server -> count.refresh { repository(graph).collections(server).map { it.size } } } }
        val n by count.count.collectAsStateWithLifecycle()
        return n?.let { pluralStringResource(R.plurals.collection_all_count, it, it) }
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<CollectionsRoute> {
            val vm = viewModel { CollectionsViewModel(graph.session, repository(graph), count(graph)) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            CollectionsScreen(
                state = state,
                images = images,
                onBack = navigator::back,
                onRetry = vm::retry,
                onOpen = { navigator.opener(CollectionRoute(it)) },
            )
        }
        composable<CollectionRoute> { entry ->
            val id = entry.toRoute<CollectionRoute>().id
            val vm = viewModel { CollectionViewModel(graph.session, repository(graph), id, graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val snackbar = remember { SnackbarHostState() }
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    is CollectionMessage.Listed -> getQuantityString(R.plurals.app_collections_listed, message.count, message.count)
                    CollectionMessage.Unlisted -> getString(R.string.app_collections_unlisted)
                    CollectionMessage.Watched -> getString(R.string.app_collections_watchedDone)
                    CollectionMessage.Unwatched -> getString(R.string.app_collections_unwatchedDone)
                    CollectionMessage.Failed -> getString(R.string.app_collections_failed)
                }
            }
            CollectionScreen(
                state = state,
                images = images,
                snackbar = snackbar,
                onBack = navigator::back,
                onRetry = vm::retry,
                onListAll = vm::onListAll,
                onWatched = vm::onWatched,
                onConfirm = vm::onConfirm,
                onDismiss = vm::onDismiss,
                onOpen = { navigator.opener(ItemRoute(it)) },
                onPlay = { navigator.opener(PlayerRoute(it)) },
            )
        }
    }
}
