package com.slouchcat.app.tv.search

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.search.SearchEvents
import com.slouchcat.core.browse.search.SearchUiState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/** A library the search offers before anything is typed (mockup «Bibliotheken»). */
data class SearchLibrary(val name: String, val icon: ImageVector, val open: () -> Unit)

/** Where the search leads; `null`: the page is not registered. */
class SearchLinks(
    val item: ((String) -> Unit)?,
    val person: ((Long) -> Unit)?,
    val collection: ((String) -> Unit)?,
    val libraries: List<SearchLibrary>,
    /** The voice input of the device (`RecognizerIntent`); `null` where the TV has none. */
    val voice: (() -> Unit)?,
    /**
     * «Noch nicht in der Bibliothek?» for the query (stage 5, code review m-5): titles to wish and
     * the card «„…“ wünschen»; `null` without the wishes' pages. Draws nothing the profile may not request.
     */
    val missing: (@Composable (query: String, focus: FocusMemory) -> Unit)? = null,
)

private const val RETRY = "retry"

/** The groups of titles, in the mockup's order. */
private val GROUPS = listOf(
    ItemKind.MOVIE to R.string.watched_page_contents_movie,
    ItemKind.SERIES to R.string.watched_page_contents_series,
    ItemKind.EPISODE to R.string.item_episodes_title,
    ItemKind.AUDIOBOOK to R.string.watched_page_contents_audiobook,
    ItemKind.PODCAST to R.string.watched_page_contents_podcast,
)

/**
 * The search (mockup `/search`, `browse.js`): the own keyboard on the left (the TV's field needs no
 * system keyboard, which covers half the screen), the voice input of the device where it has one,
 * and on the right the hits as rows (titles by kind, people, collections) or, with nothing typed,
 * the libraries and the genres. A keyboard plugged into the TV types as well.
 */
@Composable
fun SearchScreen(state: SearchUiState, images: ServerImages, links: SearchLinks, events: SearchEvents) {
    val focus = rememberFocusMemory()
    // The device's input (code review m-4): every script the own keyboard does not have.
    val mode = rememberTypingMode()
    Row(
        Modifier
            .fillMaxSize()
            .padding(start = LocalContentStart.current, top = TvDimens.safeY)
            .hardwareTyping(mode, { state.text }, events::onText),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.xxl),
    ) {
        TvTyping(
            text = state.text,
            onText = events::onText,
            onClear = events::onClear,
            focus = focus,
            mode = mode,
            placeholder = stringResource(R.string.app_search_placeholder),
            title = { Text(stringResource(R.string.nav_search), style = TvType.h1, modifier = Modifier.semantics { heading() }) },
            voice = links.voice,
        )
        CompositionLocalProvider(LocalContentStart provides 0.dp) {
            Results(state, images, links, events, focus, Modifier.weight(1f))
        }
    }
    focus.StartFocus(TYPING_START)
}

@Composable
private fun Results(state: SearchUiState, images: ServerImages, links: SearchLinks, events: SearchEvents, focus: FocusMemory, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    LazyColumn(modifier.fillMaxSize(), contentPadding = PaddingValues(top = TvDimens.Browse.HeadHeight, bottom = TvDimens.Browse.DetailTop)) {
        val genre = state.genre
        if (genre != null) {
            item(key = "genre") {
                Row(Modifier.padding(bottom = TvDimens.m)) {
                    ControlChip(stringResource(R.string.app_search_genreFilter, genre), SlouchcatIcon.Close.vector, onClick = events::onClearGenre, on = true, modifier = Modifier.focusKey(focus, "genre"))
                }
            }
        }
        when (val hits = state.hits) {
            null -> {
                if (state.tooShort) item(key = "short") { Text(stringResource(R.string.search_tooShort), style = TvType.large, color = c.tx2) }
                if (links.libraries.isNotEmpty()) {
                    item(key = "libraries") {
                        TvCardRow(title = stringResource(R.string.tv_search_libraries), items = links.libraries, key = { it.name }, focus = focus, rowKey = "lib") { library, m ->
                            ControlChip(library.name, library.icon, onClick = library.open, modifier = m)
                        }
                    }
                }
                val genres = (state.genres as? Loadable.Ready)?.value.orEmpty()
                if (genres.isNotEmpty()) {
                    item(key = "genres") {
                        TvCardRow(title = stringResource(R.string.app_search_genres), items = genres, key = { it.name }, focus = focus, rowKey = "gen") { g, m ->
                            ControlChip(g.name, SlouchcatIcon.Filter.vector, onClick = { events.onGenre(g.name) }, modifier = m)
                        }
                    }
                }
            }
            Loadable.Empty -> item(key = "none") {
                EmptyState(SlouchcatIcon.Search.vector, stringResource(R.string.search_empty_title), if (state.query.isNotEmpty()) stringResource(R.string.app_search_emptyBody, state.query) else null)
            }
            is Loadable.Ready -> {
                val found = hits.value
                item(key = "count") {
                    val count = pluralCount(R.plurals.app_search_hits, found.count)
                    Text(if (found.query.isNotEmpty()) stringResource(R.string.tv_search_hitsFor, count, found.query) else count, style = TvType.h2, modifier = Modifier.padding(bottom = TvDimens.l))
                }
                for ((kind, label) in GROUPS) {
                    val group = found.items.filter { it.kind == kind }
                    if (group.isEmpty()) continue
                    item(key = "kind-${kind.value}") {
                        val infos = group.associate { it.id to it.info() }
                        TvCardRow(
                            title = "${stringResource(label)} · ${group.size}",
                            items = group,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-${kind.value}",
                            info = { infos[it.id] },
                        ) { item, m ->
                            MediaCard(item, links.item?.let { open -> { open(item.id) } }, m.nearEnd(item, found.items, events))
                        }
                    }
                }
                if (found.people.isNotEmpty()) {
                    item(key = "people") {
                        TvCardRow(
                            title = "${stringResource(R.string.search_sections_people)} · ${found.people.size}",
                            items = found.people,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-people",
                            info = { CardInfo(it.name) },
                        ) { person, m ->
                            TvCard(person.name, images.single(person.image).forWidth(TvDimens.Browse.CastWidth), onClick = { links.person?.invoke(person.id) }, modifier = m, caption = person.name, width = TvDimens.Browse.CastWidth)
                        }
                    }
                }
                if (found.collections.isNotEmpty()) {
                    item(key = "collections") {
                        TvCardRow(
                            title = "${stringResource(R.string.search_sections_collections)} · ${found.collections.size}",
                            items = found.collections,
                            key = { it.id },
                            focus = focus,
                            rowKey = "sr-col",
                            info = { CardInfo(it.name) },
                        ) { col, m ->
                            val cover = col.cover?.let { MediaItem.from(it, images) }
                            TvCard(col.name, images.single(col.image).or(cover?.poster ?: images.single(null)).forWidth(com.slouchcat.app.tv.ui.CardShape.Poster.width), onClick = { links.collection?.invoke(col.id) }, modifier = m)
                        }
                    }
                }
            }
            else -> item(key = "failed") { LoadableContent(hits, onRetry = events::retry, retryModifier = Modifier.focusKey(focus, RETRY)) {} }
        }
        // After the hits, also after «nichts gefunden»: what is not in the library yet (code review m-5).
        val missing = links.missing
        val hits = state.hits
        if (missing != null && state.query.isNotEmpty() && (hits is Loadable.Ready || hits is Loadable.Empty)) {
            item(key = "missing") { missing(state.query, focus) }
        }
    }
}

/** The titles come in pages: near the last loaded card the next page is asked for. */
private fun Modifier.nearEnd(item: MediaItem, all: List<MediaItem>, events: SearchEvents): Modifier =
    onFocusChanged { if (it.isFocused && all.indexOf(item) >= all.size - NEAR_END) events.onMore() }

private const val NEAR_END = 6
