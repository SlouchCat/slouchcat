package com.slouchcat.app.mobile.wishes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Navigator
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.browse.wishes.MissingTitlesViewModel
import com.slouchcat.core.browse.wishes.WishTitleMessage
import com.slouchcat.core.browse.wishes.WishTitleViewModel
import com.slouchcat.core.browse.wishes.WishesMessage
import com.slouchcat.core.browse.wishes.WishesTab
import com.slouchcat.core.browse.wishes.WishesViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable

/**
 * The area «Wünsche» (mockup `/wishes`). [tab]: `mine` opens «Meine Wünsche» (a notification
 * about a wish, «Meine Wünsche» of a title's page); [query]: the search of «Entdecken» starts with
 * it (the card «„…“ wünschen» of the search).
 */
@Serializable
data class WishesRoute(val tab: String? = null, val query: String? = null)

/** A title outside the library (mockup `/wish/:i`, web `/titles/:media/:tmdbId`). */
@Serializable
data class WishTitleRoute(val media: String, val tmdbId: Long, val fromSearch: Boolean = false)

/**
 * Stage 8 «Wünsche» (concept 27, 3.1; concept 25, ADR 0037): the target «Wünsche» of the bar
 * (order 40), «Entdecken» and «Meine Wünsche», the page of a title outside the library with
 * «Anfordern» (seasons of a series) and «Zurückziehen», and «Noch nicht in der Bibliothek?» under
 * the search's hits. Rule 11: only request, show the state, withdraw and lead to the title once
 * it is there; approving and declining stay in the web app's administration (the app session
 * has no administration rights). [repository]: tests bring a fake.
 */
class WishesFeature(
    private val repository: (AppGraph) -> WishesRepository = { it.media.wishes },
) : Feature {
    override val destinations = listOf(TopDestination(TARGET, 40, WishesRoute(), R.string.nav_wishes, SlouchcatIcon.Wish))

    /** `wishes` (also `wishes?focus=…` of a notification) and `titles/movie/603` (web addresses). */
    override fun link(address: String): AppLink? {
        val path = address.removePrefix("/").substringBefore('?').trimEnd('/')
        if (path == "wishes") return AppLink(TARGET, listOf(WishesRoute(tab = MINE)))
        val parts = path.split('/')
        if (parts.size == 3 && parts[0] == "titles" && parts[1] in setOf("movie", "show")) {
            val id = parts[2].toLongOrNull()?.takeIf { it > 0 } ?: return null
            return AppLink(TARGET, listOf(WishesRoute(), WishTitleRoute(parts[1], id)))
        }
        return null
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<WishesRoute> { entry ->
            val route = entry.toRoute<WishesRoute>()
            val vm = viewModel {
                WishesViewModel(graph.session, repository(graph), graph.media.openWishesCount, if (route.tab == MINE) WishesTab.Mine else WishesTab.Discover, route.query)
            }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onResume() }
            val failed = stringResource(R.string.app_wishes_failed)
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    is WishesMessage.Withdrawn -> getString(R.string.titles_done_withdrawn)
                    is WishesMessage.Failed -> errorText(message.code) ?: failed
                }
            }
            val events = remember(vm, navigator) { wishesEvents(vm, navigator) }
            WishesScreen(state, events, snackbar)
        }
        composable<WishTitleRoute> { entry ->
            val route = entry.toRoute<WishTitleRoute>()
            val media = if (route.media == "show") ExternalMedia.SHOW else ExternalMedia.MOVIE
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
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            val snackbar = remember { SnackbarHostState() }
            val failed = stringResource(R.string.app_wishes_failed)
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    is WishTitleMessage.Requested -> getString(if (message.joined) R.string.titles_done_joined else R.string.titles_done_requested)
                    WishTitleMessage.Withdrawn -> getString(R.string.titles_done_withdrawn)
                    is WishTitleMessage.Failed -> errorText(message.code) ?: failed
                }
            }
            val events = remember(vm, navigator) {
                object : WishTitleEvents {
                    override fun onBack() = navigator.back()
                    override fun onRetry() = vm.retry()
                    override fun onRequest() = vm.onRequest()
                    override fun onRetrySeasons() = vm.onRetrySeasons()
                    override fun onSeason(number: Int) = vm.onSeason(number)
                    override fun onAllSeasons() = vm.onAllSeasons()
                    override fun onSubmit() = vm.onSubmit()
                    override fun onWithdraw() = vm.onWithdraw()
                    override fun onConfirmWithdraw() = vm.onConfirmWithdraw()
                    override fun onDismiss() = vm.onDismiss()
                    override val onMine: (() -> Unit) = { navigator.openTarget(TARGET, WishesRoute(tab = MINE)) }
                }
            }
            WishTitleScreen(state, images, events, snackbar)
        }
    }

    companion object {
        const val TARGET = "wishes"
        const val MINE = "mine"
    }
}

private fun wishesEvents(vm: WishesViewModel, navigator: Navigator) = object : WishesEvents {
    override fun onTab(tab: WishesTab) = vm.onTab(tab)
    override fun onText(text: String) = vm.onText(text)
    override fun onClear() = vm.onClear()
    override fun onSubmit() = vm.onSubmit()
    override fun onRetry() = vm.retry()
    override fun onMoreResults() = vm.onMoreResults()
    override fun onFilter(filter: WishFilter) = vm.onFilter(filter)
    override fun onMoreWishes() = vm.onMoreWishes()
    override fun onOpenTitle(card: ExternalCard) = openCard(navigator, card)
    override fun onOpenWish(card: WishCard) {
        // «Verfügbar» leads to the title in the library once the profile sees it.
        val item = card.itemId
        if (item != null && navigator.canOpen(ItemRoute::class)) navigator.navigate(ItemRoute(item)) else navigator.navigate(WishTitleRoute(card.media.value, card.tmdbId))
    }
    override fun onAskWithdraw(card: WishCard) = vm.onAskWithdraw(card)
    override fun onConfirmWithdraw() = vm.onConfirmWithdraw()
    override fun onDismissWithdraw() = vm.onDismissWithdraw()
}

/** A tile: its title in the library when the profile sees it there, else its page outside the library. */
private fun openCard(navigator: Navigator, card: ExternalCard, fromSearch: Boolean = false) {
    val item = card.itemId
    if (item != null && navigator.canOpen(ItemRoute::class)) navigator.navigate(ItemRoute(item)) else navigator.navigate(WishTitleRoute(card.media.value, card.tmdbId, fromSearch))
}

/** The text of a problem code of the wishes, `null` for one the app has no own text for. */
private fun android.content.res.Resources.errorText(code: String?): String? = when (code) {
    "tmdb_unreachable" -> getString(R.string.wishes_tmdbUnreachable)
    "seerr_unreachable" -> getString(R.string.errors_code_seerr_unreachable)
    "wish_not_withdrawable" -> getString(R.string.errors_code_wish_not_withdrawable)
    "quota_exceeded" -> getString(R.string.errors_code_quota_exceeded)
    "already_available" -> getString(R.string.errors_code_already_available)
    "wish_not_allowed" -> getString(R.string.errors_code_wish_not_allowed)
    else -> null
}

/**
 * «Noch nicht in der Bibliothek?» under the search's hits (mockup handy 67; web `SearchMissing`):
 * titles the profile may request with their state, then the card «„…“ wünschen» that opens the
 * wishes' own search. Nothing when the server says the profile may not request (rule 12).
 */
@Composable
fun MissingTitles(app: AppScope, query: String, repository: (AppGraph) -> WishesRepository = { it.media.wishes }) {
    val graph = app.graph
    val navigator = app.navigator
    if (!navigator.canOpen(WishTitleRoute::class)) return
    val vm = viewModel(key = "missing-titles") { MissingTitlesViewModel(graph.session, repository(graph)) }
    LaunchedEffect(query) { vm.onQuery(query) }
    val state by vm.state.collectAsStateWithLifecycle()
    if (!state.canRequest || query.trim().length < WishesViewModel.MIN_QUERY) return
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxWidth().padding(horizontal = gutter(), vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(stringResource(R.string.app_wishes_missingTitle), style = SlouchcatType.rowTitle, color = c.tx, modifier = Modifier.semantics { heading() })
        (state.hits as? Loadable.Ready)?.value?.forEach { card ->
            ExternalRow(card, onClick = { openCard(navigator, card, fromSearch = true) })
        }
        if (state.discover) {
            WishSearchCard(query.trim()) { navigator.openTarget(WishesFeature.TARGET, WishesRoute(query = query.trim())) }
        }
    }
}

/** The card «„spider“ wünschen» (mockup handy 67): the wishes' own search with this text. */
@Composable
private fun WishSearchCard(query: String, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(c.s1)
            .border(1.dp, c.frame, RoundedCornerShape(12.dp))
            .clickable(onClick = onClick)
            .semantics(mergeDescendants = true) { role = Role.Button }
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(40.dp).clip(RoundedCornerShape(8.dp)).background(c.accent), contentAlignment = Alignment.Center) {
            SlouchcatIconImage(SlouchcatIcon.Wish, contentDescription = null, tint = c.onAccent, size = 22.dp)
        }
        Column(Modifier.weight(1f)) {
            Text(stringResource(R.string.app_wishes_searchCard, query), style = SlouchcatType.strong, color = c.tx)
            Text(stringResource(R.string.app_wishes_searchCardHint), style = SlouchcatType.caption, color = c.tx2)
        }
        SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, tint = c.tx2, size = 20.dp)
    }
}
