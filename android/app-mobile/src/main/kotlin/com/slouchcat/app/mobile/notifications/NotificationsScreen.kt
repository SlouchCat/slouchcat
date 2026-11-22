package com.slouchcat.app.mobile.notifications

import android.text.format.DateUtils
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.item.episodeCode
import com.slouchcat.app.mobile.media.MediaEmpty
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.pluralCount
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.browse.notifications.NotificationsUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.aboutWish
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

/** What «Benachrichtigungen» does on a tap. */
interface NotificationsEvents {
    fun onBack()
    fun onRetry()
    fun onMore()
    fun onOpen(entry: Notification)
    fun onMarkAllRead()
    fun onAcknowledge(announcement: ProfileAnnouncement)

    /** «Einstellungen für Benachrichtigungen»; `null` while the page is not registered. */
    val onSettings: (() -> Unit)?
}

/**
 * «Benachrichtigungen» (mockup `/notifications`, reference photo handy 43): the open announcements
 * of the main user on top with «Verstanden», the count of the unread with «Alle als gelesen
 * markieren», the entries as one card (picture, what happened, the title, when; a dot while
 * unread), and the way to the settings. An entry leads to its title or to «Meine Wünsche».
 */
@Composable
fun NotificationsScreen(state: NotificationsUiState, images: ServerImages?, events: NotificationsEvents, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val scroll = rememberScrollState()
    val entries = state.entries
    if (entries.hasMore) {
        val nearEnd by remember(scroll) { derivedStateOf { scroll.maxValue > 0 && scroll.value >= scroll.maxValue - NEAR_END_PX } }
        LaunchedEffect(scroll, entries.loaded.size) { snapshotFlow { nearEnd }.distinctUntilChanged().filter { it }.collect { events.onMore() } }
    }
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            TopBar(stringResource(R.string.notifications_title), navigationLabel = stringResource(R.string.common_back), onNavigation = events::onBack)
            Column(
                Modifier
                    .fillMaxSize()
                    .verticalScroll(scroll)
                    .padding(horizontal = gutter())
                    .then(if (isWideLayout()) Modifier.widthIn(max = 760.dp) else Modifier),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Spacer(Modifier.height(4.dp))
                state.announcements.forEach { announcement ->
                    AnnouncementCard(announcement, busy = announcement.id in state.acknowledging) { events.onAcknowledge(announcement) }
                }
                when (val items = entries.items) {
                    is Loadable.Ready -> {
                        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            Text(pluralCount(R.plurals.notifications_summary, state.unread), style = SlouchcatType.bodySmall, color = c.tx2, modifier = Modifier.weight(1f))
                            if (state.unread > 0) {
                                SlouchcatButton(
                                    stringResource(R.string.notifications_markAllRead),
                                    onClick = events::onMarkAllRead,
                                    kind = ButtonKind.Secondary,
                                    small = true,
                                    busy = state.markingAll,
                                    leadingIcon = SlouchcatIcon.Check,
                                )
                            }
                        }
                        Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(c.s1).border(1.dp, c.line, RoundedCornerShape(14.dp))) {
                            items.value.forEachIndexed { index, entry ->
                                if (index > 0) HorizontalDivider(color = c.line)
                                Entry(entry, state.isUnread(entry), images) { events.onOpen(entry) }
                            }
                        }
                        if (entries.more != MoreState.Idle) PagingFooter(entries.more, events::onMore)
                    }
                    Loadable.Empty -> MediaEmpty(stringResource(R.string.notifications_empty_title), body = stringResource(R.string.notifications_empty_body), icon = SlouchcatIcon.Bell)
                    else -> LoadableContent(items, onRetry = events::onRetry) {}
                }
                events.onSettings?.let { open ->
                    Box(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(c.s1).border(1.dp, c.line, RoundedCornerShape(14.dp))) {
                        ListRow(stringResource(R.string.app_notifications_settings), icon = SlouchcatIcon.Settings, subtitle = stringResource(R.string.app_notifications_settingsHint), onClick = open)
                    }
                }
                Spacer(Modifier.height(24.dp))
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

/** Pixels before the end of the list at which the next page is asked for. */
private const val NEAR_END_PX = 600

/** The mockup's announcement (`.ann`): kicker, title, text, «Verstanden». */
@Composable
private fun AnnouncementCard(announcement: ProfileAnnouncement, busy: Boolean, onAcknowledge: () -> Unit) {
    val c = SlouchcatTheme.colors
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .background(c.s2)
            .border(1.dp, c.info, RoundedCornerShape(14.dp))
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            SlouchcatIconImage(SlouchcatIcon.Info, contentDescription = null, size = 16.dp, tint = c.info)
            Text(stringResource(R.string.announcements_kicker).uppercase(), style = SlouchcatType.section, color = c.info)
        }
        Text(announcement.title, style = SlouchcatType.strong, color = c.tx, modifier = Modifier.semantics { heading() })
        Text(announcement.body, style = SlouchcatType.bodySmall, color = c.tx2)
        Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.CenterEnd) {
            SlouchcatButton(stringResource(R.string.announcements_acknowledge), onClick = onAcknowledge, kind = ButtonKind.Secondary, small = true, busy = busy)
        }
    }
}

@Composable
private fun Entry(entry: Notification, unread: Boolean, images: ServerImages?, onOpen: () -> Unit) {
    val c = SlouchcatTheme.colors
    val title = entryTitle(entry)
    val context = entryContext(entry)
    val time = DateUtils.getRelativeTimeSpanString(entry.createdAt.toInstant().toEpochMilli(), System.currentTimeMillis(), DateUtils.MINUTE_IN_MILLIS).toString()
    val label = if (unread) {
        stringResource(R.string.notifications_entryLabelUnread, title, context, time)
    } else {
        stringResource(R.string.notifications_entryLabel, title, context, time)
    }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 72.dp)
            .clickable(onClick = onOpen)
            .clearAndSetSemantics {
                contentDescription = label
                role = Role.Button
            }
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        val picture = entry.parent ?: entry.item
        MediaImage(
            if (images != null && picture != null) images.posters(picture.images) else ImageSet.NONE,
            title,
            Modifier.width(44.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp)),
        )
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(context, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(title, style = SlouchcatType.bodySmall, color = c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(time, style = SlouchcatType.caption, color = c.tx3)
        }
        if (unread) Box(Modifier.size(10.dp).clip(CircleShape).background(c.accent))
    }
}

/** «S01E03 · Pilot» for an episode, the title of a podcast episode; a wish's title when the profile does not see it. */
@Composable
private fun entryTitle(entry: Notification): String {
    val item = entry.item ?: return entry.wish?.title.orEmpty()
    val code = episodeCode(item.seasonNumber, item.episodeNumber, null)
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
