package com.slouchcat.app.tv.search

import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.browse.CollectionRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.LibraryRoute
import com.slouchcat.app.tv.browse.PersonRoute
import com.slouchcat.app.tv.browse.SearchRoute
import com.slouchcat.app.tv.library.libraryIcon
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.wishes.MissingTitles
import com.slouchcat.app.tv.wishes.WishTitleRoute
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.browse.search.SearchViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/** «Suche» (concept 26, 3.3, stage 2b; mockup `/search`): the second entry of the rail (Top, order 20). */
object SearchFeature : TvFeature {
    override val rail = listOf(
        RailEntry("search", RailSection.Top, 20, Label.Res(R.string.nav_search), SlouchcatIcon.Search.vector, SearchRoute()),
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<SearchRoute> { entry ->
            val genre = entry.toRoute<SearchRoute>().genre
            val vm = viewModel { SearchViewModel(graph.session, graph.media.search, genre) }
            val state by vm.state.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val voice = rememberVoiceInput(vm::onText)
            val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
            val tiles = (libraries as? Loadable.Ready)?.value.orEmpty().filter { it.kind == LibraryKind.VIDEO }.map { library ->
                SearchLibrary(library.name, libraryIcon(library)) { navigator.openerOf("library-${library.id}")?.invoke() ?: navigator.navigate(LibraryRoute(library.id)) }
            }
            val links = SearchLinks(
                item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it)) }) else null,
                person = if (navigator.canOpen(PersonRoute::class)) ({ navigator.navigate(PersonRoute(it)) }) else null,
                collection = if (navigator.canOpen(CollectionRoute::class)) ({ navigator.navigate(CollectionRoute(it)) }) else null,
                libraries = if (navigator.canOpen(LibraryRoute::class)) tiles else emptyList(),
                voice = voice,
                // Stage 5: «Noch nicht in der Bibliothek?» once the wishes' pages are there.
                missing = if (navigator.canOpen(WishTitleRoute::class)) ({ query, focus -> MissingTitles(graph, navigator, query, focus) }) else null,
            )
            SearchScreen(state, images, links, vm)
        }
    }
}
