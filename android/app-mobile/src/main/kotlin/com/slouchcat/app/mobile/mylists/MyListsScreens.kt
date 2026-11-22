package com.slouchcat.app.mobile.mylists

import android.text.format.DateFormat
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.media.MediaCard
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaGrid
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.defaultCaption
import com.slouchcat.app.mobile.media.fullWidth
import com.slouchcat.app.mobile.media.titleCount
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * «Merkliste» (mockup `/watchlist`, reference photo handy 25): the bar with «Bearbeiten», the
 * count, the titles as a grid; in «Bearbeiten» each card has its ✕. Empty: the mockup's notice with
 * «Zur Startseite».
 */
@Composable
fun WatchlistScreen(
    state: MyListUiState<MediaItem>,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onMore: () -> Unit,
    onToggleEdit: () -> Unit,
    onRemove: (MediaItem) -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    onHome: () -> Unit,
    modifier: Modifier = Modifier,
) {
    ListPage(
        title = stringResource(R.string.nav_watchlist),
        state = state,
        id = { it.id },
        snackbar = snackbar,
        onBack = onBack,
        onRetry = onRetry,
        onMore = onMore,
        onToggleEdit = onToggleEdit,
        empty = {
            MediaEmpty(
                stringResource(R.string.watchlist_empty_title),
                body = stringResource(R.string.watchlist_empty_body),
                icon = SlouchcatIcon.Bookmark,
                actionLabel = stringResource(R.string.app_lists_toStart),
                action = onHome,
            )
        },
        modifier = modifier,
    ) { item ->
        EditableCard(
            item = item,
            caption = defaultCaption(item),
            editing = state.editing,
            busy = item.id in state.busy,
            removeLabel = stringResource(R.string.watchlist_removeLabel, item.title),
            onRemove = { onRemove(item) },
            onOpen = onOpen(item.id),
        )
    }
}

/**
 * «Gesehen» (mockup `/watched`, reference photo handy 26): as the watchlist, each card with «gesehen
 * am 9. Nov.» or «19 von 96 Folgen · zuletzt 9. Nov.» and the bar of a series; the ✕ asks before
 * the check mark goes ([asking] opens the sheet).
 */
@Composable
fun WatchedScreen(
    state: MyListUiState<WatchedCard>,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onMore: () -> Unit,
    onToggleEdit: () -> Unit,
    onAsk: (WatchedCard) -> Unit,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
    onOpen: (String) -> (() -> Unit)?,
    onHome: () -> Unit,
    modifier: Modifier = Modifier,
) {
    ListPage(
        title = stringResource(R.string.watched_page_title),
        state = state,
        id = { it.id },
        snackbar = snackbar,
        onBack = onBack,
        onRetry = onRetry,
        onMore = onMore,
        onToggleEdit = onToggleEdit,
        empty = {
            MediaEmpty(
                stringResource(R.string.watched_page_empty_title),
                body = stringResource(R.string.watched_page_empty_body),
                icon = SlouchcatIcon.Eye,
                actionLabel = stringResource(R.string.app_lists_toStart),
                action = onHome,
            )
        },
        modifier = modifier,
    ) { card ->
        val total = card.episodeCount
        val seen = if (card.item.watched) total else card.watchedCount ?: 0
        val shown = if (total != null && total > 0) card.item.copy(progress = (seen ?: 0).toFloat() / total) else card.item.copy(progress = null)
        EditableCard(
            item = shown,
            caption = watchedCaption(card),
            editing = state.editing,
            busy = card.id in state.busy,
            removeLabel = stringResource(R.string.app_lists_unwatchLabel, card.item.title),
            onRemove = { onAsk(card) },
            onOpen = onOpen(card.id),
        )
    }
    state.asking?.let { card ->
        val title = stringResource(R.string.watched_ask_removeTitle, card.item.title)
        SlouchcatSheet(title, onDismiss = onDismiss) {
            SheetHeadIcon(SlouchcatIcon.Eye, warning = true)
            SheetTitle(title, centered = true)
            SheetText(stringResource(R.string.watched_ask_removeBodyAll), centered = true)
            SheetActions {
                SlouchcatButton(stringResource(R.string.common_cancel), onClick = onDismiss, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
                SlouchcatButton(stringResource(R.string.watched_ask_removeAll), onClick = onConfirm, kind = ButtonKind.Danger, modifier = Modifier.weight(1f))
            }
        }
    }
}

/** «gesehen am 9. Nov.», «gehört am …», «19 von 96 Folgen · zuletzt 9. Nov.» (web `describe`). */
@Composable
private fun watchedCaption(card: WatchedCard): String {
    val date = dayMonth(card.lastWatchedAt)
    val total = card.episodeCount
    if (total == null) {
        return stringResource(if (card.item.isAudio) R.string.watched_page_heardOn else R.string.watched_page_watchedOn, date)
    }
    val seen = if (card.item.watched) total else card.watchedCount ?: 0
    return pluralStringResource(R.plurals.watched_page_episodes, seen, seen, total, date)
}

/** «9. Nov.» in the language of the device (web `formatDayMonth`). */
@Composable
private fun dayMonth(at: OffsetDateTime): String {
    val locale = LocalConfiguration.current.locales[0]
    val format = remember(locale) { DateTimeFormatter.ofPattern(DateFormat.getBestDateTimePattern(locale, "dMMM"), locale) }
    return at.atZoneSameInstant(ZoneId.systemDefault()).format(format)
}

/** The frame both lists share: bar with «Bearbeiten», count line, grid, empty notice, toasts. */
@Composable
private fun <T> ListPage(
    title: String,
    state: MyListUiState<T>,
    id: (T) -> String,
    snackbar: SnackbarHostState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onMore: () -> Unit,
    onToggleEdit: () -> Unit,
    empty: @Composable () -> Unit,
    modifier: Modifier = Modifier,
    card: @Composable (T) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val shown = state.shown(id)
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(
                title,
                navigationLabel = stringResource(R.string.common_back),
                onNavigation = onBack,
                actions = {
                    if (shown is com.slouchcat.core.media.Loadable.Ready) {
                        SlouchcatButton(
                            stringResource(if (state.editing) R.string.app_lists_done else R.string.app_lists_edit),
                            onClick = onToggleEdit,
                            kind = ButtonKind.Ghost,
                            small = true,
                        )
                    }
                },
            )
            LoadableContent(shown, onRetry = onRetry, empty = empty) { list ->
                MediaGrid(
                    list,
                    key = id,
                    onNearEnd = onMore,
                    header = {
                        fullWidth("count") {
                            Text(
                                if (state.editing) pluralStringResource(R.plurals.app_lists_editing, state.total, state.total) else titleCount(state.total),
                                style = SlouchcatType.caption.copy(fontSize = 14.sp),
                                color = c.tx2,
                                modifier = Modifier.padding(bottom = 2.dp).semantics { liveRegion = LiveRegionMode.Polite },
                            )
                        }
                    },
                    footer = { fullWidth("more") { PagingFooter(state.entries.more, onMore) } },
                ) { card(it) }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

/** A card of a list with, in «Bearbeiten», its ✕ in the corner (`.bx-x`). */
@Composable
private fun EditableCard(
    item: MediaItem,
    caption: String?,
    editing: Boolean,
    busy: Boolean,
    removeLabel: String,
    onRemove: () -> Unit,
    onOpen: (() -> Unit)?,
) {
    val c = SlouchcatTheme.colors
    Box {
        MediaCard(item, onClick = onOpen, caption = caption, width = null)
        if (editing) {
            Box(Modifier.align(Alignment.TopEnd), contentAlignment = Alignment.Center) {
                Box(
                    Modifier
                        .size(34.dp)
                        .clip(CircleShape)
                        .background(c.scrim)
                        .border(BorderStroke(1.dp, c.frame), CircleShape),
                )
                SlouchcatIconButton(SlouchcatIcon.Close, contentDescription = removeLabel, onClick = onRemove, enabled = !busy, tint = c.tx)
            }
        }
    }
}
