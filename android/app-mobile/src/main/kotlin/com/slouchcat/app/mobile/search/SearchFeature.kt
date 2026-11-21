package com.slouchcat.app.mobile.search

import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.LibraryRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.strings.R

/**
 * «Suche» of stage 4 (mockup `/search`): the target «Suche» of the bar (order 20) and the page
 * behind `SearchRoute(genre)`, which a library page opens with its genre. [repository]: tests
 * bring a fake.
 */
class SearchFeature(
    private val repository: (AppGraph) -> SearchRepository = { ApiSearchRepository(it.api, it.http) },
) : Feature {
    override val destinations = listOf(TopDestination("search", 20, SearchRoute(), R.string.nav_search, SlouchcatIcon.Search))

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<SearchRoute> { entry ->
            val genre = entry.toRoute<SearchRoute>().genre
            val vm = viewModel { SearchViewModel(graph.session, repository(graph), genre) }
            val state by vm.state.collectAsStateWithLifecycle()
            val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            SearchScreen(
                state = state,
                images = images,
                libraries = (libraries as? Loadable.Ready)?.value.orEmpty(),
                onText = vm::onText,
                onClear = vm::onClear,
                onSubmit = vm::onSubmit,
                onGenre = vm::onGenre,
                onClearGenre = vm::onClearGenre,
                onRetry = vm::retry,
                onRetryGenres = vm::retryGenres,
                onMore = vm::onMore,
                onItem = { navigator.opener(ItemRoute(it)) },
                onPerson = { navigator.opener(PersonRoute(it)) },
                onCollection = { navigator.opener(CollectionRoute(it)) },
                onLibrary = { navigator.opener(LibraryRoute(it)) },
            )
        }
    }
}
