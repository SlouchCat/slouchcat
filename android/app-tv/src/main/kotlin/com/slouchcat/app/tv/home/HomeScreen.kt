package com.slouchcat.app.tv.home

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.MediaCard
import com.slouchcat.app.tv.browse.MetaLine
import com.slouchcat.app.tv.browse.cardTitle
import com.slouchcat.app.tv.browse.episodeCode
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.info
import com.slouchcat.app.tv.browse.kindName
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.browse.screenWidth
import com.slouchcat.app.tv.browse.watchedTag
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.BackdropMode
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.CardShape
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBackdrop
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCardRow
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Where the start page leads; `null`: the page of that link is not registered (no control). */
class HomeLinks(
    val item: ((MediaItem) -> Unit)?,
    /** «Abspielen» on the stage and on a card of «Weiterschauen»: the player of stage 3. */
    val play: ((MediaItem) -> Unit)?,
    val showAll: (HomeSection.Cards) -> (() -> Unit)?,
    val collection: ((CollectionRef) -> Unit)?,
    val collections: (() -> Unit)?,
    val genre: ((String) -> Unit)?,
    val libraries: (() -> Unit)?,
)

/** An entry of a row: a card, or the tile «Alle anzeigen» at its end (`.tc.more`). */
private sealed interface RowEntry {
    val key: String

    data class Card(val item: MediaItem) : RowEntry {
        override val key: String get() = item.id
    }

    data object All : RowEntry {
        override val key: String = ALL
    }
}

private const val ALL = "all"
private const val HERO = "hero"
private const val RETRY = "retry"
private const val EMPTY = "empty"

/**
 * The start page (mockup `/`, `home.js`): the stage with backdrop, logo or title, facts, summary,
 * «Abspielen», «Merken», «Details» and a strip of the other titles, then the rows of `getHome` in
 * the server's order. The art layer follows the focus: the stage's title in full, a card's backdrop
 * darkened. Each row keeps its card, the page keeps the element of the focus across a detail page
 * and back. [rotate]: the stage moves on by itself while the focus rests on it (off in tests).
 */
@Composable
fun HomeScreen(
    state: HomeUiState,
    links: HomeLinks,
    onRetry: () -> Unit,
    onRowEnd: (String) -> Unit,
    onWatchlist: (MediaItem) -> Unit,
    rotate: Boolean = true,
) {
    val focus = rememberFocusMemory()
    var art by remember { mutableStateOf<Pair<MediaItem?, BackdropMode>>(null to BackdropMode.Hero) }
    val backdrop = art.first?.backdrop?.forWidth(screenWidth())
    Box(Modifier.fillMaxSize()) {
        TvBackdrop(backdrop, mode = art.second)
        when (val rows = state.rows) {
            is Loadable.Ready -> Rows(rows.value, links, focus, onRowEnd, onWatchlist, rotate) { item, mode -> art = item to mode }
            Loadable.Empty -> EmptyState(
                SlouchcatIcon.Home.vector,
                stringResource(R.string.home_empty_title),
                stringResource(R.string.home_empty_body),
                Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop),
                action = links.libraries?.let { open -> { EmptyAction(stringResource(R.string.home_empty_action), SlouchcatIcon.Film.vector, open, Modifier.focusKey(focus, EMPTY)) } },
            )
            else -> LoadableContent(
                state = rows,
                onRetry = onRetry,
                modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop),
                retryModifier = Modifier.focusKey(focus, RETRY),
            ) {}
        }
    }
    val start = when (val rows = state.rows) {
        is Loadable.Ready -> startKey(rows.value, links)
        Loadable.Empty -> EMPTY
        else -> RETRY
    }
    focus.StartFocus(start, state.rows::class)
}

/** The element a fresh start page focuses: «Abspielen» (or «Details») of the stage, else the first card. */
private fun startKey(rows: List<HomeSection>, links: HomeLinks): String {
    val first = rows.first()
    return when (first) {
        is HomeSection.Spotlight -> if (links.play != null) "$HERO-play" else "$HERO-details"
        is HomeSection.Cards -> "${first.id}-${first.items.first().id}"
        is HomeSection.Collections -> "${first.id}-${first.collections.first().first.id}"
        is HomeSection.Genres -> "${first.id}-${first.genres.first().name}"
        is HomeSection.Libraries -> "${first.id}-$ALL"
    }
}

@Composable
private fun Rows(
    rows: List<HomeSection>,
    links: HomeLinks,
    focus: FocusMemory,
    onRowEnd: (String) -> Unit,
    onWatchlist: (MediaItem) -> Unit,
    rotate: Boolean,
    onArt: (MediaItem?, BackdropMode) -> Unit,
) {
    val list = rememberLazyListState()
    val scope = rememberCoroutineScope()
    LazyColumn(state = list, modifier = Modifier.fillMaxSize(), contentPadding = androidx.compose.foundation.layout.PaddingValues(top = if (rows.first() is HomeSection.Spotlight) TvDimens.xs else TvDimens.safeY, bottom = TvDimens.Browse.DetailTop)) {
        items(rows, key = { it.id }) { row ->
            when (row) {
                is HomeSection.Spotlight -> Stage(
                    row,
                    links,
                    focus,
                    onWatchlist,
                    rotate,
                    onArt = { onArt(it, BackdropMode.Hero) },
                    onFocused = { scope.launch { list.animateScrollToItem(0) } },
                )
                is HomeSection.Cards -> CardsRow(row, links, focus, onRowEnd) { onArt(it, BackdropMode.Dim) }
                is HomeSection.Collections -> TvCardRow(
                    title = stringResource(R.string.home_rows_collections),
                    items = row.collections.map { it.second } + listOfNotNull(MediaItem(ALL, ItemKind.MOVIE, ALL).takeIf { links.collections != null }),
                    key = { it.id },
                    focus = focus,
                    rowKey = row.id,
                    info = { if (it.id == ALL) null else CardInfo(it.title, null) },
                ) { item, modifier ->
                    if (item.id == ALL) {
                        AllTile(stringResource(R.string.home_rows_collections), links.collections ?: {}, modifier)
                    } else {
                        val ref = row.collections.first { it.second.id == item.id }.first
                        MediaCard(item, links.collection?.let { open -> { open(ref) } }, modifier, subcaption = null)
                    }
                }
                is HomeSection.Genres -> TvCardRow(
                    title = stringResource(R.string.home_rows_genres),
                    items = row.genres,
                    key = { it.name },
                    focus = focus,
                    rowKey = row.id,
                ) { genre, modifier ->
                    ControlChip(genre.name, SlouchcatIcon.Filter.vector, onClick = { links.genre?.invoke(genre.name) }, modifier = modifier)
                }
                // The rail lists the libraries: the row «Meine Medien» of the phone stays away here.
                is HomeSection.Libraries -> Unit
            }
        }
    }
}

/** The title of a row of the start page (web `HomeRowTitle`). */
@Composable
private fun rowTitle(row: HomeSection.Cards): String = when (row.key) {
    HomeRowKey.CONTINUE -> stringResource(R.string.home_rows_continue)
    HomeRowKey.RECENT -> stringResource(R.string.home_rows_recent)
    HomeRowKey.WATCHLIST -> stringResource(R.string.home_rows_watchlist)
    HomeRowKey.SPOTLIGHT -> stringResource(R.string.home_rows_spotlight)
    HomeRowKey.RELEASED -> stringResource(R.string.home_rows_released)
    HomeRowKey.PICKS -> stringResource(R.string.home_picks)
    HomeRowKey.WISHES -> stringResource(R.string.home_rows_wishes)
    HomeRowKey.LIBRARY -> stringResource(R.string.home_rows_library, row.libraryName.orEmpty())
    else -> stringResource(R.string.home_title)
}

@Composable
private fun CardsRow(row: HomeSection.Cards, links: HomeLinks, focus: FocusMemory, onRowEnd: (String) -> Unit, onArt: (MediaItem?) -> Unit) {
    val all = links.showAll(row)
    val entries = row.items.map { RowEntry.Card(it) } + listOfNotNull(RowEntry.All.takeIf { all != null })
    val continuing = row.key == HomeRowKey.CONTINUE
    val title = rowTitle(row)
    val infos = row.items.associate { it.id to it.info() }
    TvCardRow(
        title = title,
        items = entries,
        key = { it.key },
        focus = focus,
        rowKey = row.id,
        info = { (it as? RowEntry.Card)?.let { card -> infos[card.item.id] } },
    ) { entry, modifier ->
        when (entry) {
            RowEntry.All -> AllTile(title, all ?: {}, modifier)
            is RowEntry.Card -> {
                val item = entry.item
                val index = row.items.indexOf(item)
                val near = Modifier.onFocusChanged {
                    if (it.isFocused) {
                        onArt(item)
                        if (index >= row.items.size - NEAR_END) onRowEnd(row.id)
                    }
                }
                if (continuing) {
                    // «Weiterschauen» plays where it stopped (mockup `href: #/play/<i>`); without the player it opens the title.
                    val open = links.play?.let { play -> { play(item) } } ?: links.item?.let { show -> { show(item) } }
                    MediaCard(
                        item,
                        open,
                        modifier.then(near),
                        shape = CardShape.Wide,
                        caption = item.cardTitle,
                        subcaption = continueLine(item),
                        progress = item.progress,
                    )
                } else {
                    MediaCard(item, links.item?.let { show -> { show(item) } }, modifier.then(near), tag = watchedTag(item))
                }
            }
        }
    }
}

/** «S1 · F3 · noch 18 min» or «Noch 18 min» under a card of «Weiterschauen». */
@Composable
private fun continueLine(item: MediaItem): String? {
    val left = item.remainingMinutes?.let { stringResource(R.string.home_remaining, runtimeText(it)) }
    val code = if (item.kind == ItemKind.EPISODE) episodeCode(item) else null
    return listOfNotNull(code, left).takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
}

/** `.tc.more`: the last tile of a row, «Alle anzeigen». */
@Composable
private fun AllTile(title: String, onClick: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val label = stringResource(R.string.app_browse_showAllLabel, title)
    Surface(
        onClick = onClick,
        modifier = modifier
            .size(CardShape.Poster.width, CardShape.Poster.width * POSTER_HEIGHT)
            .semantics { contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusCard)),
        colors = ClickableSurfaceDefaults.colors(containerColor = c.s1, contentColor = c.tx2, focusedContainerColor = c.tx, focusedContentColor = c.bg),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.line)),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx)),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.spacedBy(TvDimens.m, Alignment.CenterVertically), horizontalAlignment = Alignment.CenterHorizontally) {
            com.slouchcat.app.tv.ui.TvIcon(SlouchcatIcon.Grid, TvDimens.Browse.Icon)
            Text(stringResource(R.string.app_browse_showAll), style = TvType.bodyStrong)
        }
    }
}

/**
 * `.hero`: the stage. The thumbnails of the strip choose the title when they get the focus and
 * open it with OK; the stage moves on after the server's interval while the focus rests on its
 * buttons ([rotate]).
 */
@Composable
private fun Stage(
    row: HomeSection.Spotlight,
    links: HomeLinks,
    focus: FocusMemory,
    onWatchlist: (MediaItem) -> Unit,
    rotate: Boolean,
    onArt: (MediaItem) -> Unit,
    onFocused: () -> Unit,
) {
    var at by rememberSaveable(row.id) { mutableIntStateOf(row.start) }
    val index = at.coerceIn(0, row.items.lastIndex)
    val item = row.items[index]
    var onButtons by remember { mutableStateOf(false) }
    var inside by remember { mutableStateOf(false) }
    LaunchedEffect(inside, item.id) { if (inside) onArt(item) }
    LaunchedEffect(index, onButtons, rotate) {
        if (!rotate || !onButtons || row.items.size < 2) return@LaunchedEffect
        delay(row.intervalSeconds * MS_PER_SECOND)
        at = (index + 1) % row.items.size
    }
    val c = SlouchcatTheme.colors
    Column(
        Modifier
            .fillMaxWidth()
            .heightIn(min = TvDimens.Browse.HeroHeight)
            .padding(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.Browse.HeroTop)
            .onFocusChanged {
                if (it.hasFocus && !inside) onFocused()
                inside = it.hasFocus
            },
    ) {
        Column(Modifier.widthIn(max = TvDimens.Browse.HeroInfo).heightIn(min = TvDimens.Browse.HeroInfoMin), verticalArrangement = Arrangement.Bottom) {
            kindName(item.kind)?.let { KindLine(it, Modifier.padding(bottom = TvDimens.Browse.KindGap)) }
            TitleOrLogo(item, TvDimens.Browse.LogoHeight, TvDimens.Browse.LogoWidth)
            MetaLine(heroFacts(item), Modifier.padding(top = TvDimens.Browse.MetaTop), rating = item.rating)
            row.overviews.getOrNull(index)?.takeIf { it.isNotBlank() }?.let {
                Text(it, style = TvType.body, color = c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.m).widthIn(max = TvDimens.Browse.Overview))
            }
        }
        Row(
            Modifier.padding(top = TvDimens.Browse.ActionsTop).onFocusChanged { onButtons = it.hasFocus },
            horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
        ) {
            links.play?.let { play ->
                TvButton(stringResource(R.string.item_play), onClick = { play(item) }, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, "$HERO-play"))
            }
            TvButton(
                stringResource(if (item.inWatchlist) R.string.item_watchlist_added else R.string.item_watchlist_add),
                onClick = { onWatchlist(item) },
                icon = if (item.inWatchlist) SlouchcatIcon.BookmarkOn.vector else SlouchcatIcon.Bookmark.vector,
                modifier = Modifier.focusKey(focus, "$HERO-watch"),
            )
            links.item?.let { show ->
                TvButton(
                    stringResource(R.string.item_details),
                    onClick = { show(item) },
                    icon = SlouchcatIcon.Info.vector,
                    kind = if (links.play == null) TvButtonKind.Primary else TvButtonKind.Standard,
                    modifier = Modifier.focusKey(focus, "$HERO-details"),
                )
            }
        }
        if (row.items.size > 1) {
            Row(Modifier.padding(top = TvDimens.Browse.StripTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                row.items.forEachIndexed { k, other ->
                    Thumb(
                        other,
                        current = k == index,
                        label = stringResource(R.string.app_home_slide, row.items.size, (k + 1).toString()) + ", " + other.title,
                        onFocus = { at = k },
                        onClick = { links.item?.invoke(other) },
                        modifier = Modifier.focusKey(focus, "$HERO-$k"),
                    )
                }
            }
        }
        Spacer(Modifier.height(TvDimens.xl))
    }
}

/** `.sp-th`: a thumbnail of the strip, the current one with a bar of the accent. */
@Composable
private fun Thumb(item: MediaItem, current: Boolean, label: String, onFocus: () -> Unit, onClick: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusCard)
    Surface(
        onClick = onClick,
        modifier = modifier
            .size(TvDimens.Browse.ThumbWidth, TvDimens.Browse.ThumbHeight)
            .onFocusChanged { if (it.isFocused) onFocus() }
            .semantics { contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(containerColor = c.s2, focusedContainerColor = c.s2),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.Card.Ring, c.tx), inset = -TvDimens.Card.RingGap)),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Box(Modifier.fillMaxSize()) {
            TvImage(item.backdrop.or(item.poster).forWidth(TvDimens.Browse.ThumbWidth), Modifier.fillMaxSize())
            if (!current) Box(Modifier.fillMaxSize().background(c.bg.copy(alpha = 1f - TvMotion.profileRest)))
            if (current) {
                Box(
                    Modifier
                        .align(Alignment.BottomCenter)
                        .padding(horizontal = TvDimens.s, vertical = TvDimens.xs)
                        .fillMaxWidth()
                        .height(TvDimens.Card.Progress)
                        .background(c.accent, RoundedCornerShape(TvDimens.Card.Progress)),
                )
            }
        }
    }
}

/** The title as its logo when the server has one (`.sp-logo`), else as text (`.sp-title`). */
@Composable
fun TitleOrLogo(item: MediaItem, height: androidx.compose.ui.unit.Dp, width: androidx.compose.ui.unit.Dp) {
    val logo = item.logo.forWidth(width)
    if (logo != null) {
        TvImage(
            logo,
            Modifier
                .heightIn(max = height)
                .widthIn(max = width)
                .height(height)
                .semantics { contentDescription = item.title },
            contentScale = ContentScale.Fit,
            // `.sp-logo`: object-position left bottom.
            alignment = Alignment.BottomStart,
        )
    } else {
        Text(item.title, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis)
    }
}

/** Year, then the length or «Serie» (the rating goes second, in [MetaLine]). */
@Composable
private fun heroFacts(item: MediaItem): List<String> = buildList {
    item.year?.let { add(it.toString()) }
    if (item.kind == ItemKind.SERIES) kindName(item.kind)?.let(::add) else item.runtimeMinutes?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
}

private const val NEAR_END = 4
private const val MS_PER_SECOND = 1000L
private const val POSTER_HEIGHT = 1.5f
