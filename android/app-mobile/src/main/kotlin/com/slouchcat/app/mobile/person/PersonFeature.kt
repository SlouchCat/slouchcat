package com.slouchcat.app.mobile.person

import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.PersonRepository

/** The person page of stage 4 (mockup `/person/:id`); in-app address `person/<id>`. [repository]: tests bring a fake. */
class PersonFeature(
    private val repository: (AppGraph) -> PersonRepository = { it.media.person },
) : Feature {
    override fun link(address: String): AppLink? =
        address.removePrefix("/").takeIf { it.startsWith("person/") }?.removePrefix("person/")?.toLongOrNull()?.takeIf { it > 0 }
            ?.let { AppLink("start", listOf(PersonRoute(it))) }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<PersonRoute> { entry ->
            val id = entry.toRoute<PersonRoute>().id
            val vm = viewModel { PersonViewModel(graph.session, repository(graph), id) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            PersonScreen(
                state = state,
                images = images,
                onBack = navigator::back,
                onRetry = vm::retry,
                onToggleBiography = vm::onToggleBiography,
                onOpen = { navigator.opener(ItemRoute(it)) },
            )
        }
    }
}
