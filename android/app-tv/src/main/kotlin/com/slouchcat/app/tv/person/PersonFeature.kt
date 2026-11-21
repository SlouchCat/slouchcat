package com.slouchcat.app.tv.person

import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.PersonRoute
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.media.ServerImages

/** A person (concept 26, 3.3, stage 2b; mockup `/person/:id`), opened from the cast or the search. */
object PersonFeature : TvFeature {
    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<PersonRoute> { entry ->
            val id = entry.toRoute<PersonRoute>().id
            val vm = viewModel { PersonViewModel(graph.session, graph.media.person, id) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            PersonScreen(state, images, if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it)) }) else null, vm::retry)
        }
    }
}
