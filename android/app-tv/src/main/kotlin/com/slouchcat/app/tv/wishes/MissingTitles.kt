package com.slouchcat.app.tv.wishes

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.shell.TvNavigator
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.core.browse.wishes.MissingTitlesViewModel
import com.slouchcat.core.browse.wishes.WishesViewModel
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.strings.R

/** The key of the card «„…“ wünschen» in the row (a title has `media-tmdbId`). */
private const val WISH_SEARCH = "wish-search"

/**
 * «Noch nicht in der Bibliothek?» under the hits of the main search (code review m-5; phone app and
 * web `SearchMissing`): the titles the profile may request with their state as a row, then the card
 * «„…“ wünschen» that opens the wishes' own search with the query. The shared
 * [MissingTitlesViewModel] asks only when the server says the profile may request (rule 12); without
 * that the row is not there. The cards are known to the search's [focus] as `miss-<key>`.
 */
@Composable
fun MissingTitles(
    graph: TvGraph,
    navigator: TvNavigator,
    query: String,
    focus: FocusMemory,
    repository: (TvGraph) -> WishesRepository = { it.media.wishes },
) {
    val vm = viewModel(key = "missing-titles") { MissingTitlesViewModel(graph.session, repository(graph)) }
    LaunchedEffect(query) { vm.onQuery(query) }
    val state by vm.state.collectAsStateWithLifecycle()
    val asked = query.trim()
    if (!state.canRequest || asked.length < WishesViewModel.MIN_QUERY) return
    val hits = (state.hits as? Loadable.Ready)?.value.orEmpty()
    // `null` is the card «„…“ wünschen», only where the wishes' search is there (`discover`).
    val items: List<ExternalCard?> = hits + if (state.discover) listOf(null) else emptyList()
    if (items.isEmpty()) return
    val infos = hits.associate { it.key to it.info() }
    val searchCard = stringResource(R.string.app_wishes_searchCard, asked)
    val searchHint = stringResource(R.string.app_wishes_searchCardHint)
    TvCardRow(
        title = stringResource(R.string.app_wishes_missingTitle),
        items = items,
        key = { it?.key ?: WISH_SEARCH },
        focus = focus,
        rowKey = "miss",
        info = { card -> card?.let { infos[it.key] } ?: CardInfo(searchCard, searchHint) },
    ) { card, modifier ->
        if (card != null) {
            ExternalTile(card, { openCard(navigator, card, fromSearch = true) }, modifier)
        } else {
            TvCard(
                title = listOf(searchCard, searchHint).joinToString(stringResource(R.string.format_listSeparator)),
                image = null,
                onClick = { navigator.navigate(WishesRoute(tab = WishesFeature.SEARCH, query = asked)) },
                modifier = modifier,
                caption = searchCard,
                subcaption = searchHint,
            )
        }
    }
}
