package com.slouchcat.app.tv.wishes

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvNavigator
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.browse.wishes.wishErrorRes
import com.slouchcat.core.browse.wishes.WishTitleMessage
import com.slouchcat.core.browse.wishes.WishTitleViewModel
import com.slouchcat.core.browse.wishes.WishesMessage
import com.slouchcat.core.browse.wishes.WishesTab
import com.slouchcat.core.browse.wishes.WishesViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/**
 * The area «Wünsche» (mockup `/wishes`); [tab] `mine` opens «Meine Wünsche» (a notification about a
 * wish, «Meine Wünsche» of a title), `search` its search with [query] (the card «„…“ wünschen» of the
 * main search, code review m-5).
 */
@Serializable
data class WishesRoute(val tab: String? = null, val query: String? = null)

/** A title outside the library (mockup `/wish/:i`, web `/titles/:media/:tmdbId`); [fromSearch]: a request counts as from the search. */
@Serializable
data class WishTitleRoute(val media: String, val tmdbId: Long, val fromSearch: Boolean = false)

/**
 * Stage 5 «Wünsche» of the TV app (concept 26, section 6 and 3.3; concept 25, ADR 0037): the entry
 * of the rail (star, Bottom, order 40, mockup `NAV`), the area with «Entdecken» (Seerr's rows of
 * films and series), «Suche» with the own keyboard, «Meine Wünsche» with state, progress and limit;
 * the page of a title outside the library with «Anfordern» (seasons of a series in a side menu) and
 * «Zurückziehen». Rule 11: request, show the state, withdraw, lead to the title once it is there;
 * approving and declining stay in the browser (a device session has no administration, 4.5). Rule
 * 12: the server says what the profile may request, the pages show it. [repository]: tests bring a fake.
 */
class WishesFeature(
    private val repository: (TvGraph) -> WishesRepository = { it.media.wishes },
) : TvFeature {
    override val rail = listOf(
        RailEntry(KEY, RailSection.Bottom, ORDER, Label.Res(R.string.nav_wishes), SlouchcatIcon.Wish.vector, WishesRoute()),
    )

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<WishesRoute> { entry ->
            val route = entry.toRoute<WishesRoute>()
            val start = when (route.tab) {
                MINE -> TvWishesTab.Mine
                SEARCH -> TvWishesTab.Search
                else -> TvWishesTab.Discover
            }
            val vm = viewModel {
                WishesViewModel(
                    graph.session,
                    repository(graph),
                    graph.media.openWishesCount,
                    if (start == TvWishesTab.Mine) WishesTab.Mine else WishesTab.Discover,
                    route.query,
                    withQuota = true,
                    searchPauseMs = SEARCH_PAUSE_MS,
                )
            }
            val state by vm.state.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onResume() }
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is WishesMessage.Withdrawn -> graph.toasts.show(R.string.titles_done_withdrawn)
                        is WishesMessage.Failed -> graph.toasts.show(wishErrorRes(message.code))
                    }
                }
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val links = remember(navigator) {
                WishesLinks(
                    title = { card -> openCard(navigator, card, fromSearch = false) },
                    hit = { card -> openCard(navigator, card, fromSearch = true) },
                    wish = { card -> openWish(navigator, card) },
                    home = navigator.openerOf(graph.registry.home.key),
                )
            }
            WishesScreen(state, images, links, vm, start = start)
        }
        composable<WishTitleRoute> { entry ->
            val route = entry.toRoute<WishTitleRoute>()
            val media = if (route.media == ExternalMedia.SHOW.value) ExternalMedia.SHOW else ExternalMedia.MOVIE
            val vm = viewModel {
                WishTitleViewModel(
                    graph.session,
                    repository(graph),
                    graph.media.openWishesCount,
                    media,
                    route.tmdbId,
                    if (route.fromSearch) WishCreate.Source.SEARCH else WishCreate.Source.DETAIL,
                )
            }
            val state by vm.state.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onResume() }
            LaunchedEffect(vm) {
                vm.messages.collect { message ->
                    when (message) {
                        is WishTitleMessage.Requested -> graph.toasts.show(if (message.joined) R.string.titles_done_joined else R.string.titles_done_requested)
                        WishTitleMessage.Withdrawn -> graph.toasts.show(R.string.titles_done_withdrawn)
                        is WishTitleMessage.Failed -> graph.toasts.show(wishErrorRes(message.code))
                    }
                }
            }
            // In the library meanwhile (409 `already_available`, code review m-3): its title instead of this page.
            LaunchedEffect(state.inLibrary) {
                val item = state.inLibrary ?: return@LaunchedEffect
                if (navigator.canOpen(ItemRoute::class)) navigator.replace(ItemRoute(item))
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            WishTitleScreen(
                state = state,
                images = images,
                actions = vm,
                // Above this page (code review m-7): Back returns here; the rail keeps its own pages.
                onMine = { navigator.navigate(WishesRoute(tab = MINE)) },
                onBack = { navigator.back() },
            )
        }
    }

    companion object {
        const val KEY = "wishes"
        const val MINE = "mine"
        const val SEARCH = "search"

        /** Below «Suche» … above «Einstellungen» (50): the place the rail keeps for it (mockup `NAV`). */
        const val ORDER = 40

        /** The search of «Wünsche» asks Seerr this long after the last key (each search is a request to TMDB). */
        const val SEARCH_PAUSE_MS = 600L
    }
}

/** A tile: its title in the library when the profile sees it there, else its page outside the library. */
internal fun openCard(navigator: TvNavigator, card: ExternalCard, fromSearch: Boolean) {
    val item = card.itemId
    if (item != null && navigator.canOpen(ItemRoute::class)) navigator.navigate(ItemRoute(item)) else navigator.navigate(WishTitleRoute(card.media.value, card.tmdbId, fromSearch))
}

/** A wish of «Meine Wünsche»: «Verfügbar» leads to the title in the library, every other one to its page. */
private fun openWish(navigator: TvNavigator, card: WishCard) {
    val item = card.itemId
    if (item != null && navigator.canOpen(ItemRoute::class)) navigator.navigate(ItemRoute(item)) else navigator.navigate(WishTitleRoute(card.media.value, card.tmdbId))
}
