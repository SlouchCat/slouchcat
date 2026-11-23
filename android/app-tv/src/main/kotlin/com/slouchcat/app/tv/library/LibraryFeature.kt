package com.slouchcat.app.tv.library

import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.LibraryRoute
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailMedia
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.shell.isRoute
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.browse.library.LibraryViewModel

/**
 * The libraries (concept 26, 3.3, stage 2b; mockup `/library/:key`): one entry per library of the
 * profile in the section «Medien» of the rail, in the server's order, and the page of each: films
 * and series, and since stage 4 audiobooks and podcasts (square covers; the radio is an entry of
 * `AudioFeature` after them). An entry is
 * marked on its library and on the titles opened from it ([LibraryRoute] only: a title belongs to
 * whatever page it was opened from).
 */
object LibraryFeature : TvFeature {
    override val media = listOf(
        RailMedia { libraries ->
            libraries.filter { it.kind in SHOWN }.mapIndexed { index, library ->
                RailEntry(
                    key = "library-${library.id}",
                    section = RailSection.Media,
                    order = ORDER + index,
                    label = Label.Plain(library.name),
                    icon = libraryIcon(library),
                    route = LibraryRoute(library.id),
                    isCurrent = { entry -> entry.isRoute(LibraryRoute::class) && entry.toRoute<LibraryRoute>().id == library.id },
                    count = library.itemCount,
                )
            }
        },
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<LibraryRoute> { entry ->
            val route = entry.toRoute<LibraryRoute>()
            val vm = viewModel {
                LibraryViewModel(
                    graph.session,
                    graph.media.library,
                    route.id,
                    graph.librariesSource.libraries,
                    initialSort = LibrarySort.entries.firstOrNull { it.value == route.sort },
                )
            }
            val state by vm.state.collectAsStateWithLifecycle()
            val actions = remember(vm, navigator) {
                LibraryActions(
                    retry = vm::retry,
                    nearEnd = vm::onNearEnd,
                    sheet = vm::onSheet,
                    sort = vm::onSort,
                    all = vm::onAll,
                    unseen = vm::onUnseen,
                    genre = vm::onGenre,
                    decade = vm::onDecade,
                    item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it.id)) }) else null,
                )
            }
            LibraryScreen(state, actions)
        }
    }

    /** Libraries first in «Medien» (the radio of stage 4 after them). */
    const val ORDER = 10

    /** The kinds of libraries with a page (stage 4 added sound). */
    private val SHOWN = setOf(LibraryKind.VIDEO, LibraryKind.AUDIOBOOKS, LibraryKind.PODCASTS)
}
