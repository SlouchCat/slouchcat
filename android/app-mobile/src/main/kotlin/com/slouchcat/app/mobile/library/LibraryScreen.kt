package com.slouchcat.app.mobile.library

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaGrid
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.fullWidth
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.app.mobile.media.watchedTag
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.strings.R
import java.text.NumberFormat

/** What the library page does on a tap; a `null` target has no page yet. */
data class LibraryActions(
    val back: () -> Unit = {},
    val retry: () -> Unit = {},
    val nearEnd: () -> Unit = {},
    val sheet: (LibrarySheet?) -> Unit = {},
    val sort: (LibrarySort) -> Unit = {},
    val all: () -> Unit = {},
    val unseen: () -> Unit = {},
    val genre: (String?) -> Unit = {},
    val decade: (Decade?) -> Unit = {},
    val content: (RootContent?) -> Unit = {},
    val item: ((MediaItem) -> Unit)? = null,
    val search: (() -> Unit)? = null,
)

/**
 * A library (mockup `/library/:key`, reference photos 12 to 15 and the sheets 49 to 51): bar with
 * back and search, the filter chips, the count and the order, the grid in pages; square cards for
 * audiobooks and podcasts. Sheets from below (centred from 700 dp) for order, genre and year.
 */
@Composable
fun LibraryScreen(state: LibraryUiState, actions: LibraryActions, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val filters = state.filters
    val items = state.items
    Column(modifier.fillMaxSize().background(c.bg)) {
        TopBar(
            title = state.library?.name.orEmpty(),
            navigationLabel = stringResource(R.string.common_back),
            onNavigation = actions.back,
            actions = {
                actions.search?.let { SlouchcatIconButton(SlouchcatIcon.Search, contentDescription = stringResource(R.string.nav_search), onClick = it) }
            },
        )
        val grid = rememberLazyGridState()
        MediaGrid(
            items = items.loaded,
            key = { it.id },
            state = grid,
            onNearEnd = actions.nearEnd,
            modifier = Modifier.fillMaxSize(),
            header = {
                // One cell: the grid's gap stays between the head and the cards, not inside the head.
                fullWidth("head") {
                    Column {
                        Chips(state, actions)
                        Tools(state, actions)
                    }
                }
                if (items.items !is Loadable.Ready) {
                    fullWidth("state") {
                        when {
                            items.items is Loadable.Empty && filters.active -> MediaEmpty(
                                stringResource(R.string.app_browse_noMatch),
                                body = stringResource(R.string.library_page_emptyFilters),
                                icon = SlouchcatIcon.Filter,
                                actionLabel = stringResource(R.string.app_browse_reset),
                                action = actions.all,
                            )
                            items.items is Loadable.Empty -> MediaEmpty(stringResource(R.string.library_notFound_title), body = stringResource(R.string.library_notFound_body), icon = SlouchcatIcon.Film)
                            else -> LoadableContent(items.items, onRetry = actions.retry) {}
                        }
                    }
                }
            },
            footer = { fullWidth("more") { PagingFooter(items.more, onRetry = actions.nearEnd) } },
        ) { item ->
            MediaCard(item, onClick = actions.item?.let { { it(item) } }, tag = watchedTag(item), width = null)
        }
    }
    when (state.sheet) {
        LibrarySheet.Sort -> SortSheet(state, actions)
        LibrarySheet.Genre -> GenreSheet(state, actions)
        LibrarySheet.Year -> YearSheet(state, actions)
        null -> Unit
    }
}

/** `.bx-chips`: Alle, the content kinds of a mixed library, Ungesehen, Genre, Jahr (not for podcasts). */
@Composable
private fun Chips(state: LibraryUiState, actions: LibraryActions) {
    val f = state.filters
    val contents = state.library?.contents.orEmpty()
    LazyRow(
        // The grid has the side margin already.
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        modifier = Modifier.fillMaxWidth().padding(top = 2.dp, bottom = 4.dp),
    ) {
        item { SlouchcatChip(stringResource(R.string.app_browse_all), selected = !f.active, onClick = actions.all) }
        if (contents.size > 1) {
            items(contents.size) { index ->
                val content = contents[index]
                SlouchcatChip(contentName(content), selected = f.content == content, onClick = { actions.content(if (f.content == content) null else content) })
            }
        }
        item {
            SlouchcatChip(
                stringResource(if (state.audio) R.string.library_status_unheard else R.string.library_status_unwatched),
                selected = f.unseen,
                onClick = actions.unseen,
                icon = if (f.unseen) SlouchcatIcon.Check else null,
            )
        }
        if (state.genres.isNotEmpty() || f.genre != null) {
            item {
                SlouchcatChip(f.genre ?: stringResource(R.string.library_filters_genre), selected = f.genre != null, onClick = { actions.sheet(LibrarySheet.Genre) }, trailingIcon = SlouchcatIcon.Down)
            }
        }
        if (!state.podcasts) {
            item {
                SlouchcatChip(f.decade?.let { decadeName(it) } ?: stringResource(R.string.library_filters_year), selected = f.decade != null, onClick = { actions.sheet(LibrarySheet.Year) }, trailingIcon = SlouchcatIcon.Down)
            }
        }
    }
}

@Composable
private fun contentName(content: RootContent): String = stringResource(
    when (content) {
        RootContent.SERIES -> R.string.library_content_series
        RootContent.AUDIOBOOK -> R.string.library_content_audiobook
        RootContent.PODCAST -> R.string.library_content_podcast
        else -> R.string.library_content_movie
    },
)

/** «2010er», «Vor 1960». */
@Composable
fun decadeName(decade: Decade): String =
    if (decade.before) stringResource(R.string.app_browse_before, decade.start.toString()) else stringResource(R.string.app_browse_decade, decade.start.toString())

/** The name of an order (the mockup's «Titel A–Z» for the title). */
@Composable
fun sortName(sort: LibrarySort): String = stringResource(
    when (sort) {
        LibrarySort.TITLE -> R.string.app_browse_sortTitleAz
        LibrarySort.RELEASED -> R.string.library_sort_released
        LibrarySort.VOTE -> R.string.library_sort_vote
        LibrarySort.RUNTIME -> R.string.library_sort_runtime
        LibrarySort.AGE -> R.string.library_sort_age
        LibrarySort.PLAYED -> R.string.library_sort_played
        LibrarySort.AUTHOR -> R.string.library_sort_author
        else -> R.string.library_sort_added
    },
)

/** The direction an order has by default on the server, as a second line of its option. */
@Composable
private fun sortHint(sort: LibrarySort): String? = when (sort) {
    LibrarySort.ADDED, LibrarySort.RELEASED -> stringResource(R.string.app_browse_order_newest)
    LibrarySort.VOTE -> stringResource(R.string.app_browse_order_best)
    LibrarySort.RUNTIME -> stringResource(R.string.app_browse_order_longest)
    LibrarySort.PLAYED -> stringResource(R.string.app_browse_order_lastSeen)
    LibrarySort.AUTHOR -> stringResource(R.string.app_browse_order_aToZ)
    LibrarySort.AGE -> stringResource(R.string.app_browse_order_youngest)
    else -> null
}

/** `.bx-tools`: «1846 Titel» (or «12 Treffer» while filtered) and the order. */
@Composable
private fun Tools(state: LibraryUiState, actions: LibraryActions) {
    val c = SlouchcatTheme.colors
    val total = state.items.total
    val count = when {
        state.items.items !is Loadable.Ready -> null
        state.filters.active -> pluralStringResource(R.plurals.app_browse_matches, total, total)
        else -> {
            val noun = when (state.library?.kind) {
                LibraryKind.AUDIOBOOKS -> pluralStringResource(R.plurals.library_page_noun_audiobooks, total)
                LibraryKind.PODCASTS -> pluralStringResource(R.plurals.library_page_noun_podcasts, total)
                else -> pluralStringResource(R.plurals.library_page_noun_video, total)
            }
            pluralStringResource(R.plurals.library_page_count, total, total, noun).replace(total.toString(), NumberFormat.getIntegerInstance().format(total))
        }
    }
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(
            count.orEmpty(),
            style = SlouchcatType.bodySmall.copy(fontSize = SlouchcatType.chip.fontSize),
            color = c.tx3,
            modifier = Modifier.weight(1f).semantics { liveRegion = LiveRegionMode.Polite },
        )
        val name = sortName(state.filters.sort)
        val label = stringResource(R.string.app_browse_sortLabel, name)
        Row(
            Modifier
                .clip(RoundedCornerShape(24.dp))
                .clickable(role = Role.Button, onClick = { actions.sheet(LibrarySheet.Sort) })
                .semantics(mergeDescendants = true) { contentDescription = label }
                .heightIn(min = 48.dp)
                .padding(horizontal = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            SlouchcatIconImage(SlouchcatIcon.Sort, contentDescription = null, size = 18.dp, tint = c.tx2)
            Text(name, style = SlouchcatType.buttonSmall, color = c.tx2)
        }
    }
}

/** `.bx-opt`: one choice of a sheet, a radio with the check mark in the accent. */
@Composable
private fun Option(title: String, selected: Boolean, onClick: () -> Unit, subtitle: String? = null) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(role = Role.RadioButton, onClick = onClick)
            .semantics(mergeDescendants = true) { this.selected = selected }
            .heightIn(min = 56.dp)
            .padding(horizontal = gutter(), vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle.copy(fontWeight = FontWeight.SemiBold), color = if (selected) c.accent else c.tx)
            if (subtitle != null) Text(subtitle, style = SlouchcatType.caption, color = c.tx3)
        }
        if (selected) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 22.dp, tint = c.accent)
    }
}

@Composable
private fun SheetClose(actions: LibraryActions) {
    Box(Modifier.fillMaxWidth().padding(start = gutter(), end = gutter(), top = 14.dp)) {
        SlouchcatButton(stringResource(R.string.common_close), onClick = { actions.sheet(null) }, kind = ButtonKind.Ghost, block = true)
    }
}

@Composable
private fun SheetHead(title: String) {
    Text(title, style = SlouchcatType.sheetTitle, color = SlouchcatTheme.colors.tx, modifier = Modifier.padding(start = gutter(), end = gutter(), top = 4.dp, bottom = 10.dp))
}

@Composable
private fun SortSheet(state: LibraryUiState, actions: LibraryActions) {
    val title = stringResource(R.string.app_browse_sortTitle)
    SlouchcatSheet(title, onDismiss = { actions.sheet(null) }) {
        SheetHead(title)
        for (sort in state.sorts) Option(sortName(sort), state.filters.sort == sort, { actions.sort(sort) }, sortHint(sort))
        SheetClose(actions)
    }
}

@Composable
private fun GenreSheet(state: LibraryUiState, actions: LibraryActions) {
    val title = stringResource(R.string.library_filters_genre)
    SlouchcatSheet(title, onDismiss = { actions.sheet(null) }) {
        SheetHead(title)
        Option(stringResource(R.string.library_filters_allGenres), state.filters.genre == null, { actions.genre(null) }, state.library?.itemCount?.let { titleCount(it) })
        for (genre in state.genres) Option(genre.name, state.filters.genre == genre.name, { actions.genre(genre.name) }, titleCount(genre.count))
        SheetClose(actions)
    }
}

@Composable
private fun YearSheet(state: LibraryUiState, actions: LibraryActions) {
    val title = stringResource(R.string.app_browse_yearTitle)
    SlouchcatSheet(title, onDismiss = { actions.sheet(null) }) {
        SheetHead(title)
        Option(stringResource(R.string.app_browse_allYears), state.filters.decade == null, { actions.decade(null) })
        for (decade in state.decades) Option(decadeName(decade), state.filters.decade == decade, { actions.decade(decade) })
        SheetClose(actions)
    }
}
