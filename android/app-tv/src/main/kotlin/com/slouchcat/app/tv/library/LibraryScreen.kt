package com.slouchcat.app.tv.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.PageHead
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.watchedTag
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvCardGrid
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvRowInfo
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.strings.R

/** What the library page can be asked to do. */
class LibraryActions(
    val retry: () -> Unit,
    val nearEnd: () -> Unit,
    val sheet: (LibrarySheet?) -> Unit,
    val sort: (LibrarySort) -> Unit,
    val all: () -> Unit,
    val unseen: () -> Unit,
    val genre: (String?) -> Unit,
    val decade: (Decade?) -> Unit,
    /** A card: its detail page; `null` while no feature registered it. */
    val item: ((MediaItem) -> Unit)?,
)

/** The symbol of a library in the rail and on its page: a film strip, a TV for series, a headphone for audio. */
fun libraryIcon(library: LibraryNav): ImageVector = when {
    library.kind == LibraryKind.AUDIOBOOKS -> SlouchcatIcon.Headphones.vector
    library.kind == LibraryKind.PODCASTS -> SlouchcatIcon.Mic.vector
    library.contents.isNotEmpty() && library.contents.all { it == RootContent.SERIES } -> SlouchcatIcon.Tv.vector
    else -> SlouchcatIcon.Film.vector
}

/** The name of a sort order. */
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

/** «2010er», «Vor 1960». */
@Composable
fun decadeName(decade: Decade): String =
    if (decade.before) stringResource(R.string.app_browse_before, decade.start.toString()) else stringResource(R.string.app_browse_decade, decade.start.toString())

private const val GRID = "grid"
private const val RETRY = "retry"
private const val EMPTY = "empty"
private const val SHOW = "lf-show"

/** Posters in a row of the grid (`.br-grid`), squares for audio (`.br-grid.sq`). */
private const val COLUMNS = 7
private const val COLUMNS_SQUARE = 6

/**
 * A library (mockup `/library/:key`, `browse.js`): head with symbol, name and count, the chips
 * «Alle/Ungesehen», «Genre», «Jahr» and the order, each opening a side menu, «Zurücksetzen» while
 * a filter is on, the line with the focused title, and the grid in pages from the server. Up from
 * the grid's first row goes to the chips; a closed menu gives the focus back to its chip.
 */
@Composable
fun LibraryScreen(state: LibraryUiState, actions: LibraryActions) {
    val focus = rememberFocusMemory()
    val filters = state.filters
    var focused by remember { mutableStateOf<MediaItem?>(null) }
    val noun = if (state.audio) R.string.library_status_unheard else R.string.library_status_unwatched
    Column(Modifier.fillMaxSize()) {
        val library = state.library
        Column(Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.safeY)) {
            PageHead(
                library?.let(::libraryIcon) ?: SlouchcatIcon.Film.vector,
                library?.name.orEmpty(),
                library?.let { pluralCount(R.plurals.home_list_count, it.itemCount) },
            )
            Row(Modifier.padding(top = TvDimens.Browse.ControlsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
                ControlChip(
                    stringResource(if (filters.unseen) noun else R.string.app_browse_all),
                    SlouchcatIcon.Eye.vector,
                    onClick = { actions.sheet(LibrarySheet.Show) },
                    on = filters.unseen,
                    menu = true,
                    modifier = Modifier.focusKey(focus, SHOW),
                )
                ControlChip(
                    filters.genre ?: stringResource(R.string.library_filters_genre),
                    SlouchcatIcon.Filter.vector,
                    onClick = { actions.sheet(LibrarySheet.Genre) },
                    on = filters.genre != null,
                    menu = true,
                    modifier = Modifier.focusKey(focus, "lf-genre"),
                )
                if (!state.podcasts) {
                    ControlChip(
                        filters.decade?.let { decadeName(it) } ?: stringResource(R.string.library_filters_year),
                        SlouchcatIcon.Clock.vector,
                        onClick = { actions.sheet(LibrarySheet.Year) },
                        on = filters.decade != null,
                        menu = true,
                        modifier = Modifier.focusKey(focus, "lf-year"),
                    )
                }
                ControlChip(
                    sortName(filters.sort),
                    SlouchcatIcon.Sort.vector,
                    onClick = { actions.sheet(LibrarySheet.Sort) },
                    on = filters.sort != LibrarySort.ADDED,
                    menu = true,
                    modifier = Modifier.focusKey(focus, "lf-sort"),
                )
                if (filters.active) {
                    ControlChip(
                        stringResource(R.string.library_page_clear),
                        SlouchcatIcon.Close.vector,
                        onClick = {
                            actions.all()
                            focus.focus(SHOW)
                        },
                        ghost = true,
                        modifier = Modifier.focusKey(focus, "lf-reset"),
                    )
                }
            }
        }
        val summary = CardInfo(
            if (filters.active && state.items.items is Loadable.Ready) pluralCount(R.plurals.app_browse_matches, state.items.total) else stringResource(R.string.app_browse_all),
            sortName(filters.sort),
        )
        TvRowInfo(focused?.info() ?: summary, visible = true, modifier = Modifier.padding(top = TvDimens.l))
        when (val items = state.items.items) {
            is Loadable.Ready -> Grid(items.value, state, actions, focus) { focused = it }
            Loadable.Empty -> EmptyState(
                SlouchcatIcon.Filter.vector,
                stringResource(if (filters.active) R.string.library_page_emptyFilters else R.string.home_list_empty),
                null,
                Modifier.padding(start = LocalContentStart.current),
                action = if (filters.active) ({ EmptyAction(stringResource(R.string.app_browse_reset), SlouchcatIcon.Refresh.vector, { actions.all(); focus.focus(SHOW) }, Modifier.focusKey(focus, EMPTY)) }) else null,
            )
            else -> LoadableContent(items, onRetry = actions.retry, modifier = Modifier.padding(start = LocalContentStart.current), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        }
    }
    val start = when (val items = state.items.items) {
        is Loadable.Ready -> "$GRID-${items.value.first().id}"
        is Loadable.Failed -> RETRY
        Loadable.Empty -> if (filters.active) EMPTY else SHOW
        Loadable.Loading -> SHOW
    }
    focus.StartFocus(start, state.items.items::class, state.filters)
    state.sheet?.let { sheet -> LibraryMenu(sheet, state, actions, focus) }
}

@Composable
private fun Grid(items: List<MediaItem>, state: LibraryUiState, actions: LibraryActions, focus: FocusMemory, onFocused: (MediaItem?) -> Unit) {
    val index = remember(items) { items.withIndex().associate { it.value.id to it.index } }
    val columns = if (state.audio) COLUMNS_SQUARE else COLUMNS
    TvCardGrid(
        items = items,
        key = { it.id },
        focus = focus,
        gridKey = GRID,
        columns = columns,
        modifier = Modifier.fillMaxWidth().heightIn(min = TvDimens.Browse.HeadHeight).onFocusChanged { if (!it.hasFocus) onFocused(null) },
    ) { item, modifier ->
        MediaCard(
            item,
            actions.item?.let { open -> { open(item) } },
            modifier.onFocusChanged {
                if (it.isFocused) {
                    onFocused(item)
                    if ((index[item.id] ?: 0) >= items.size - columns * 2) actions.nearEnd()
                }
            },
            tag = watchedTag(item),
            width = null,
        )
    }
    if (state.items.more == MoreState.Failed) {
        // The next page failed: OK on the last card asks again (the grid has no room for a button).
        Text(stringResource(R.string.library_page_pageFailed), modifier = Modifier.padding(start = LocalContentStart.current))
    }
}

/** The side menus of the chips (mockup `brLibMenu`): one list, the current choice checked and focused. */
@Composable
private fun LibraryMenu(sheet: LibrarySheet, state: LibraryUiState, actions: LibraryActions, focus: FocusMemory) {
    val first = remember { FocusRequester() }
    val close = {
        actions.sheet(null)
        focus.restore()
    }
    val filters = state.filters
    val title = when (sheet) {
        LibrarySheet.Show -> stringResource(R.string.tv_library_show)
        LibrarySheet.Sort -> stringResource(R.string.app_browse_sortTitle)
        LibrarySheet.Genre -> stringResource(R.string.library_filters_genre)
        LibrarySheet.Year -> stringResource(R.string.app_browse_yearTitle)
        LibrarySheet.Rating -> stringResource(R.string.library_filters_rating)
    }
    TvSideMenu(title, onDismiss = close, initialFocus = first) {
        @Composable
        fun option(text: String, current: Boolean, subtitle: String? = null, onClick: () -> Unit) = TvMenuItem(
            text,
            onClick = {
                onClick()
                focus.restore()
            },
            subtitle = subtitle,
            selected = current,
            modifier = if (current) Modifier.focusRequester(first) else Modifier,
        )
        when (sheet) {
            LibrarySheet.Show -> {
                option(stringResource(R.string.app_browse_all), !filters.unseen) { if (filters.unseen) actions.unseen() else actions.sheet(null) }
                option(stringResource(if (state.audio) R.string.library_status_unheard else R.string.library_status_unwatched), filters.unseen) {
                    if (!filters.unseen) actions.unseen() else actions.sheet(null)
                }
            }
            LibrarySheet.Sort -> state.sorts.forEach { sort -> option(sortName(sort), filters.sort == sort) { actions.sort(sort) } }
            LibrarySheet.Genre -> {
                option(stringResource(R.string.library_filters_allGenres), filters.genre == null) { actions.genre(null) }
                state.genres.forEach { genre -> option(genre.name, filters.genre == genre.name, pluralCount(R.plurals.home_list_count, genre.count)) { actions.genre(genre.name) } }
            }
            LibrarySheet.Year -> {
                option(stringResource(R.string.app_browse_allYears), filters.decade == null) { actions.decade(null) }
                state.decades.forEach { decade -> option(decadeName(decade), filters.decade == decade) { actions.decade(decade) } }
            }
            LibrarySheet.Rating -> Unit
        }
    }
}
