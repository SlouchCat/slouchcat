package com.slouchcat.app.tv.wishes

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.ProgressBar
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.search.TYPING_START
import com.slouchcat.app.tv.search.TvTyping
import com.slouchcat.app.tv.search.hardwareTyping
import com.slouchcat.app.tv.search.rememberTypingMode
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.BadgeTone
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.CardShape
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvBadge
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.wishes.WishesTab
import com.slouchcat.core.browse.wishes.WishesUiState
import com.slouchcat.core.browse.wishes.WishesViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.strings.R
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** Where the area leads: a tile ([title], from the search [hit]), a wish, «Start»; `null`: not registered. */
class WishesLinks(
    val title: (ExternalCard) -> Unit,
    val hit: (ExternalCard) -> Unit,
    val wish: (WishCard) -> Unit,
    val home: (() -> Unit)?,
)

/** The parts of the area on the TV: «Entdecken» and «Suche» are the model's [WishesTab.Discover]. */
enum class TvWishesTab { Discover, Search, Mine }

/** Test tags and focus keys of the area. */
object WishesKeys {
    fun tab(tab: TvWishesTab) = "tab-${tab.name.lowercase()}"
    fun wish(id: Long) = "wl-$id"
    fun withdraw(id: Long) = "wd-$id"
    const val HOME = "home"
    const val RETRY = "retry"
    const val GONE = "gone"
}

internal val W = SlouchcatTokens.Tv.Size.Wishes

/**
 * The area «Wünsche» (mockup `/wishes`, reference photo tv 32): the head with the tabs
 * «Entdecken», «Suche», «Meine Wünsche» (with the number of open wishes); «Entdecken» with the note
 * that a wish goes to Seerr and Seerr's rows of films and series; «Suche» with the own keyboard and
 * the hits as rows of films and series; «Meine Wünsche» with the limit, the filter and every wish
 * with its state, progress and «Zurückziehen». Wishes off or Seerr not set up: the page says so,
 * as the web app does. OK on a tab opens it; the focus starts on the open tab.
 */
@Composable
fun WishesScreen(state: WishesUiState, images: ServerImages, links: WishesLinks, vm: WishesViewModel, startMine: Boolean = false) {
    val focus = rememberFocusMemory()
    var tab by rememberSaveable { mutableStateOf(if (startMine) TvWishesTab.Mine else TvWishesTab.Discover) }
    // «Entdecken» and «Suche» are not for this profile: «Meine Wünsche» alone (the model's shownTab).
    val shown = if (state.shownTab == WishesTab.Mine) TvWishesTab.Mine else tab
    val choose = { next: TvWishesTab ->
        tab = next
        vm.onTab(if (next == TvWishesTab.Mine) WishesTab.Mine else WishesTab.Discover)
        // «Suche» opens on the keyboard (mockup `/search`); the other tabs keep the focus on themselves.
        if (next == TvWishesTab.Search) focus.focus(TYPING_START)
    }
    Column(Modifier.fillMaxSize()) {
        Column(Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.safeY)) {
            Text(stringResource(R.string.nav_wishes), style = TvType.h1, modifier = Modifier.semantics { heading() })
            if (state.rights is Loadable.Ready && state.discover) Tabs(shown, state.counts?.open ?: 0, focus, choose)
        }
        when (val rights = state.rights) {
            is Loadable.Ready -> when (shown) {
                TvWishesTab.Discover -> Discover(state, links, vm, focus, onMine = { choose(TvWishesTab.Mine) })
                TvWishesTab.Search -> Search(state, links, vm, focus)
                TvWishesTab.Mine -> Mine(state, links, vm, focus, onDiscover = if (state.discover) ({ choose(TvWishesTab.Discover) }) else null)
            }
            Loadable.Empty -> EmptyState(
                SlouchcatIcon.Wish.vector,
                stringResource(R.string.app_wishes_off_title),
                stringResource(R.string.app_wishes_off_body),
                Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX),
                action = links.home?.let { home -> { EmptyAction(stringResource(R.string.app_lists_toStart), SlouchcatIcon.Home.vector, home, Modifier.focusKey(focus, WishesKeys.HOME)) } },
            )
            else -> LoadableContent(
                rights,
                onRetry = {
                    // The button goes with the error: the focus waits for the open tab.
                    focus.focus(WishesKeys.tab(shown))
                    vm.retry()
                },
                modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.l),
                retryModifier = Modifier.focusKey(focus, WishesKeys.RETRY),
            ) {}
        }
    }
    val fallback = when (state.rights) {
        is Loadable.Ready -> if (state.discover) WishesKeys.tab(shown) else firstMineKey(state)
        Loadable.Empty -> WishesKeys.HOME
        else -> WishesKeys.RETRY
    }
    focus.StartFocus(fallback, state.rights::class, shown, fallback)
    Withdraw(state, vm, focus)
}

/** The first control of «Meine Wünsche» without the tabs: the filter «Alle». */
private fun firstMineKey(state: WishesUiState) = "f-${state.filter.name.lowercase()}"

/** `.md-tabs`: pills; the open one on `--s3` with a frame, the focused one light. */
@Composable
private fun Tabs(shown: TvWishesTab, open: Int, focus: FocusMemory, onTab: (TvWishesTab) -> Unit) {
    Row(Modifier.padding(top = W.TabsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
        Tab(stringResource(R.string.wishes_area_discover), shown == TvWishesTab.Discover, null, Modifier.focusKey(focus, WishesKeys.tab(TvWishesTab.Discover))) { onTab(TvWishesTab.Discover) }
        Tab(stringResource(R.string.wishes_area_search), shown == TvWishesTab.Search, null, Modifier.focusKey(focus, WishesKeys.tab(TvWishesTab.Search))) { onTab(TvWishesTab.Search) }
        Tab(stringResource(R.string.wishes_area_mine), shown == TvWishesTab.Mine, open.takeIf { it > 0 }, Modifier.focusKey(focus, WishesKeys.tab(TvWishesTab.Mine))) { onTab(TvWishesTab.Mine) }
    }
}

@Composable
private fun Tab(text: String, selected: Boolean, count: Int?, modifier: Modifier, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val said = if (count != null) pluralCount(R.plurals.wishes_area_mineCount, count) else text
    Surface(
        onClick = onClick,
        modifier = modifier
            .height(W.Tab)
            .semantics {
                role = Role.Tab
                this.selected = selected
                contentDescription = said
            },
        shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = if (selected) c.s3 else androidx.compose.ui.graphics.Color.Transparent,
            contentColor = if (selected) c.tx else c.tx2,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, if (selected) c.frame else androidx.compose.ui.graphics.Color.Transparent), shape = CircleShape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = CircleShape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(Modifier.align(Alignment.Center).padding(horizontal = W.TabPad), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
            Text(text, style = TvType.bodyStrong, maxLines = 1)
            if (count != null) Count(count)
        }
    }
}

/** `.md-cnt`: the number in the accent. */
@Composable
private fun Count(count: Int) {
    val c = SlouchcatTheme.colors
    Box(Modifier.heightIn(min = W.Count).widthIn(min = W.Count).clip(CircleShape).background(c.accent).padding(horizontal = W.CountPad), contentAlignment = Alignment.Center) {
        Text(count.toString(), style = TvType.badge, color = c.onAccent, maxLines = 1)
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

/** A tile of a title outside the library: the poster, the state on top (`.tc` with tag). */
@Composable
private fun ExternalTile(card: ExternalCard, onClick: () -> Unit, modifier: Modifier) {
    val mark = cardMark(card)
    val said = listOfNotNull(card.title, externalCaption(card.year, card.media), mark).joinToString(stringResource(R.string.format_listSeparator))
    TvCard(title = said, image = card.posters.forWidth(CardShape.Poster.width), onClick = onClick, modifier = modifier, tag = mark)
}

/** What the line under a row says about a tile: «2024 · Film · nicht in der Bibliothek». */
@Composable
private fun ExternalCard.info(): CardInfo = CardInfo(title, listOf(externalCaption(year, media), cardMark(this) ?: stringResource(R.string.titles_missing)).joinToString(stringResource(R.string.format_separator)))

/** «Entdecken»: the note, then Seerr's rows (films and series; the genre rows stay in the web app). */
@Composable
private fun Discover(state: WishesUiState, links: WishesLinks, vm: WishesViewModel, focus: FocusMemory, onMine: () -> Unit) {
    val c = SlouchcatTheme.colors
    val list = rememberLazyListState()
    LazyColumn(state = list, modifier = Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = TvDimens.Browse.DetailTop)) {
        item(key = "lead") {
            Text(
                stringResource(R.string.app_wishes_discoverIntro),
                style = TvType.body,
                color = c.tx3,
                modifier = Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX, top = W.LeadTop, bottom = TvDimens.m),
            )
        }
        if (state.stale) item(key = "stale") { TvNote(stringResource(R.string.app_wishes_stale), Modifier.padding(start = LocalContentStart.current, bottom = TvDimens.m), tone = NoteTone.Warn, small = true) }
        when (val rows = state.rows) {
            is Loadable.Ready -> items(rows.value, key = { it.key.value }) { row ->
                val infos = row.cards.associate { it.key to it.info() }
                TvCardRow(
                    title = rowTitle(row.key),
                    items = row.cards,
                    key = { it.key },
                    focus = focus,
                    rowKey = "r-${row.key.value}",
                    info = { infos[it.key] },
                ) { card, m -> ExternalTile(card, { links.title(card) }, m) }
            }
            Loadable.Empty -> item(key = "empty") {
                EmptyState(SlouchcatIcon.Wish.vector, stringResource(R.string.wishes_discover_empty), null, Modifier.padding(start = LocalContentStart.current))
            }
            is Loadable.Failed -> item(key = "failed") {
                if (rows.error == LoadError.NotFound) {
                    EmptyState(
                        SlouchcatIcon.Wish.vector,
                        stringResource(R.string.wishes_discover_gone_title),
                        stringResource(R.string.wishes_discover_gone_body),
                        Modifier.padding(start = LocalContentStart.current),
                        action = { EmptyAction(stringResource(R.string.wishes_discover_gone_action), SlouchcatIcon.Wish.vector, onMine, Modifier.focusKey(focus, WishesKeys.GONE)) },
                    )
                } else {
                    LoadableContent(
                        rows,
                        onRetry = {
                            focus.focus(WishesKeys.tab(TvWishesTab.Discover))
                            vm.retry()
                        },
                        modifier = Modifier.padding(start = LocalContentStart.current),
                        retryModifier = Modifier.focusKey(focus, WishesKeys.RETRY),
                    ) {}
                }
            }
            Loadable.Loading -> item(key = "loading") { TvBusy(stringResource(R.string.app_state_loading), Modifier.padding(start = LocalContentStart.current, top = TvDimens.l)) }
        }
    }
}

/**
 * «Suche»: the own keyboard (the search of stage 2b) on the left, the hits on the right as a row
 * of films and a row of series. Seerr is asked a short pause after the last key.
 */
@Composable
private fun Search(state: WishesUiState, links: WishesLinks, vm: WishesViewModel, focus: FocusMemory) {
    val c = SlouchcatTheme.colors
    val mode = rememberTypingMode()
    Row(
        Modifier
            .fillMaxSize()
            .padding(start = LocalContentStart.current, top = TvDimens.s)
            .hardwareTyping(mode, { state.text }, vm::onText),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.xxl),
    ) {
        TvTyping(
            text = state.text,
            onText = vm::onText,
            onClear = vm::onClear,
            focus = focus,
            mode = mode,
            placeholder = stringResource(R.string.wishes_search_placeholder),
        )
        CompositionLocalProvider(LocalContentStart provides 0.dp) {
            LazyColumn(Modifier.weight(1f).fillMaxSize(), contentPadding = PaddingValues(top = TvDimens.l, bottom = TvDimens.Browse.DetailTop)) {
                when (val hits = state.results?.items) {
                    null -> item(key = "start") {
                        val tooShort = state.text.isNotBlank() && state.text.trim().length < WishesViewModel.MIN_QUERY
                        Text(stringResource(if (tooShort) R.string.wishes_search_tooShort else R.string.wishes_search_start), style = TvType.large, color = c.tx2, modifier = Modifier.padding(end = TvDimens.safeX))
                    }
                    is Loadable.Ready -> {
                        val cards = hits.value
                        for ((media, label) in listOf(ExternalMedia.MOVIE to R.string.wishes_area_movies, ExternalMedia.SHOW to R.string.wishes_area_series)) {
                            val group = cards.filter { it.media == media }
                            if (group.isEmpty()) continue
                            item(key = "hits-${media.value}") {
                                val infos = group.associate { it.key to it.info() }
                                TvCardRow(
                                    title = "${stringResource(label)} · ${group.size}",
                                    items = group,
                                    key = { it.key },
                                    focus = focus,
                                    rowKey = "s-${media.value}",
                                    info = { infos[it.key] },
                                ) { card, m ->
                                    ExternalTile(card, { links.hit(card) }, m.onFocusChanged { if (it.isFocused && cards.indexOf(card) >= cards.size - NEAR_END) vm.onMoreResults() })
                                }
                            }
                        }
                    }
                    Loadable.Empty -> item(key = "none") {
                        EmptyState(SlouchcatIcon.Search.vector, stringResource(R.string.wishes_search_empty_title, state.text.trim()), stringResource(R.string.wishes_search_empty_body))
                    }
                    is Loadable.Failed -> item(key = "failed") {
                        LoadableContent(
                            hits,
                            onRetry = {
                                focus.focus(TYPING_START)
                                vm.onSubmit()
                            },
                            retryModifier = Modifier.focusKey(focus, WishesKeys.RETRY),
                        ) {}
                    }
                    Loadable.Loading -> item(key = "loading") { TvBusy(stringResource(R.string.app_state_loading)) }
                }
            }
        }
    }
}

/** «Meine Wünsche»: the limit, the filter, the wishes page by page. */
@Composable
private fun Mine(state: WishesUiState, links: WishesLinks, vm: WishesViewModel, focus: FocusMemory, onDiscover: (() -> Unit)?) {
    val c = SlouchcatTheme.colors
    val mine = state.mine
    val loaded = mine.loaded
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = LocalContentStart.current, end = TvDimens.safeX, top = W.BodyTop, bottom = TvDimens.Browse.DetailTop)) {
        state.quota?.let { quota -> item(key = "quota") { Quota(quota) } }
        item(key = "filters") {
            Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                listOf(
                    WishFilter.All to R.string.wishes_page_filters_all,
                    WishFilter.Open to R.string.wishes_page_filters_open,
                    WishFilter.Available to R.string.wishes_page_filters_available,
                    WishFilter.Closed to R.string.wishes_page_filters_closed,
                ).forEach { (filter, label) ->
                    ControlChip(stringResource(label), SlouchcatIcon.Filter.vector, onClick = { vm.onFilter(filter) }, on = state.filter == filter, modifier = Modifier.focusKey(focus, "f-${filter.name.lowercase()}"))
                }
            }
        }
        when (val items = mine.items) {
            is Loadable.Ready -> {
                item(key = "count") {
                    Text(pluralCount(R.plurals.wishes_page_countAll, mine.total), style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = TvDimens.l, bottom = TvDimens.s))
                }
                items(items.value, key = { "wish-${it.id}" }) { card ->
                    WishRow(
                        card,
                        focus,
                        onOpen = { links.wish(card) },
                        onWithdraw = { vm.onAskWithdraw(card) },
                        modifier = Modifier.onFocusChanged { if (it.hasFocus && loaded.indexOf(card) >= loaded.size - NEAR_END) vm.onMoreWishes() },
                    )
                }
                if (mine.more == MoreState.Failed) {
                    item(key = "more") { TvButton(stringResource(R.string.app_state_retry), onClick = vm::onMoreWishes, icon = SlouchcatIcon.Refresh.vector, modifier = Modifier.padding(top = TvDimens.l).focusKey(focus, "more")) }
                }
            }
            Loadable.Empty -> item(key = "empty") {
                val (title, body) = when (state.filter) {
                    WishFilter.All -> R.string.wishes_page_empty_title to R.string.wishes_page_empty_body
                    WishFilter.Open -> R.string.wishes_page_emptyFilter_open to null
                    WishFilter.Available -> R.string.wishes_page_emptyFilter_available to null
                    WishFilter.Closed -> R.string.wishes_page_emptyFilter_closed to null
                }
                EmptyState(
                    SlouchcatIcon.Wish.vector,
                    stringResource(title),
                    body?.let { stringResource(it) },
                    action = onDiscover?.takeIf { state.filter == WishFilter.All }?.let { open -> { EmptyAction(stringResource(R.string.wishes_area_discover), SlouchcatIcon.Wish.vector, open, Modifier.focusKey(focus, WishesKeys.GONE)) } },
                )
            }
            else -> item(key = "state") {
                LoadableContent(
                    items,
                    onRetry = {
                        focus.focus(firstMineKey(state))
                        vm.retry()
                    },
                    modifier = Modifier.padding(top = TvDimens.l),
                    retryModifier = Modifier.focusKey(focus, WishesKeys.RETRY),
                ) {}
            }
        }
    }
}

/** `.md-quota`: films and series with their share of the limit. */
@Composable
private fun Quota(quota: WishQuota) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    Column(
        Modifier
            .widthIn(max = W.Quota)
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(TvDimens.borderThin, c.line), shape)
            .padding(horizontal = W.QuotaPadX, vertical = W.QuotaPadY)
            .semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.spacedBy(TvDimens.s),
    ) {
        for (media in listOf(ExternalMedia.MOVIE, ExternalMedia.SHOW)) {
            val part = if (media == ExternalMedia.SHOW) quota.tv else quota.movie
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                Text(quotaLine(media, quota), style = TvType.small, color = if (media == ExternalMedia.MOVIE) c.tx else c.tx2, modifier = Modifier.width(W.QuotaKey))
                if (part != null && !quota.unlimited && part.limit > 0) ProgressBar(part.used.toFloat() / part.limit, W.Quota - W.QuotaKey - W.QuotaPadX * 2 - TvDimens.l)
            }
        }
    }
}

/** `.md-wrow`: the wish (poster, title, kind and seasons, state, when, progress) and «Zurückziehen» beside it. */
@Composable
private fun WishRow(card: WishCard, focus: FocusMemory, onOpen: () -> Unit, onWithdraw: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    var focused by remember { mutableStateOf(false) }
    val separator = stringResource(R.string.format_separator)
    // «Staffeln {{list}}» has no count of its own: the list is its only argument.
    val list = card.seasons.joinToString(stringResource(R.string.format_listSeparator))
    val seasons = if (card.seasons.isEmpty()) null else LocalResources.current.getQuantityString(R.plurals.wishes_seasons, card.seasons.size, list)
    val caption = listOfNotNull(externalCaption(card.year, card.media), seasons).joinToString(separator)
    val state = wishStateText(card.state, card.stateDetail)
    val requested = if (card.joined) stringResource(R.string.wishes_card_joined) else stringResource(R.string.app_wishes_requestedOn, card.createdAt.atZoneSameInstant(ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)))
    val progress = card.progress?.let { progressText(it) }
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    Row(modifier.widthIn(max = W.List).padding(vertical = W.ListGap / 2), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        Surface(
            onClick = onOpen,
            modifier = Modifier
                .weight(1f)
                .focusKey(focus, WishesKeys.wish(card.id))
                .onFocusChanged { focused = it.isFocused }
                .semantics(mergeDescendants = true) { contentDescription = listOfNotNull(card.title, caption, state, requested, progress).joinToString(", ") },
            shape = ClickableSurfaceDefaults.shape(shape = shape),
            colors = ClickableSurfaceDefaults.colors(containerColor = c.s1, contentColor = c.tx, focusedContainerColor = c.tx, focusedContentColor = c.bg, pressedContainerColor = c.tx, pressedContentColor = c.bg),
            border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape)),
            scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
        ) {
            Row(Modifier.padding(W.RowPad), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(W.RowGap)) {
                Box(Modifier.width(W.RowPoster).aspectRatio(POSTER).clip(RoundedCornerShape(TvDimens.radiusCard)).background(c.s2)) {
                    card.posters.forWidth(W.RowPoster)?.let { TvImage(it, Modifier.fillMaxSize()) }
                }
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TvDimens.xs)) {
                    val quiet = if (focused) c.bg else c.tx3
                    Text(card.title, style = TvType.largeStrong, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(caption, style = TvType.small, color = quiet, maxLines = 1)
                    Row(Modifier.padding(top = TvDimens.xs), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                        TvBadge(state, tone = toneOf(card.state), icon = if (card.state == WishState.AVAILABLE) SlouchcatIcon.Check.vector else null, onLight = focused)
                        Text(listOfNotNull(requested, progress).joinToString(separator), style = TvType.small, color = quiet, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    card.progress?.percent?.let { ProgressBar(it / PERCENT, W.RowBar, Modifier.padding(top = TvDimens.xs)) }
                    card.failReason?.let { reason -> failText(reason)?.let { Text(it, style = TvType.small, color = quiet, maxLines = 2) } }
                }
            }
        }
        Box(Modifier.width(W.RowAction)) {
            if (card.canWithdraw) {
                TvButton(
                    stringResource(if (card.joined) R.string.wishes_withdraw_actionJoined else R.string.wishes_withdraw_action),
                    onClick = onWithdraw,
                    icon = SlouchcatIcon.Close.vector,
                    kind = TvButtonKind.Ghost,
                    small = true,
                    modifier = Modifier.focusKey(focus, WishesKeys.withdraw(card.id)).semantics {
                        contentDescription = listOf(card.title, state).joinToString(", ")
                    },
                )
            }
        }
    }
}

/** The badge of a state (`.badge`, `.badge.acc`, `.badge.ok`): on its way the accent, there the green. */
fun toneOf(state: WishState): BadgeTone = when (state) {
    WishState.AVAILABLE -> BadgeTone.Ok
    WishState.PROCESSING, WishState.PARTIAL -> BadgeTone.Accent
    WishState.FAILED, WishState.DECLINED -> BadgeTone.Warn
    else -> BadgeTone.Plain
}

/** «Wunsch zurückziehen?» (mockup tv 60): «Zurückziehen» and «Behalten» (the focus starts there). */
@Composable
private fun Withdraw(state: WishesUiState, vm: WishesViewModel, focus: FocusMemory) {
    val asking = state.withdrawing
    // The question closed (kept, withdrawn or failed): back to the element of before (2b review M-4).
    LaunchedEffect(asking == null) { if (asking == null) focus.restore() }
    asking ?: return
    val keep = remember { FocusRequester() }
    TvDialog(
        stringResource(if (asking.joined) R.string.wishes_withdraw_titleJoined else R.string.wishes_withdraw_title),
        onDismiss = vm::onDismissWithdraw,
        initialFocus = keep,
    ) {
        Text(
            if (asking.joined) stringResource(R.string.wishes_withdraw_bodyJoined, asking.title) else stringResource(R.string.app_wishes_withdrawBody, asking.title),
            style = TvType.large,
            color = SlouchcatTheme.colors.tx2,
        )
        TvActions {
            TvButton(
                stringResource(if (asking.joined) R.string.wishes_withdraw_confirmJoined else R.string.wishes_withdraw_confirm),
                onClick = {
                    // The button «Zurückziehen» of the row goes: the focus stays on the wish itself.
                    focus.focus(WishesKeys.wish(asking.id))
                    vm.onConfirmWithdraw()
                },
                icon = SlouchcatIcon.Close.vector,
            )
            TvButton(stringResource(R.string.app_wishes_keep), onClick = vm::onDismissWithdraw, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(keep))
        }
    }
}

/** How many tiles or wishes before the end of the loaded ones the next page is asked for. */
private const val NEAR_END = 4

private const val POSTER = 2f / 3f
private const val PERCENT = 100f
