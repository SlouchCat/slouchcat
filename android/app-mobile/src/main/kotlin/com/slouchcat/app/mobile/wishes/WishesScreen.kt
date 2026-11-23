package com.slouchcat.app.mobile.wishes

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.MediaRow
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.pluralCount
import com.slouchcat.app.mobile.shell.ShellTopBar
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.browse.wishes.WishesTab
import com.slouchcat.core.browse.wishes.WishesUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTextField
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** What the area «Wünsche» does on a tap. */
interface WishesEvents {
    fun onTab(tab: WishesTab)
    fun onText(text: String)
    fun onClear()
    fun onSubmit()
    fun onRetry()
    fun onMoreResults()
    fun onFilter(filter: WishFilter)
    fun onMoreWishes()
    fun onOpenTitle(card: ExternalCard)
    fun onOpenWish(card: WishCard)
    fun onAskWithdraw(card: WishCard)
    fun onConfirmWithdraw()
    fun onDismissWithdraw()
}

/**
 * The area «Wünsche» (mockup `/wishes`, reference photos handy 40 and 64): the tabs «Entdecken»
 * and «Meine Wünsche» with the number of the profile's wishes; «Entdecken» with the search field,
 * the note that a wish goes to Seerr and the rows of Seerr; «Meine Wünsche» with the filter and the
 * wishes with their state. Wishes off or Seerr not set up: the page says so (the web app hides its
 * entry; the bar of the app has its five targets fixed).
 */
@Composable
fun WishesScreen(state: WishesUiState, events: WishesEvents, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            ShellTopBar(stringResource(R.string.nav_wishes))
            when (val rights = state.rights) {
                is Loadable.Ready -> {
                    if (state.discover) Tabs(state, events::onTab)
                    when (state.shownTab) {
                        WishesTab.Discover -> Discover(state, events)
                        WishesTab.Mine -> Mine(state, events)
                    }
                }
                Loadable.Empty -> Box(Modifier.padding(horizontal = gutter())) {
                    MediaEmpty(stringResource(R.string.app_wishes_off_title), body = stringResource(R.string.app_wishes_off_body), icon = SlouchcatIcon.Wish)
                }
                else -> LoadableContent(rights, onRetry = events::onRetry, modifier = Modifier.padding(horizontal = gutter())) {}
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
    val asking = state.withdrawing
    if (asking != null) {
        val title = stringResource(if (asking.joined) R.string.wishes_withdraw_titleJoined else R.string.wishes_withdraw_title)
        SlouchcatSheet(title, onDismiss = events::onDismissWithdraw) {
            SheetHeadIcon(SlouchcatIcon.Wish, warning = true)
            SheetTitle(title)
            SheetText(
                if (asking.joined) stringResource(R.string.wishes_withdraw_bodyJoined, asking.title) else stringResource(R.string.app_wishes_withdrawBody, asking.title),
            )
            SheetActions {
                SlouchcatButton(stringResource(R.string.app_wishes_keep), onClick = events::onDismissWithdraw, kind = ButtonKind.Secondary, modifier = Modifier.weight(1f))
                SlouchcatButton(
                    stringResource(if (asking.joined) R.string.wishes_withdraw_confirmJoined else R.string.wishes_withdraw_confirm),
                    onClick = events::onConfirmWithdraw,
                    kind = ButtonKind.Danger,
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

/** The two tabs under the bar (mockup `.tabs`): the chosen one with the accent line. */
@Composable
private fun Tabs(state: WishesUiState, onTab: (WishesTab) -> Unit) {
    val c = SlouchcatTheme.colors
    val open = state.counts?.open ?: 0
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = gutter())
            .semantics { },
    ) {
        listOf(
            WishesTab.Discover to stringResource(R.string.wishes_area_discover),
            WishesTab.Mine to stringResource(R.string.wishes_area_mine),
        ).forEach { (tab, label) ->
            val selected = state.shownTab == tab
            Column(
                Modifier
                    .weight(1f)
                    .heightIn(min = 48.dp)
                    .clickable { onTab(tab) }
                    .semantics {
                        role = Role.Tab
                        this.selected = selected
                    },
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Row(Modifier.padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(label, style = SlouchcatType.button, color = if (selected) c.tx else c.tx2)
                    if (tab == WishesTab.Mine && open > 0) {
                        Box(Modifier.clip(RoundedCornerShape(10.dp)).background(c.s3).padding(horizontal = 7.dp, vertical = 1.dp)) {
                            Text(open.toString(), style = SlouchcatType.badge, color = c.tx)
                        }
                    }
                }
                Box(Modifier.fillMaxWidth().height(3.dp).background(if (selected) c.accent else c.line))
            }
        }
    }
}

@Composable
private fun Discover(state: WishesUiState, events: WishesEvents) {
    val c = SlouchcatTheme.colors
    // A plain column, not a lazy list: a handful of rows and one page of hits at a time.
    val scroll = rememberScrollState()
    val results = state.results
    if (results != null && results.hasMore) {
        val nearEnd by remember(scroll) { derivedStateOf { scroll.maxValue > 0 && scroll.value >= scroll.maxValue - NEAR_END_PX } }
        LaunchedEffect(scroll, results.loaded.size) { snapshotFlow { nearEnd }.distinctUntilChanged().filter { it }.collect { events.onMoreResults() } }
    }
    Column(Modifier.fillMaxSize().imePadding().verticalScroll(scroll)) {
        Column(Modifier.padding(start = gutter(), end = gutter(), top = 16.dp).then(if (isWideLayout()) Modifier.widthIn(max = 720.dp) else Modifier.fillMaxWidth())) {
            SlouchcatTextField(
                value = state.text,
                onValueChange = events::onText,
                label = stringResource(R.string.wishes_discover_searchLabel),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                keyboardActions = KeyboardActions(onSearch = { events.onSubmit() }),
                trailing = {
                    if (state.text.isNotEmpty()) {
                        SlouchcatIconButton(SlouchcatIcon.Close, contentDescription = stringResource(R.string.app_search_clear), onClick = events::onClear)
                    } else {
                        SlouchcatIconButton(SlouchcatIcon.Search, contentDescription = stringResource(R.string.wishes_discover_searchAction), onClick = events::onSubmit)
                    }
                },
            )
            Text(stringResource(R.string.app_wishes_discoverIntro), style = SlouchcatType.caption, color = c.tx2, modifier = Modifier.padding(top = 8.dp, bottom = 8.dp))
        }
        if (results != null) {
            SearchResults(results.items, state.text, events)
            if (results.more != MoreState.Idle) Box(Modifier.padding(horizontal = gutter())) { PagingFooter(results.more, events::onMoreResults) }
        } else {
            if (state.stale) Note(stringResource(R.string.app_wishes_stale), Modifier.padding(horizontal = gutter(), vertical = 8.dp))
            when (val rows = state.rows) {
                is Loadable.Ready -> rows.value.forEach { row ->
                    MediaRow(rowTitle(row.key), onShowAll = null, modifier = Modifier.padding(top = 12.dp)) {
                        items(row.cards, key = { it.key }) { card -> ExternalPosterCard(card, onClick = { events.onOpenTitle(card) }) }
                    }
                }
                Loadable.Empty -> MediaEmpty(stringResource(R.string.wishes_discover_empty), modifier = Modifier.padding(horizontal = gutter()))
                is Loadable.Failed -> if (rows.error == com.slouchcat.core.media.LoadError.NotFound) {
                    MediaEmpty(
                        stringResource(R.string.wishes_discover_gone_title),
                        body = stringResource(R.string.wishes_discover_gone_body),
                        actionLabel = stringResource(R.string.wishes_discover_gone_action),
                        action = { events.onTab(WishesTab.Mine) },
                        modifier = Modifier.padding(horizontal = gutter()),
                    )
                } else {
                    LoadableContent(rows, onRetry = events::onRetry, modifier = Modifier.padding(horizontal = gutter())) {}
                }
                Loadable.Loading -> LoadableContent(rows, onRetry = events::onRetry) {}
            }
        }
        Spacer(Modifier.heightIn(min = 24.dp))
    }
}

/** Pixels before the end of the hits at which the next page of Seerr is asked for. */
private const val NEAR_END_PX = 600

@Composable
private fun SearchResults(hits: Loadable<List<ExternalCard>>, query: String, events: WishesEvents) {
    when (hits) {
        is Loadable.Ready -> {
            Text(stringResource(R.string.wishes_search_hits), style = SlouchcatType.section, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(horizontal = gutter(), vertical = 8.dp).semantics { heading() })
            hits.value.forEach { card -> ExternalRow(card, onClick = { events.onOpenTitle(card) }, modifier = Modifier.padding(horizontal = gutter())) }
        }
        Loadable.Empty -> MediaEmpty(stringResource(R.string.wishes_search_empty_title, query.trim()), body = stringResource(R.string.wishes_search_empty_body), icon = SlouchcatIcon.Search, modifier = Modifier.padding(horizontal = gutter()))
        else -> LoadableContent(hits, onRetry = events::onSubmit, modifier = Modifier.padding(horizontal = gutter())) {}
    }
}

@Composable
private fun rowTitle(key: DiscoverRowKey): String = stringResource(
    when (key) {
        DiscoverRowKey.TRENDING -> R.string.wishes_discover_rows_trending
        DiscoverRowKey.POPULAR_MOVIES -> R.string.wishes_discover_rows_popular_movies
        DiscoverRowKey.UPCOMING_MOVIES -> R.string.wishes_discover_rows_upcoming_movies
        DiscoverRowKey.POPULAR_SHOWS -> R.string.wishes_discover_rows_popular_shows
        DiscoverRowKey.UPCOMING_SHOWS -> R.string.wishes_discover_rows_upcoming_shows
        DiscoverRowKey.MOVIE_GENRES -> R.string.wishes_discover_rows_movie_genres
        else -> R.string.wishes_discover_rows_show_genres
    },
)

@Composable
private fun Mine(state: WishesUiState, events: WishesEvents) {
    val list = rememberLazyListState()
    val mine = state.mine
    if (mine.hasMore) {
        val nearEnd by remember(list) {
            derivedStateOf {
                val info = list.layoutInfo
                val last = info.visibleItemsInfo.lastOrNull()?.index ?: -1
                info.totalItemsCount > 0 && last >= info.totalItemsCount - 4
            }
        }
        LaunchedEffect(list, mine.loaded.size) { snapshotFlow { nearEnd }.distinctUntilChanged().filter { it }.collect { events.onMoreWishes() } }
    }
    LazyColumn(state = list, modifier = Modifier.fillMaxSize()) {
        item("filters") {
            Row(
                Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(horizontal = gutter(), vertical = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                listOf(
                    WishFilter.All to R.string.wishes_page_filters_all,
                    WishFilter.Open to R.string.wishes_page_filters_open,
                    WishFilter.Available to R.string.wishes_page_filters_available,
                    WishFilter.Closed to R.string.wishes_page_filters_closed,
                ).forEach { (filter, label) ->
                    SlouchcatChip(stringResource(label), selected = state.filter == filter, onClick = { events.onFilter(filter) })
                }
            }
        }
        when (val items = mine.items) {
            is Loadable.Ready -> {
                item("count") {
                    Text(
                        pluralCount(R.plurals.wishes_page_countAll, mine.total),
                        style = SlouchcatType.caption,
                        color = SlouchcatTheme.colors.tx2,
                        modifier = Modifier.padding(horizontal = gutter(), vertical = 4.dp),
                    )
                }
                items(items.value, key = { "wish-${it.id}" }) { card ->
                    WishRow(card, busy = card.id in state.busy, onOpen = { events.onOpenWish(card) }, onWithdraw = { events.onAskWithdraw(card) })
                }
                if (mine.more != MoreState.Idle) item("more") { Box(Modifier.padding(horizontal = gutter())) { PagingFooter(mine.more, events::onMoreWishes) } }
            }
            Loadable.Empty -> item("empty") {
                val (title, body) = when (state.filter) {
                    WishFilter.All -> R.string.wishes_page_empty_title to R.string.wishes_page_empty_body
                    WishFilter.Open -> R.string.wishes_page_emptyFilter_open to null
                    WishFilter.Available -> R.string.wishes_page_emptyFilter_available to null
                    WishFilter.Closed -> R.string.wishes_page_emptyFilter_closed to null
                }
                MediaEmpty(stringResource(title), body = body?.let { stringResource(it) }, icon = SlouchcatIcon.Wish, modifier = Modifier.padding(horizontal = gutter()))
            }
            else -> item("state") { LoadableContent(items, onRetry = events::onRetry, modifier = Modifier.padding(horizontal = gutter())) {} }
        }
        item("end") { Spacer(Modifier.heightIn(min = 24.dp)) }
    }
}

/** One wish (web `WishRow`): poster, title, «2024 · Film · Staffel 2», its state; the ✕ withdraws. */
@Composable
private fun WishRow(card: WishCard, busy: Boolean, onOpen: () -> Unit, onWithdraw: () -> Unit) {
    val c = SlouchcatTheme.colors
    val separator = stringResource(R.string.format_separator)
    val seasons = if (card.seasons.isEmpty()) null else pluralStringResource(R.plurals.wishes_seasons, card.seasons.size, card.seasons.joinToString(stringResource(R.string.format_listSeparator)))
    val requested = stringResource(R.string.app_wishes_requestedOn, card.createdAt.atZoneSameInstant(ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)))
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 88.dp)
            .padding(horizontal = gutter(), vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(
            Modifier.weight(1f).clip(RoundedCornerShape(8.dp)).clickable(onClick = onOpen).semantics(mergeDescendants = true) { role = Role.Button },
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            MediaImage(card.posters, card.title, Modifier.width(52.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp)))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                Text(card.title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(listOfNotNull(externalCaption(card.year, card.media), seasons).joinToString(separator), style = SlouchcatType.caption, color = c.tx2)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StateTag(wishStateText(card.state, card.stateDetail))
                    Text(if (card.joined) stringResource(R.string.wishes_card_joined) else requested, style = SlouchcatType.caption, color = c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                card.failReason?.let { reason -> failText(reason)?.let { Text(it, style = SlouchcatType.caption, color = c.tx2) } }
                if (card.fromSeerr) Text(stringResource(R.string.wishes_row_fromSeerr), style = SlouchcatType.caption, color = c.tx3)
            }
        }
        if (card.canWithdraw) {
            SlouchcatIconButton(
                SlouchcatIcon.Close,
                contentDescription = stringResource(if (card.joined) R.string.wishes_row_withdrawJoinedLabel else R.string.wishes_row_withdrawLabel, card.title),
                onClick = onWithdraw,
                enabled = !busy,
            )
        }
    }
}
