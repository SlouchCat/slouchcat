package com.slouchcat.app.tv.item

import android.content.ActivityNotFoundException
import android.content.Intent
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.core.net.toUri
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.PersonRoute
import com.slouchcat.app.tv.browse.PlayerRoute
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.strings.R

/**
 * The detail pages of stage 2b (concept 26, 3.3; mockup `/item/:i`): film, series with seasons and
 * episodes, season, episode. «Abspielen» and «Von vorn» open [PlayerRoute] and show only once the
 * player of stage 3 registers it; the trailer opens the app of YouTube on the TV. [repository]:
 * the data of the page; tests bring a fake.
 */
class ItemFeature(
    private val repository: (TvGraph) -> DetailRepository = { it.media.detail },
) : TvFeature {
    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ItemRoute> { entry ->
            val id = entry.toRoute<ItemRoute>().id
            val vm = viewModel { ItemViewModel(graph.session, repository(graph), id, watchlistCount = graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        ItemMessage.Failed -> graph.toasts.show(R.string.app_item_failed)
                        is ItemMessage.Recommended -> graph.toasts.show(R.string.tv_item_recommended)
                        ItemMessage.Unrecommended -> graph.toasts.show(R.string.item_recommend_removed)
                    }
                }
            }
            // Back from the player (stage 3): the title again, quietly (the new progress).
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val context = LocalContext.current
            val links = remember(navigator) {
                val player = navigator.canOpen(PlayerRoute::class)
                ItemLinks(
                    item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it)) }) else null,
                    person = if (navigator.canOpen(PersonRoute::class)) ({ navigator.navigate(PersonRoute(it)) }) else null,
                    play = if (player) ({ navigator.navigate(PlayerRoute(it)) }) else null,
                    playFromStart = if (player) ({ navigator.navigate(PlayerRoute(it, fromStart = true)) }) else null,
                    trailer = { youtube ->
                        // The app of YouTube takes the address; without one, a message instead of a crash.
                        try {
                            context.startActivity(Intent(Intent.ACTION_VIEW, (YOUTUBE + youtube).toUri()).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                        } catch (_: ActivityNotFoundException) {
                            graph.toasts.show(R.string.app_item_trailerFailed)
                        }
                    },
                )
            }
            ItemScreen(state, images, links, vm)
        }
    }

    private companion object {
        const val YOUTUBE = "https://www.youtube.com/watch?v="
    }
}
