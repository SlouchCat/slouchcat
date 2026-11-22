package com.slouchcat.app.tv.notifications

import android.text.format.DateUtils
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.EmptyAction
import com.slouchcat.app.tv.browse.EmptyState
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.browse.notifications.NotificationsUiState
import com.slouchcat.core.browse.notifications.NotificationsViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.NotificationTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.aboutWish
import com.slouchcat.core.strings.R

/** Where an entry leads ([NotificationTarget]) and «Start» for the empty page. */
class NotificationsLinks(val open: (NotificationTarget) -> Unit, val home: (() -> Unit)?)

/** Focus keys of «Benachrichtigungen». */
object NotificationsKeys {
    fun announcement(id: Long) = "ann-$id"
    fun entry(id: Long) = "nt-$id"
    const val ALL = "all"
    const val HOME = "home"
    const val RETRY = "retry"
    const val MORE = "more"
}

private val W = SlouchcatTokens.Tv.Size.Wishes

/**
 * «Benachrichtigungen» (mockup `/notifications`, reference photo tv 35): «2 ungelesen», the open
 * announcements of the server in a frame with «Verstanden», «Alle als gelesen markieren», the
 * entries (picture, what happened with a dot while unread, the title, when). OK on an entry marks it
 * read and opens its title or «Meine Wünsche». The focus starts on an announcement, else on the
 * first unread entry; after «Verstanden» and «Alle als gelesen» it moves on, never into nothing.
 */
@Composable
fun NotificationsScreen(state: NotificationsUiState, images: ServerImages, links: NotificationsLinks, vm: NotificationsViewModel) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    val entries = (state.entries.items as? Loadable.Ready)?.value.orEmpty()
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.safeY, bottom = TvDimens.Browse.DetailTop),
        verticalArrangement = Arrangement.spacedBy(W.ListGap),
    ) {
        item(key = "head") {
            Column {
                Text(stringResource(R.string.notifications_title), style = TvType.h1, modifier = Modifier.semantics { heading() })
                if (state.entries.items is Loadable.Ready || state.entries.items is Loadable.Empty) {
                    Text(
                        if (state.unread > 0) pluralCount(R.plurals.notifications_summary, state.unread) else stringResource(R.string.tv_notifications_allRead),
                        style = TvType.large,
                        color = c.tx2,
                        modifier = Modifier.padding(top = TvDimens.xs),
                    )
                }
            }
        }
        items(state.announcements, key = { "ann-${it.id}" }) { announcement ->
            Announcement(announcement, focus) {
                // «Verstanden»: the card goes, the focus to the next announcement or the list.
                val next = state.announcements.firstOrNull { it.id != announcement.id }?.let { NotificationsKeys.announcement(it.id) }
                    ?: if (state.unread > 0) NotificationsKeys.ALL else entries.firstOrNull()?.let { NotificationsKeys.entry(it.id) } ?: NotificationsKeys.HOME
                focus.focus(next)
                vm.onAcknowledge(announcement)
            }
        }
        when (val items = state.entries.items) {
            is Loadable.Ready -> {
                if (state.unread > 0) {
                    item(key = "all") {
                        Row(Modifier.widthIn(max = W.Notes).fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            TvButton(
                                stringResource(R.string.notifications_markAllRead),
                                onClick = {
                                    // The button goes with the last unread entry: the focus to the list.
                                    items.value.firstOrNull()?.let { focus.focus(NotificationsKeys.entry(it.id)) }
                                    vm.onMarkAllRead()
                                },
                                icon = SlouchcatIcon.Check.vector,
                                kind = TvButtonKind.Ghost,
                                small = true,
                                modifier = Modifier.focusKey(focus, NotificationsKeys.ALL),
                            )
                        }
                    }
                }
                items(items.value, key = { "nt-${it.id}" }) { entry ->
                    Entry(
                        entry,
                        state.isUnread(entry),
                        images,
                        focus,
                        onOpen = { links.open(vm.onOpen(entry)) },
                        modifier = Modifier.onFocusChanged { if (it.hasFocus && items.value.indexOf(entry) >= items.value.size - NEAR_END) vm.onMore() },
                    )
                }
                if (state.entries.more == MoreState.Failed) {
                    item(key = "more") { TvButton(stringResource(R.string.app_state_retry), onClick = vm::onMore, icon = SlouchcatIcon.Refresh.vector, modifier = Modifier.focusKey(focus, NotificationsKeys.MORE)) }
                }
            }
            Loadable.Empty -> item(key = "empty") {
                EmptyState(
                    SlouchcatIcon.Bell.vector,
                    stringResource(R.string.notifications_empty_title),
                    stringResource(R.string.notifications_empty_body),
                    action = links.home?.let { home -> { EmptyAction(stringResource(R.string.app_lists_toStart), SlouchcatIcon.Home.vector, home, Modifier.focusKey(focus, NotificationsKeys.HOME)) } },
                )
            }
            else -> item(key = "state") { LoadableContent(items, onRetry = vm::retry, retryModifier = Modifier.focusKey(focus, NotificationsKeys.RETRY)) {} }
        }
    }
    val start = state.announcements.firstOrNull()?.let { NotificationsKeys.announcement(it.id) }
        ?: (entries.firstOrNull { state.isUnread(it) } ?: entries.firstOrNull())?.let { NotificationsKeys.entry(it.id) }
        ?: when (state.entries.items) {
            Loadable.Empty -> NotificationsKeys.HOME
            is Loadable.Failed -> NotificationsKeys.RETRY
            else -> NotificationsKeys.ALL
        }
    focus.StartFocus(start, state.entries.items::class, state.announcements.size, start)
}

/** `.md-ann`: the frame of the warning colour, the sign, kicker, title, text and «Verstanden». */
@Composable
private fun Announcement(announcement: ProfileAnnouncement, focus: FocusMemory, onAcknowledge: () -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    Row(
        Modifier
            .widthIn(max = W.Notes)
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(TvDimens.borderThick, c.warn), shape)
            .padding(horizontal = W.AnnPadX, vertical = W.AnnPadY),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TvIcon(SlouchcatIcon.Alert, W.AnnIcon, tint = c.warn)
        Column(Modifier.weight(1f).semantics(mergeDescendants = true) {}, verticalArrangement = Arrangement.spacedBy(TvDimens.xs / 2)) {
            Text(stringResource(R.string.announcements_kicker).uppercase(), style = TvType.section.copy(fontSize = TvType.small.fontSize), color = c.tx2)
            Text(announcement.title, style = TvType.largeStrong, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.semantics { heading() })
            Text(announcement.body, style = TvType.body, color = c.tx2, maxLines = BODY_LINES, overflow = TextOverflow.Ellipsis)
        }
        TvButton(
            stringResource(R.string.announcements_acknowledge),
            onClick = onAcknowledge,
            icon = SlouchcatIcon.Check.vector,
            kind = TvButtonKind.Primary,
            small = true,
            modifier = Modifier.focusKey(focus, NotificationsKeys.announcement(announcement.id)).semantics {
                contentDescription = announcement.title
            },
        )
    }
}

/** `.md-nrow`: the picture, what happened (with the dot while unread), the title, when. */
@Composable
private fun Entry(entry: Notification, unread: Boolean, images: ServerImages, focus: FocusMemory, onOpen: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    var focused by remember { mutableStateOf(false) }
    val title = entryTitle(entry)
    val context = entryContext(entry)
    // A clock a little behind the server's never reads «in 0 minutes»: the future counts as now.
    val now = System.currentTimeMillis()
    val time = DateUtils.getRelativeTimeSpanString(minOf(entry.createdAt.toInstant().toEpochMilli(), now), now, DateUtils.MINUTE_IN_MILLIS).toString()
    val said = stringResource(if (unread) R.string.notifications_entryLabelUnread else R.string.notifications_entryLabel, title, context, time)
    val shape = RoundedCornerShape(TvDimens.radiusPanel)
    val picture = entry.parent ?: entry.item
    Surface(
        onClick = onOpen,
        modifier = modifier
            .widthIn(max = W.Notes)
            .fillMaxWidth()
            .focusKey(focus, NotificationsKeys.entry(entry.id))
            .onFocusChanged { focused = it.isFocused }
            .semantics(mergeDescendants = true) { contentDescription = said },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(containerColor = c.s1, contentColor = c.tx, focusedContainerColor = c.tx, focusedContentColor = c.bg, pressedContainerColor = c.tx, pressedContentColor = c.bg),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, if (unread) c.line else androidx.compose.ui.graphics.Color.Transparent), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
    ) {
        Row(Modifier.padding(start = W.NotePad, top = W.NotePad, bottom = W.NotePad, end = W.NotePadEnd), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(W.NoteGap)) {
            Box(Modifier.width(W.NotePoster).aspectRatio(POSTER).clip(RoundedCornerShape(TvDimens.radiusCard)).background(c.s2)) {
                picture?.let { MediaItem.from(it, images).poster.forWidth(W.NotePoster) }?.let { TvImage(it, Modifier.fillMaxSize()) }
            }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TvDimens.xs / 2)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                    if (unread) Box(Modifier.size(W.Dot).clip(CircleShape).background(if (focused) c.bg else c.accent))
                    Text(context.uppercase(), style = TvType.section.copy(fontSize = TvType.small.fontSize), color = if (focused) c.bg else c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                Text(
                    title,
                    style = TvType.large.copy(fontWeight = if (unread) FontWeight.SemiBold else FontWeight.Medium),
                    color = if (focused) c.bg else if (unread) c.tx else c.tx2,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Text(time, style = TvType.small, color = if (focused) c.bg else c.tx3, maxLines = 1)
        }
    }
}

/** «S01E03 · Pilot» for an episode, the title of a podcast episode; a wish's title when the profile does not see it. */
@Composable
private fun entryTitle(entry: Notification): String {
    val item = entry.item ?: return entry.wish?.title.orEmpty()
    val season = item.seasonNumber
    val episode = item.episodeNumber
    val code = if (season != null && episode != null) stringResource(R.string.home_continue_episode, season.toString(), episode.toString()) else null
    return if (code != null) code + stringResource(R.string.format_separator) + item.title else item.title
}

/** What happened: a new episode of a series, or what became of a wish (web `entryContext`). */
@Composable
private fun entryContext(entry: Notification): String {
    if (entry.aboutWish) {
        return stringResource(
            when (entry.kind) {
                Notification.Kind.WISH_AVAILABLE -> R.string.notifications_wish_available
                Notification.Kind.WISH_SEASON_AVAILABLE -> R.string.notifications_wish_season_available
                Notification.Kind.WISH_APPROVED -> R.string.notifications_wish_approved
                Notification.Kind.WISH_DECLINED -> R.string.notifications_wish_declined
                else -> R.string.notifications_wish_failed
            },
        )
    }
    val parent = entry.parent?.title ?: entry.item?.seriesTitle
    return if (parent != null) stringResource(R.string.notifications_newEpisode, parent) else stringResource(R.string.notifications_newEpisodeAlone)
}

/** How many entries before the end of the loaded ones the next page is asked for. */
private const val NEAR_END = 4
private const val BODY_LINES = 3
private const val POSTER = 2f / 3f
