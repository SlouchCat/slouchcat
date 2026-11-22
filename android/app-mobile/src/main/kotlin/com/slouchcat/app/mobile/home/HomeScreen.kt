package com.slouchcat.app.mobile.home

import android.view.accessibility.AccessibilityManager
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.PagerState
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.CardTag
import com.slouchcat.app.mobile.media.FskBadge
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.RowHeader
import com.slouchcat.app.mobile.media.kindName
import com.slouchcat.app.mobile.media.remainingText
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.app.mobile.shell.LocalShell
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.home.HomeSection
import com.slouchcat.core.browse.home.HomeUiState
import com.slouchcat.core.browse.home.StageDetails
import com.slouchcat.core.browse.home.LibraryChip
import com.slouchcat.core.design.Avatar
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatChip
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.CardShape
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.launch

/**
 * Where the start page leads. A `null` target has no page yet (its feature is not registered):
 * the card, title or button that would open it is no control and the bell is not drawn.
 */
data class HomeLinks(
    val item: ((MediaItem) -> Unit)? = null,
    val play: ((MediaItem) -> Unit)? = null,
    val showAll: (HomeSection.Cards) -> (() -> Unit)? = { null },
    val collection: ((CollectionRef) -> Unit)? = null,
    val collections: (() -> Unit)? = null,
    val genre: ((String) -> Unit)? = null,
    val library: ((LibraryChip) -> Unit)? = null,
    val notifications: (() -> Unit)? = null,
    val profiles: () -> Unit = {},
)

/**
 * The start page (mockup `/`): the bar with the menu of the media choice, the wordmark, the bell
 * and the profile; the stage; the rows in the server's order. Pull to refresh. From 700 dp the
 * stage is a band with its text on the left (reference photo `handy-tablet/01-start`).
 * [rotate]: the stage moves on by itself (off in tests).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    state: HomeUiState,
    links: HomeLinks,
    onRetry: () -> Unit,
    onRefresh: () -> Unit,
    onRowEnd: (String) -> Unit,
    onWatchlist: (MediaItem) -> Unit,
    modifier: Modifier = Modifier,
    snackbar: SnackbarHostState? = null,
    rotate: Boolean = true,
    /** The stage shows a title: its genres are read once (`HomeViewModel.onStage`). */
    onStage: (String) -> Unit = {},
) {
    CompositionLocalProvider(LocalStage provides StageInfo(state.stage, onStage)) {
        HomeContent(state, links, onRetry, onRefresh, onRowEnd, onWatchlist, modifier, snackbar, rotate)
    }
}

/** The details of the titles on the stage and the call that asks for them, for the stage deep in the rows. */
private class StageInfo(val details: Map<String, StageDetails>, val onStage: (String) -> Unit)

private val LocalStage = staticCompositionLocalOf { StageInfo(emptyMap()) {} }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun HomeContent(
    state: HomeUiState,
    links: HomeLinks,
    onRetry: () -> Unit,
    onRefresh: () -> Unit,
    onRowEnd: (String) -> Unit,
    onWatchlist: (MediaItem) -> Unit,
    modifier: Modifier,
    snackbar: SnackbarHostState?,
    rotate: Boolean,
) {
    Box(modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
        PullToRefreshBox(isRefreshing = state.refreshing, onRefresh = onRefresh, modifier = Modifier.fillMaxSize()) {
            val rows = state.rows
            val stageLeads = (rows as? Loadable.Ready)?.value?.firstOrNull() is HomeSection.Spotlight
            LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 24.dp)) {
                if (!stageLeads) item(key = "bar") { HomeBar(state.profile, links, overlay = false) }
                when (rows) {
                    is Loadable.Ready -> rows.value.forEachIndexed { index, section ->
                        item(key = section.id) {
                            HomeRow(section, links, onRowEnd, onWatchlist, rotate, bar = if (index == 0 && stageLeads) ({ HomeBar(state.profile, links, overlay = true) }) else null)
                        }
                    }
                    else -> item(key = "state") {
                        LoadableContent(rows, onRetry = onRetry, modifier = Modifier.padding(horizontal = gutter())) {}
                        if (rows is Loadable.Empty) {
                            MediaEmpty(stringResource(R.string.home_empty_title), body = stringResource(R.string.home_empty_body), icon = SlouchcatIcon.Film)
                        }
                    }
                }
            }
        }
        if (snackbar != null) SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
}

/** `bar over` of the start page: menu, wordmark, bell, profile. */
@Composable
private fun HomeBar(profile: ActiveProfile?, links: HomeLinks, overlay: Boolean, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val menu = LocalShell.current.openMenu
    Row(
        modifier
            .fillMaxWidth()
            .then(if (overlay) Modifier else Modifier.background(c.bg))
            .heightIn(min = 60.dp)
            .padding(start = 4.dp, end = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        if (menu != null) SlouchcatIconButton(SlouchcatIcon.More, contentDescription = stringResource(R.string.app_shell_menuOpen), onClick = menu)
        val name = stringResource(R.string.app_name)
        val split = if (name.endsWith("cat")) name.length - 3 else name.length
        Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = c.tx)) { append(name.substring(0, split)) }
                withStyle(SpanStyle(color = c.accent)) { append(name.substring(split)) }
            },
            style = SlouchcatType.strong.copy(fontSize = 22.sp, fontWeight = FontWeight.ExtraBold, lineHeight = 26.sp),
            modifier = Modifier.weight(1f).padding(start = if (menu != null) 2.dp else 12.dp).semantics { heading() },
        )
        if (links.notifications != null) {
            SlouchcatIconButton(SlouchcatIcon.Bell, contentDescription = stringResource(R.string.nav_notifications), onClick = links.notifications)
        }
        if (profile != null) {
            val label = stringResource(R.string.nav_switchProfile, profile.name)
            Box(
                Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .clickable(role = Role.Button, onClick = links.profiles)
                    .semantics { contentDescription = label },
                contentAlignment = Alignment.Center,
            ) {
                Avatar(profile.name, profile.accent, size = 34.dp)
            }
        }
    }
}

@Composable
private fun HomeRow(
    section: HomeSection,
    links: HomeLinks,
    onRowEnd: (String) -> Unit,
    onWatchlist: (MediaItem) -> Unit,
    rotate: Boolean,
    bar: (@Composable () -> Unit)?,
) {
    when (section) {
        is HomeSection.Spotlight -> Spotlight(section, links, onWatchlist, rotate, bar)
        is HomeSection.Cards -> CardsRow(section, links, onRowEnd)
        is HomeSection.Collections -> {
            Column {
                RowHeader(stringResource(R.string.home_rows_collections), links.collections)
                LazyRow(contentPadding = PaddingValues(horizontal = gutter()), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(section.collections, key = { it.first.id }) { (ref, card) ->
                        MediaCard(card, onClick = links.collection?.let { open -> { open(ref) } }, caption = titleCount(ref.count))
                    }
                }
            }
        }
        is HomeSection.Genres -> if (links.genre != null) {
            Column {
                RowHeader(stringResource(R.string.home_rows_genres), null)
                ChipRow {
                    items(section.genres, key = { it.name }) { genre -> SlouchcatChip(genre.name, selected = false, onClick = { links.genre.invoke(genre.name) }) }
                }
            }
        }
        is HomeSection.Libraries -> if (links.library != null) {
            Column {
                RowHeader(stringResource(R.string.home_rows_libraries), null)
                ChipRow {
                    items(section.libraries, key = { it.id }) { library ->
                        SlouchcatChip(library.name, selected = false, onClick = { links.library.invoke(library) }, icon = libraryIcon(library.kind, library.seriesOnly), modifier = Modifier.heightIn(min = 48.dp))
                    }
                }
            }
        }
    }
}

@Composable
private fun ChipRow(content: androidx.compose.foundation.lazy.LazyListScope.() -> Unit) {
    LazyRow(
        contentPadding = PaddingValues(horizontal = gutter()),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        modifier = Modifier.padding(bottom = 6.dp),
        content = content,
    )
}

/** The title of a row of cards (`homeRowTitle` of the web app). */
@Composable
fun rowTitle(section: HomeSection.Cards): String = when (section.key) {
    HomeRowKey.SPOTLIGHT -> stringResource(R.string.home_rows_spotlight)
    HomeRowKey.PICKS -> stringResource(R.string.home_picks)
    HomeRowKey.CONTINUE -> stringResource(R.string.home_rows_continue)
    HomeRowKey.RECENT -> stringResource(R.string.home_rows_recent)
    HomeRowKey.RELEASED -> stringResource(R.string.home_rows_released)
    HomeRowKey.WATCHLIST -> stringResource(R.string.home_rows_watchlist)
    HomeRowKey.COLLECTIONS -> stringResource(R.string.home_rows_collections)
    HomeRowKey.GENRES -> stringResource(R.string.home_rows_genres)
    HomeRowKey.LIBRARIES -> stringResource(R.string.home_rows_libraries)
    HomeRowKey.LIBRARY -> stringResource(R.string.home_rows_library, section.libraryName.orEmpty())
    HomeRowKey.RADIO -> stringResource(R.string.home_rows_radio)
    HomeRowKey.WISHES -> stringResource(R.string.home_rows_wishes)
    HomeRowKey.UNKNOWN_DEFAULT_OPEN_API -> ""
}

/** The caption of a card of «Weiterschauen»: «S1 · F2 · Noch 18 min», «Noch 1 h 12 min», «S1 · F3 · Neu». */
@Composable
private fun continueCaption(item: MediaItem): String? {
    val separator = stringResource(R.string.format_separator)
    val season = item.seasonNumber
    val number = item.episodeNumber
    val code = if (item.kind == ItemKind.EPISODE && season != null && number != null) {
        val end = item.episodeEnd
        if (end != null && end > number) {
            stringResource(R.string.home_continue_episodeRange, season.toString(), number.toString(), end.toString())
        } else {
            stringResource(R.string.home_continue_episode, season.toString(), number.toString())
        }
    } else {
        null
    }
    val state = remainingText(item) ?: if (code != null) stringResource(R.string.home_continue_new) else null
    return listOfNotNull(code, state).takeIf { it.isNotEmpty() }?.joinToString(separator)
}

@Composable
private fun CardsRow(section: HomeSection.Cards, links: HomeLinks, onRowEnd: (String) -> Unit) {
    val title = rowTitle(section)
    val list = rememberLazyListState()
    val atEnd by remember(list) {
        derivedStateOf {
            val info = list.layoutInfo
            info.totalItemsCount > 0 && (info.visibleItemsInfo.lastOrNull()?.index ?: -1) >= info.totalItemsCount - 3
        }
    }
    LaunchedEffect(list, section.items.size, section.complete) {
        if (!section.complete) snapshotFlow { atEnd }.distinctUntilChanged().filter { it }.collect { onRowEnd(section.id) }
    }
    val tip = CardTag(stringResource(R.string.app_browse_tip), SlouchcatIcon.Heart)
    Column {
        RowHeader(title, links.showAll(section))
        LazyRow(
            state = list,
            contentPadding = PaddingValues(horizontal = gutter()),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.padding(bottom = 4.dp),
        ) {
            items(section.items, key = { it.id }) { item ->
                if (section.key == HomeRowKey.CONTINUE) {
                    // «Weiterschauen» plays where it stopped (mockup `#/play/i`); until the player exists the card opens the title.
                    val open = links.play ?: links.item
                    MediaCard(item, onClick = open?.let { { it(item) } }, shape = CardShape.Wide, caption = continueCaption(item))
                } else {
                    MediaCard(item, onClick = links.item?.let { { it(item) } }, tag = if (section.key == HomeRowKey.PICKS) tip else null)
                }
            }
            if (section.more != MoreState.Idle) {
                item(key = "more") {
                    Box(Modifier.width(64.dp).padding(top = 40.dp), contentAlignment = Alignment.Center) {
                        if (section.more == MoreState.Loading) {
                            androidx.compose.material3.CircularProgressIndicator(color = SlouchcatTheme.colors.tx2, modifier = Modifier.size(24.dp))
                        } else {
                            SlouchcatIconButton(SlouchcatIcon.Refresh, contentDescription = stringResource(R.string.app_state_retry), onClick = { onRowEnd(section.id) })
                        }
                    }
                }
            }
        }
    }
}

/** The stage (mockup `hero()`): one title at a time, wiped or by the dots, moving on by itself. */
@Composable
private fun Spotlight(section: HomeSection.Spotlight, links: HomeLinks, onWatchlist: (MediaItem) -> Unit, rotate: Boolean, bar: (@Composable () -> Unit)?) {
    val wide = isWideLayout()
    val pager = rememberPagerState(initialPage = section.start) { section.items.size }
    val scope = rememberCoroutineScope()
    // The stage holds still while a screen reader explores, while it has the focus and while a finger
    // rests on it (WCAG 2.2.2, code review m-5 of part a); after each change of title the time starts again.
    val exploring = touchExploration()
    val stage = LocalStage.current
    LaunchedEffect(pager.currentPage, section.items) { section.items.getOrNull(pager.currentPage)?.let { stage.onStage(it.id) } }
    var focused by remember { mutableStateOf(false) }
    var pressed by remember { mutableStateOf(false) }
    if (rotate && section.items.size > 1 && !exploring && !focused && !pressed) {
        LaunchedEffect(pager, section.intervalSeconds, pager.currentPage) {
            delay(section.intervalSeconds * 1_000L)
            // The number of titles now, not of the first drawing (code review L-6 of part a).
            if (!pager.isScrollInProgress && pager.pageCount > 0) pager.animateScrollToPage((pager.currentPage + 1) % pager.pageCount)
        }
    }
    val region = stringResource(R.string.home_rows_spotlight)
    Box(
        Modifier
            .fillMaxWidth()
            .semantics { contentDescription = region }
            .onFocusChanged { focused = it.hasFocus }
            .pointerInput(Unit) {
                awaitPointerEventScope {
                    while (true) {
                        val event = awaitPointerEvent(PointerEventPass.Initial)
                        pressed = event.changes.any { it.pressed }
                    }
                }
            },
    ) {
        HorizontalPager(state = pager, modifier = Modifier.fillMaxWidth(), key = { section.items[it].id }) { page ->
            Slide(section.items[page], links, onWatchlist, wide)
        }
        if (bar != null) Box(Modifier.align(Alignment.TopStart).fillMaxWidth()) { bar() }
        Dots(pager, section.items.size, wide, Modifier.align(if (wide) Alignment.BottomStart else Alignment.BottomCenter)) { scope.launch { pager.animateScrollToPage(it) } }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Slide(item: MediaItem, links: HomeLinks, onWatchlist: (MediaItem) -> Unit, wide: Boolean) {
    val c = SlouchcatTheme.colors
    Box(Modifier.fillMaxWidth().heightIn(min = if (wide) 440.dp else 520.dp)) {
        MediaImage(item.backdrop.or(item.poster), item.title, Modifier.matchParentSize(), placeholder = false)
        val shade = if (wide) {
            Brush.horizontalGradient(0f to c.bg.copy(alpha = 0.92f), 0.45f to c.bg.copy(alpha = 0.5f), 0.75f to Color.Transparent)
        } else {
            Brush.verticalGradient(0f to c.bg.copy(alpha = 0.55f), 0.26f to Color.Transparent, 0.55f to c.bg.copy(alpha = 0.35f), 0.96f to c.bg)
        }
        Box(Modifier.matchParentSize().background(shade))
        if (wide) Box(Modifier.matchParentSize().background(Brush.verticalGradient(0.6f to Color.Transparent, 1f to c.bg)))
        Column(
            Modifier
                .align(if (wide) Alignment.BottomStart else Alignment.BottomCenter)
                .then(if (wide) Modifier.widthIn(max = 552.dp).padding(start = 32.dp, end = 24.dp) else Modifier.fillMaxWidth().padding(horizontal = gutter()))
                .padding(top = 84.dp, bottom = if (wide) 44.dp else 34.dp),
            horizontalAlignment = if (wide) Alignment.Start else Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (!item.logo.isEmpty) {
                MediaImage(
                    item.logo,
                    item.title,
                    Modifier.heightIn(max = 84.dp).fillMaxWidth(if (wide) 0.8f else 0.78f).heightIn(min = 64.dp).semantics { contentDescription = item.title },
                    contentScale = ContentScale.Fit,
                    placeholder = false,
                    surface = false,
                    alignment = if (wide) Alignment.CenterStart else Alignment.Center,
                )
            } else {
                Text(
                    item.title,
                    style = SlouchcatType.pageTitle.copy(fontSize = 28.sp, lineHeight = 32.sp, fontWeight = FontWeight.ExtraBold),
                    color = c.tx,
                    textAlign = if (wide) TextAlign.Start else TextAlign.Center,
                    modifier = Modifier.semantics { heading() },
                )
            }
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp, if (wide) Alignment.Start else Alignment.CenterHorizontally),
                verticalArrangement = Arrangement.spacedBy(4.dp),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                val meta = SlouchcatType.caption.copy(fontSize = 13.5.sp)
                item.year?.let { Text(it.toString(), style = meta, color = c.tx2) }
                item.rating?.let { FskBadge(it) }
                val length = if (item.kind == ItemKind.SERIES) kindName(item.kind) else item.runtimeMinutes?.let { runtimeText(it) }
                length?.let { Text(it, style = meta, color = c.tx2) }
                // The genres after year, rating and length (mockup `hero()`), once `getItem` has them.
                LocalStage.current.details[item.id]?.genres?.takeIf { it.isNotEmpty() }?.let {
                    Text(it.take(2).joinToString(stringResource(R.string.format_separator)), style = meta, color = c.tx2)
                }
            }
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(10.dp, if (wide) Alignment.Start else Alignment.CenterHorizontally),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                itemVerticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.padding(top = 4.dp),
            ) {
                links.play?.let { play -> SlouchcatButton(stringResource(R.string.item_play), onClick = { play(item) }, leadingIcon = SlouchcatIcon.Play) }
                SlouchcatButton(
                    stringResource(if (item.inWatchlist) R.string.item_watchlist_added else R.string.item_watchlist_add),
                    onClick = { onWatchlist(item) },
                    kind = ButtonKind.Secondary,
                    leadingIcon = if (item.inWatchlist) SlouchcatIcon.BookmarkOn else SlouchcatIcon.Bookmark,
                    modifier = Modifier.semantics { selected = item.inWatchlist },
                )
                links.item?.let { open -> SlouchcatIconButton(SlouchcatIcon.Info, contentDescription = stringResource(R.string.app_home_details, item.title), onClick = { open(item) }) }
            }
        }
    }
}

/** Whether a screen reader explores by touch (TalkBack), following its changes. */
@Composable
private fun touchExploration(): Boolean {
    val manager = LocalContext.current.getSystemService(AccessibilityManager::class.java) ?: return false
    var on by remember { mutableStateOf(manager.isTouchExplorationEnabled) }
    DisposableEffect(manager) {
        val listener = AccessibilityManager.TouchExplorationStateChangeListener { on = it }
        manager.addTouchExplorationStateChangeListener(listener)
        onDispose { manager.removeTouchExplorationStateChangeListener(listener) }
    }
    return on
}

/** The dots under the stage: one per title, the current one long; each jumps to its title. */
@Composable
private fun Dots(pager: PagerState, count: Int, wide: Boolean, modifier: Modifier, onDot: (Int) -> Unit) {
    if (count < 2) return
    val c = SlouchcatTheme.colors
    Row(modifier.padding(start = if (wide) 18.dp else 0.dp, bottom = 0.dp), horizontalArrangement = Arrangement.Center) {
        for (index in 0 until count) {
            val current = pager.currentPage == index
            val label = stringResource(R.string.app_home_slide, count, (index + 1).toString())
            Box(
                Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .clickable(role = Role.Tab, onClick = { onDot(index) })
                    .semantics {
                        contentDescription = label
                        selected = current
                    },
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    Modifier
                        .size(width = if (current) 20.dp else 7.dp, height = 7.dp)
                        .clip(RoundedCornerShape(4.dp))
                        .background(if (current) c.tx else c.tx.copy(alpha = 0.4f)),
                )
            }
        }
    }
}

/** The symbol of a library (mockup `LIBS`: film, tv, headphones, mic). */
fun libraryIcon(kind: com.slouchcat.core.api.generated.models.LibraryKind, seriesOnly: Boolean): SlouchcatIcon = when (kind) {
    com.slouchcat.core.api.generated.models.LibraryKind.AUDIOBOOKS -> SlouchcatIcon.Headphones
    com.slouchcat.core.api.generated.models.LibraryKind.PODCASTS -> SlouchcatIcon.Mic
    else -> if (seriesOnly) SlouchcatIcon.Tv else SlouchcatIcon.Film
}
