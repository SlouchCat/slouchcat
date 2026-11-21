package com.slouchcat.app.mobile.collections

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalResources
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
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.flow
import java.util.WeakHashMap

/**
 * «Sammlungen» of stage 4 (mockup `/collections`, `/collection/:id`): the entry of «Meine Medien»
 * with the number, a row of «Mehr» and the two pages; in-app address `collection/<id>`.
 * [repository]: tests bring a fake.
 */
class CollectionsFeature(
    private val repository: (AppGraph) -> CollectionsRepository = { ApiCollectionsRepository(it.api, it.http) },
) : Feature {
    private val counts = WeakHashMap<AppGraph, CollectionsCount>()

    private fun count(graph: AppGraph): CollectionsCount = synchronized(counts) {
        counts.getOrPut(graph) { CollectionsCount().also { count -> graph.hooks.onProfileOrSessionEnd { count.set(null) } } }
    }

    private fun countFlow(graph: AppGraph) = flow {
        val count = count(graph)
        graph.session.server?.let { count.refresh(repository(graph), it) }
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
        LaunchedEffect(graph) { graph.session.server?.let { count.refresh(repository(graph), it) } }
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
            val vm = viewModel { CollectionViewModel(graph.session, repository(graph), id) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    snackbar.showSnackbar(
                        when (message) {
                            is CollectionMessage.Listed -> resources.getQuantityString(R.plurals.app_collections_listed, message.count, message.count)
                            CollectionMessage.Unlisted -> resources.getString(R.string.app_collections_unlisted)
                            CollectionMessage.Watched -> resources.getString(R.string.app_collections_watchedDone)
                            CollectionMessage.Unwatched -> resources.getString(R.string.app_collections_unwatchedDone)
                            CollectionMessage.Failed -> resources.getString(R.string.app_collections_failed)
                        },
                    )
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
