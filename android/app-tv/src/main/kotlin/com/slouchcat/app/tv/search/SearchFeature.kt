package com.slouchcat.app.tv.search

import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.speech.RecognizerIntent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
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
import com.slouchcat.app.tv.browse.media
import com.slouchcat.app.tv.library.libraryIcon
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.api.generated.models.LibraryKind
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
            val context = LocalContext.current
            // The voice input of the device (paper 3.4), where an app on the TV offers it.
            val speech = remember { Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM) }
            val canSpeak = remember { context.packageManager.queryIntentActivities(speech, PackageManager.MATCH_DEFAULT_ONLY).isNotEmpty() }
            val listen = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
                val said = result.data?.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()
                if (result.resultCode == Activity.RESULT_OK && !said.isNullOrBlank()) vm.onText(said)
            }
            val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
            val tiles = (libraries as? Loadable.Ready)?.value.orEmpty().filter { it.kind == LibraryKind.VIDEO }.map { library ->
                SearchLibrary(library.name, libraryIcon(library)) { navigator.openerOf("library-${library.id}")?.invoke() ?: navigator.navigate(LibraryRoute(library.id)) }
            }
            val links = SearchLinks(
                item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it)) }) else null,
                person = if (navigator.canOpen(PersonRoute::class)) ({ navigator.navigate(PersonRoute(it)) }) else null,
                collection = if (navigator.canOpen(CollectionRoute::class)) ({ navigator.navigate(CollectionRoute(it)) }) else null,
                libraries = if (navigator.canOpen(LibraryRoute::class)) tiles else emptyList(),
                voice = if (canSpeak) ({ listen.launch(speech) }) else null,
            )
            SearchScreen(state, images, links, vm)
        }
    }
}
