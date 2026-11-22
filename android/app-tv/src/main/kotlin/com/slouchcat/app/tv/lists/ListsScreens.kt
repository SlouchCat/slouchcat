package com.slouchcat.app.tv.lists

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.PageHead
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.screenWidth
import com.slouchcat.app.tv.browse.watchedTag
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.BackdropMode
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvActions
import com.slouchcat.app.tv.ui.TvBackdrop
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvCardGrid
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDialog
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvRowInfo
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.browse.collections.CollectionPage
import com.slouchcat.core.browse.collections.CollectionUiState
import com.slouchcat.core.browse.collections.CollectionWatched
import com.slouchcat.core.browse.collections.CollectionsUiState
import com.slouchcat.core.browse.lists.ListKind
import com.slouchcat.core.browse.mylists.MyListUiState
import com.slouchcat.core.browse.mylists.WatchedCard
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

private const val GRID = "grid"
private const val RETRY = "retry"
private const val EMPTY = "empty"
private const val EDIT = "edit"
private const val COLUMNS = 7

/**
 * `.br-fixed`: the frame of the grid pages (lists, watchlist, watched, collections): the head, an
 * optional row of controls, the line with the focused title (or [summary]), the grid below.
 */
@Composable
private fun GridPage(
    icon: ImageVector,
    title: String,
    count: String?,
    summary: CardInfo?,
    focused: CardInfo?,
    controls: (@Composable () -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    Column(Modifier.fillMaxSize()) {
        Column(Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.safeY)) {
            PageHead(icon, title, count)
            if (controls != null) {
                Row(Modifier.padding(top = TvDimens.Browse.ControlsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) { controls() }
            }
        }
        TvRowInfo(focused ?: summary, visible = true, modifier = Modifier.padding(top = TvDimens.l))
        content()
    }
}

/** A grid of title cards; [onCard] decides what OK does; the next page comes near the end. */
@Composable
private fun TitleGrid(
    items: List<MediaItem>,
    focus: FocusMemory,
    onCard: ((MediaItem) -> Unit)?,
    onNearEnd: () -> Unit,
    onFocused: (MediaItem?) -> Unit,
    tag: @Composable (MediaItem) -> String? = { watchedTag(it) },
) {
    val index = remember(items) { items.withIndex().associate { it.value.id to it.index } }
    TvCardGrid(
        items = items,
        key = { it.id },
        focus = focus,
        gridKey = GRID,
        columns = COLUMNS,
        modifier = Modifier.fillMaxWidth().onFocusChanged { if (!it.hasFocus) onFocused(null) },
    ) { item, modifier ->
        MediaCard(
            item,
            onCard?.let { open -> { open(item) } },
            modifier.onFocusChanged {
                if (it.isFocused) {
                    onFocused(item)
                    if ((index[item.id] ?: 0) >= items.size - COLUMNS * 2) onNearEnd()
                }
            },
            tag = tag(item),
            width = null,
        )
    }
}

/** What a [ListKind] is called and shown with (mockup `BR_LISTS`). */
@Composable
private fun listHead(kind: ListKind?): Pair<ImageVector, String> = when (kind) {
    ListKind.Picks -> SlouchcatIcon.Heart.vector to stringResource(R.string.home_picks)
    is ListKind.Row -> when (kind.key) {
        HomeListKey.CONTINUE -> SlouchcatIcon.Play.vector to stringResource(R.string.home_rows_continue)
        HomeListKey.RECENT -> SlouchcatIcon.Clock.vector to stringResource(R.string.home_rows_recent)
        HomeListKey.RELEASED -> SlouchcatIcon.Star.vector to stringResource(R.string.home_rows_released)
        HomeListKey.SPOTLIGHT -> SlouchcatIcon.Star.vector to stringResource(R.string.home_rows_spotlight)
        else -> SlouchcatIcon.Grid.vector to stringResource(R.string.home_title)
    }
    null -> SlouchcatIcon.Grid.vector to stringResource(R.string.home_list_notFound_title)
}

/** «Alle anzeigen» of a row of the start page (mockup `/list/:key`): the whole list in a grid. */
@Composable
fun ListScreen(kind: ListKind?, state: Paged<MediaItem>, onItem: ((MediaItem) -> Unit)?, onRetry: () -> Unit, onNearEnd: () -> Unit, onHome: (() -> Unit)?) {
    val focus = rememberFocusMemory()
    var focused by remember { mutableStateOf<MediaItem?>(null) }
    val (icon, title) = listHead(kind)
    val tip = stringResource(R.string.app_browse_tip)
    GridPage(icon, title, (state.items as? Loadable.Ready)?.let { pluralCount(R.plurals.home_list_count, state.total) }, null, focused?.info()) {
        when (val items = state.items) {
            is Loadable.Ready -> TitleGrid(items.value, focus, onItem, onNearEnd, { focused = it }) { item -> if (kind == ListKind.Picks) tip else watchedTag(item) }
            Loadable.Empty, Loadable.Failed(com.slouchcat.core.media.LoadError.NotFound) -> Empty(
                SlouchcatIcon.Grid.vector,
                stringResource(if (kind == null || items != Loadable.Empty) R.string.home_list_notFound_title else R.string.home_list_empty),
                if (kind == null || items != Loadable.Empty) stringResource(R.string.home_list_notFound_body) else null,
                onHome,
                focus,
            )
            else -> LoadableContent(items, onRetry, Modifier.padding(start = LocalContentStart.current), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        }
    }
    focus.StartFocus(startOf(state.items) { it.id }, state.items::class)
}

private fun <T> startOf(items: Loadable<List<T>>, id: (T) -> String): String = when (items) {
    is Loadable.Ready -> "$GRID-${id(items.value.first())}"
    is Loadable.Failed -> if (items.error == com.slouchcat.core.media.LoadError.NotFound) EMPTY else RETRY
    Loadable.Empty -> EMPTY
    Loadable.Loading -> RETRY
}

/** `.br-empty` of a grid page, with «Zur Startseite» when there is one. */
@Composable
private fun Empty(icon: ImageVector, title: String, text: String?, onHome: (() -> Unit)?, focus: FocusMemory) {
    EmptyState(
        icon,
        title,
        text,
        Modifier.padding(start = LocalContentStart.current),
        action = onHome?.let { home -> { EmptyAction(stringResource(R.string.app_lists_toStart), SlouchcatIcon.Home.vector, home, Modifier.focusKey(focus, EMPTY)) } },
    )
}

/** The chip «Bearbeiten» / «Fertig» and the hint of the mode (mockup `brEditList`). */
@Composable
private fun EditChip(editing: Boolean, hint: String, onToggle: () -> Unit, focus: FocusMemory) {
    ControlChip(
        stringResource(if (editing) R.string.app_lists_done else R.string.app_lists_edit),
        if (editing) SlouchcatIcon.Check.vector else com.slouchcat.app.tv.ui.TvIcons.Edit,
        onClick = onToggle,
        on = editing,
        modifier = Modifier.focusKey(focus, EDIT),
    )
    if (editing) {
        Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
            com.slouchcat.app.tv.ui.TvIcon(SlouchcatIcon.Trash, TvDimens.Browse.Icon, tint = SlouchcatTheme.colors.tx2)
            Text(hint, style = TvType.body, color = SlouchcatTheme.colors.tx2)
        }
    }
}

/**
 * «Merkliste» (mockup `/watchlist`): the grid, «Bearbeiten» switches OK on a card from opening it
 * to taking it off the list (the card shows «Entfernen»); the focus moves on to the next card.
 */
@Composable
fun WatchlistScreen(
    state: MyListUiState<MediaItem>,
    onItem: ((MediaItem) -> Unit)?,
    onRemove: (MediaItem) -> Unit,
    onToggleEdit: () -> Unit,
    onRetry: () -> Unit,
    onMore: () -> Unit,
    onHome: (() -> Unit)?,
) {
    val focus = rememberFocusMemory()
    var focused by remember { mutableStateOf<MediaItem?>(null) }
    val shown = state.shown { it.id }
    val remove = stringResource(R.string.watchlist_remove)
    GridPage(
        SlouchcatIcon.Bookmark.vector,
        stringResource(R.string.watchlist_title),
        (shown as? Loadable.Ready)?.let { pluralCount(R.plurals.home_list_count, state.total) },
        null,
        focused?.info(),
        controls = if (shown is Loadable.Ready) ({ EditChip(state.editing, stringResource(R.string.tv_lists_removeHint), onToggleEdit, focus) }) else null,
    ) {
        when (shown) {
            is Loadable.Ready -> TitleGrid(
                shown.value,
                focus,
                onCard = if (state.editing) ({ item -> moveOn(shown.value, item, focus); onRemove(item) }) else onItem,
                onNearEnd = onMore,
                onFocused = { focused = it },
                tag = { if (state.editing) remove else watchedTag(it) },
            )
            Loadable.Empty -> Empty(SlouchcatIcon.Bookmark.vector, stringResource(R.string.watchlist_empty_title), stringResource(R.string.watchlist_empty_body), onHome, focus)
            else -> LoadableContent(shown, onRetry, Modifier.padding(start = LocalContentStart.current), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        }
    }
    focus.StartFocus(startOf(shown) { it.id }, shown::class)
}

/** After a card goes: the focus to its neighbour (mockup `brRemove`), else to «Bearbeiten». */
private fun moveOn(items: List<MediaItem>, gone: MediaItem, focus: FocusMemory) {
    val at = items.indexOf(gone)
    val next = items.getOrNull(at + 1) ?: items.getOrNull(at - 1)
    focus.focus(if (next != null) "$GRID-${next.id}" else EDIT)
}

/** «Gesehen» (mockup `/watched`): like the watchlist; OK in «Bearbeiten» takes the check mark off (a series asks first). */
@Composable
fun WatchedScreen(
    state: MyListUiState<WatchedCard>,
    onItem: ((MediaItem) -> Unit)?,
    onAsk: (WatchedCard) -> Unit,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
    onToggleEdit: () -> Unit,
    onRetry: () -> Unit,
    onMore: () -> Unit,
    onHome: (() -> Unit)?,
) {
    val focus = rememberFocusMemory()
    var focused by remember { mutableStateOf<MediaItem?>(null) }
    val shown = state.shown { it.id }
    val unmark = stringResource(R.string.watched_hint_remove)
    GridPage(
        SlouchcatIcon.Eye.vector,
        stringResource(R.string.watched_page_title),
        (shown as? Loadable.Ready)?.let { pluralCount(R.plurals.home_list_count, state.total) },
        null,
        focused?.info(),
        controls = if (shown is Loadable.Ready) ({ EditChip(state.editing, stringResource(R.string.tv_lists_unwatchHint), onToggleEdit, focus) }) else null,
    ) {
        when (shown) {
            is Loadable.Ready -> {
                val items = shown.value.map { it.item }
                TitleGrid(
                    items,
                    focus,
                    onCard = if (state.editing) ({ item: MediaItem -> unmarkCard(shown.value, items, item, focus, onAsk) }) else onItem,
                    onNearEnd = onMore,
                    onFocused = { focused = it },
                    tag = { if (state.editing) unmark else null },
                )
            }
            Loadable.Empty -> Empty(SlouchcatIcon.Eye.vector, stringResource(R.string.watched_page_empty_title), stringResource(R.string.watched_page_empty_body), onHome, focus)
            else -> LoadableContent(shown, onRetry, Modifier.padding(start = LocalContentStart.current), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        }
    }
    focus.StartFocus(startOf(shown) { it.id }, shown::class)
    state.asking?.let { card ->
        val cancel = remember { FocusRequester() }
        val close = {
            onDismiss()
            focus.restore()
        }
        TvDialog(stringResource(R.string.watched_ask_removeTitle, card.item.title), onDismiss = close, initialFocus = cancel) {
            Text(stringResource(R.string.watched_ask_removeBodyAll), style = TvType.large, color = SlouchcatTheme.colors.tx2)
            TvActions {
                TvButton(stringResource(R.string.watched_ask_removeAll), onClick = { onConfirm(); focus.restore() }, icon = SlouchcatIcon.Close.vector)
                TvButton(stringResource(R.string.common_cancel), onClick = close, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
            }
        }
    }
}

/** OK on a card of «Gesehen» while editing: a film goes at once (the focus moves on), a series asks. */
private fun unmarkCard(cards: List<WatchedCard>, items: List<MediaItem>, item: MediaItem, focus: FocusMemory, onAsk: (WatchedCard) -> Unit) {
    val card = cards.firstOrNull { it.id == item.id } ?: return
    if (card.episodeCount == null) moveOn(items, item, focus)
    onAsk(card)
}

/** «Sammlungen» (mockup `/collections`): the posters of the collections, the count in the info line. */
@Composable
fun CollectionsScreen(state: CollectionsUiState, images: ServerImages, onCollection: ((CollectionRef) -> Unit)?, onRetry: () -> Unit, onHome: (() -> Unit)?) {
    val focus = rememberFocusMemory()
    var focused by remember { mutableStateOf<CollectionRef?>(null) }
    val list = state.collections
    val counts = (list as? Loadable.Ready)?.value.orEmpty().associate { it.id to pluralCount(R.plurals.collection_count, it.count) }
    GridPage(
        SlouchcatIcon.Grid.vector,
        stringResource(R.string.collection_all_title),
        (list as? Loadable.Ready)?.let { pluralCount(R.plurals.collection_all_count, it.value.size) },
        null,
        focused?.let { CardInfo(it.name, counts[it.id]) },
    ) {
        when (list) {
            is Loadable.Ready -> TvCardGrid(list.value, key = { it.id }, focus = focus, gridKey = GRID, columns = COLUMNS, modifier = Modifier.onFocusChanged { if (!it.hasFocus) focused = null }) { col, modifier ->
                val cover = col.cover?.let { MediaItem.from(it, images) }
                TvCard(
                    listOfNotNull(col.name, counts[col.id]).joinToString(", "),
                    images.single(col.image).or(cover?.poster ?: images.single(null)).forWidth(com.slouchcat.app.tv.ui.CardShape.Poster.width),
                    onClick = { onCollection?.invoke(col) },
                    modifier = modifier.onFocusChanged { if (it.isFocused) focused = col },
                    width = null,
                )
            }
            Loadable.Empty -> Empty(SlouchcatIcon.Grid.vector, stringResource(R.string.collection_all_empty), null, onHome, focus)
            else -> LoadableContent(list, onRetry, Modifier.padding(start = LocalContentStart.current), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        }
    }
    focus.StartFocus(startOf(list) { it.id }, list::class)
}

/** What a collection page can be asked to do. */
class CollectionActions(
    val item: ((MediaItem) -> Unit)?,
    /** «Abspielen» of the next part: the player of stage 3; `null` without it. */
    val play: ((MediaItem) -> Unit)?,
    val listAll: () -> Unit,
    val watched: () -> Unit,
    val confirm: (Boolean) -> Unit,
    val dismiss: () -> Unit,
    val retry: () -> Unit,
)

/**
 * One collection (mockup `/collection/:id`): the poster on the left; «Sammlung», the name, count and
 * years, «Abspielen» (next part, with the player of stage 3), «Alle merken», «Gesehen», and «In
 * dieser Sammlung» as a row. The art layer shows the first part, darkened.
 */
@Composable
fun CollectionScreen(state: CollectionUiState, images: ServerImages, actions: CollectionActions) {
    val focus = rememberFocusMemory()
    val c = SlouchcatTheme.colors
    val page = (state.page as? Loadable.Ready)?.value
    Box(Modifier.fillMaxSize()) {
        TvBackdrop(page?.items?.firstOrNull()?.backdrop?.forWidth(screenWidth()), mode = BackdropMode.Dim)
        if (page == null) {
            LoadableContent(state.page, actions.retry, Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop), retryModifier = Modifier.focusKey(focus, RETRY)) {}
        } else {
            Row(Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.PersonTop, end = TvDimens.safeX), horizontalArrangement = Arrangement.spacedBy(TvDimens.Browse.PersonGap)) {
                Box(Modifier.width(TvDimens.Browse.PersonImage).aspectRatio(POSTER).clip(RoundedCornerShape(TvDimens.radiusPanel)).background(c.s2)) {
                    TvImage(images.single(page.collection.image).or(page.items.firstOrNull()?.poster ?: images.single(null)).forWidth(TvDimens.Browse.PersonImage), Modifier.fillMaxSize())
                }
                Column(Modifier.weight(1f)) {
                    KindLine(stringResource(R.string.app_collections_overline))
                    Text(page.collection.name, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).semantics { heading() })
                    Text(collectionFacts(page), style = TvType.large, color = c.tx2, modifier = Modifier.padding(vertical = TvDimens.m))
                    Row(Modifier.padding(top = TvDimens.m), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                        val next = page.collection.next?.let { n -> page.items.firstOrNull { it.id == n.id } } ?: page.items.firstOrNull()
                        val play = actions.play
                        if (play != null && next != null) {
                            TvButton(stringResource(R.string.item_play), onClick = { play(next) }, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, "play"))
                        }
                        TvButton(
                            stringResource(if (page.allListed) R.string.collection_watchlist_added else R.string.collection_watchlist_add),
                            onClick = actions.listAll,
                            icon = if (page.allListed) SlouchcatIcon.BookmarkOn.vector else SlouchcatIcon.Bookmark.vector,
                            modifier = Modifier.focusKey(focus, "list"),
                        )
                        TvButton(
                            stringResource(
                                when (page.watched) {
                                    CollectionWatched.Full -> R.string.watched_state_full
                                    CollectionWatched.Half -> R.string.watched_state_half
                                    CollectionWatched.None -> R.string.watched_hint_set
                                },
                            ),
                            onClick = actions.watched,
                            icon = SlouchcatIcon.Check.vector,
                            modifier = Modifier.focusKey(focus, "seen"),
                        )
                    }
                    Text(stringResource(R.string.app_collections_members), style = TvType.h2, modifier = Modifier.padding(top = TvDimens.xl).semantics { heading() })
                    CompositionLocalProvider(LocalContentStart provides 0.dp) {
                        val infos = page.items.associate { it.id to it.info() }
                        TvCardRow(title = null, items = page.items, key = { it.id }, focus = focus, rowKey = "part", info = { infos[it.id] }) { item, modifier ->
                            MediaCard(item, actions.item?.let { open -> { open(item) } }, modifier, tag = watchedTag(item))
                        }
                    }
                }
            }
        }
    }
    focus.StartFocus(if (page == null) RETRY else if (actions.play != null) "play" else "list", state.page::class)
    state.asking?.let { asking ->
        val cancel = remember { FocusRequester() }
        val close = {
            actions.dismiss()
            focus.restore()
        }
        val name = page?.collection?.name.orEmpty()
        TvDialog(stringResource(if (asking == CollectionWatched.Half) R.string.watched_ask_halfTitle else R.string.watched_ask_removeTitle, name), onDismiss = close, initialFocus = cancel) {
            TvActions(vertical = true) {
                if (asking == CollectionWatched.Half) TvButton(stringResource(R.string.watched_ask_markAll), onClick = { actions.confirm(true); focus.restore() }, icon = SlouchcatIcon.Check.vector)
                TvButton(stringResource(R.string.watched_ask_removeAll), onClick = { actions.confirm(false); focus.restore() }, icon = SlouchcatIcon.Close.vector)
                TvButton(stringResource(R.string.common_cancel), onClick = close, kind = TvButtonKind.Primary, modifier = Modifier.focusRequester(cancel))
            }
        }
    }
}

/** «4 Titel · 2002–2007». */
@Composable
private fun collectionFacts(page: CollectionPage): String {
    val first = page.collection.firstYear
    val last = page.collection.lastYear
    val years = when {
        first == null -> null
        last == null || last == first -> first.toString()
        else -> stringResource(R.string.collection_years, first.toString(), last.toString())
    }
    return listOfNotNull(pluralCount(R.plurals.collection_count, page.collection.total), years).joinToString(stringResource(R.string.format_separator))
}

private const val POSTER = 2f / 3f
