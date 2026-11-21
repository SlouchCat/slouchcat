package com.slouchcat.app.mobile.item

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.LocalUriHandler
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.core.strings.R

/**
 * The detail pages of stage 4 (concept 27, 3.1; mockup `/item/:i`): film, series with seasons and
 * episodes, season, episode, and a plain page for every other kind until its own page comes
 * (audiobooks and podcasts in stage 7). In-app address `item/<id>` (a notification of stage 8).
 * [repository]: the data of the page; tests bring a fake.
 */
class ItemFeature(
    private val repository: (AppGraph) -> DetailRepository = { ApiDetailRepository(it.api, it.http) },
) : Feature {
    override fun link(address: String): AppLink? =
        address.removePrefix("/").takeIf { it.startsWith("item/") }?.removePrefix("item/")?.takeIf { it.isNotBlank() && '/' !in it }
            ?.let { AppLink(TARGET, listOf(ItemRoute(it))) }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ItemRoute> { entry ->
            val id = entry.toRoute<ItemRoute>().id
            val vm = viewModel { ItemViewModel(graph.session, repository(graph), id) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val resources = LocalResources.current
            val uri = LocalUriHandler.current
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    val text = when (message) {
                        ItemMessage.Failed -> resources.getString(R.string.app_item_failed)
                        is ItemMessage.Recommended -> resources.getQuantityString(R.plurals.item_recommend_added, message.days, message.days)
                        ItemMessage.Unrecommended -> resources.getString(R.string.item_recommend_removed)
                    }
                    snackbar.showSnackbar(text)
                }
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val play = navigator.opener(PlayerRoute(id))?.let { { target: String -> navigator.navigate(PlayerRoute(target)) } }
            ItemScreen(
                state = state,
                images = images,
                links = ItemLinks(
                    back = navigator::back,
                    item = { navigator.opener(ItemRoute(it)) },
                    person = { navigator.opener(PersonRoute(it)) },
                    collection = { navigator.opener(CollectionRoute(it)) },
                    search = navigator.opener(SearchRoute()),
                    play = play,
                    trailer = { youtube -> uri.openUri(YOUTUBE + youtube) },
                ),
                events = vm,
                snackbar = snackbar,
            )
        }
    }

    private companion object {
        /** The target a jump to a title opens under: «Start» (the first target when there is none). */
        const val TARGET = "start"
        const val YOUTUBE = "https://www.youtube.com/watch?v="
    }
}
