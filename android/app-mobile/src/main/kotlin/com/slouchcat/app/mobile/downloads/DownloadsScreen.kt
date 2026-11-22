package com.slouchcat.app.mobile.downloads

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.RowGroup
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.shell.ShellTopBar
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.ImageSource
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.ListRow
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatCard
import com.slouchcat.core.design.SlouchcatDimens
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.SwitchRow
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.offline.FetchError
import com.slouchcat.core.offline.OfflinePin
import com.slouchcat.core.offline.StoredProfile
import com.slouchcat.core.strings.R

@Composable
fun qualityName(quality: DownloadQuality): String = stringResource(
    when (quality) {
        DownloadQuality.HIGH -> R.string.offline_quality_high
        DownloadQuality.LOW -> R.string.offline_quality_low
        else -> R.string.offline_quality_medium
    },
)

/** What the page can open: the offline player, the detail page (with contact), the start page. */
class DownloadsLinks(
    val play: (DownloadRowUi) -> Unit,
    val item: ((String) -> Unit)?,
    val start: (() -> Unit)?,
)

/**
 * «Downloads» (mockup `/downloads`, reference photo handy 32): the offline banner, the storage of
 * the device, the sections «Wird geladen», «Wartet», «Fertig» with episodes grouped under their
 * series, the empty state, and folded below: the rule of auto-downloads and the setting «nur im
 * WLAN». From 700 dp the storage and the folds stand beside the list (mockup `md-dlgrid`).
 */
@Composable
fun DownloadsScreen(state: DownloadsUiState, events: DownloadsEvents, links: DownloadsLinks, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    Box(modifier.fillMaxSize().background(c.bg)) {
        Column(Modifier.fillMaxSize()) {
            ShellTopBar(stringResource(R.string.nav_downloads))
            Column(
                Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = gutter(), vertical = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Column(Modifier.widthIn(max = if (wide) 1100.dp else 728.dp).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    if (state.offline) OfflineBanner()
                    if (!state.viewerIsSession || (state.offline && state.offlineProfiles.size > 1)) ViewerRow(state, events)
                    if (wide) {
                        Row(horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) { Lists(state, events, links) }
                            Column(Modifier.width(340.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                                state.storage?.let { StorageCard(it) }
                                Folds(state, events)
                            }
                        }
                    } else {
                        state.storage?.let { StorageCard(it) }
                        Lists(state, events, links)
                        Folds(state, events)
                    }
                    Spacer(Modifier.height(24.dp))
                }
            }
        }
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
    when (state.sheet) {
        DownloadsSheet.Delete -> DeleteSheet(state, events)
        DownloadsSheet.Profiles -> ProfilesSheet(state, events)
        DownloadsSheet.Pin -> state.pin?.let { PinSheet(it, events) }
        null -> Unit
    }
}

/** `md-offline`: no contact with the server, the page shows what lies on the device. */
@Composable
private fun OfflineBanner() {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Column(
        Modifier.fillMaxWidth().clip(shape).background(c.s2).border(BorderStroke(1.dp, c.warn), shape).padding(14.dp)
            .semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite },
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
            SlouchcatIconImage(SlouchcatIcon.Wifi, contentDescription = null, size = 22.dp, tint = c.warn)
            Text(stringResource(R.string.app_downloads_offlineBanner), style = SlouchcatType.rowTitle, color = c.tx)
        }
        Text(stringResource(R.string.app_downloads_offlineHint), style = SlouchcatType.caption, color = c.tx2)
    }
}

@Composable
private fun ViewerRow(state: DownloadsUiState, events: DownloadsEvents) {
    RowGroup(
        rows = listOf {
            ListRow(
                stringResource(R.string.app_downloads_viewing, state.profileName.orEmpty()),
                icon = SlouchcatIcon.User,
                subtitle = stringResource(R.string.app_downloads_switchProfile),
                onClick = if (state.offlineProfiles.size > 1) events::onSwitchProfile else null,
            )
        },
    )
}

/** `md-store`: downloads, other data and free space of the device's volume (9.4). */
@Composable
private fun StorageCard(storage: Storage) {
    val c = SlouchcatTheme.colors
    val used = fileSize(storage.downloads)
    val free = fileSize(storage.free)
    val total = fileSize(storage.total)
    val other = (storage.total - storage.free - storage.downloads).coerceAtLeast(0)
    val otherText = fileSize(other)
    val bar = stringResource(R.string.app_downloads_storageBarApps, used, otherText, free)
    SlouchcatCard(Modifier.fillMaxWidth().semantics(mergeDescendants = true) { contentDescription = bar }) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(stringResource(R.string.app_downloads_storageUsed, used), style = SlouchcatType.strong, color = c.tx, modifier = Modifier.weight(1f))
            Text(stringResource(R.string.app_downloads_storageFree, free, total), style = SlouchcatType.caption, color = c.tx2)
        }
        Spacer(Modifier.height(10.dp))
        val totalBytes = storage.total.coerceAtLeast(1).toFloat()
        Row(Modifier.fillMaxWidth().height(10.dp).clip(RoundedCornerShape(5.dp)).background(c.s3)) {
            val a = (storage.downloads / totalBytes).coerceIn(0.006f, 1f)
            val o = (other / totalBytes).coerceIn(0.006f, 1f - a)
            Box(Modifier.weight(a).height(10.dp).background(c.accent))
            Box(Modifier.weight(o).height(10.dp).background(c.tx3))
            if (1f - a - o > 0.001f) Spacer(Modifier.weight(1f - a - o))
        }
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            Legend(c.accent, stringResource(R.string.app_downloads_legendDownloads))
            Legend(c.tx3, stringResource(R.string.app_downloads_legendOtherApps))
            Legend(c.s3, stringResource(R.string.app_downloads_legendFree))
        }
    }
}

@Composable
private fun Legend(color: androidx.compose.ui.graphics.Color, label: String) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        Box(Modifier.size(10.dp).clip(RoundedCornerShape(3.dp)).background(color).border(BorderStroke(1.dp, SlouchcatTheme.colors.ctl), RoundedCornerShape(3.dp)))
        Text(label, style = SlouchcatType.caption, color = SlouchcatTheme.colors.tx2)
    }
}

@Composable
private fun Lists(state: DownloadsUiState, events: DownloadsEvents, links: DownloadsLinks) {
    if (!state.loaded) return
    if (state.rows.isEmpty()) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
            EmptyState(stringResource(R.string.app_downloads_emptyTitle), body = stringResource(R.string.app_downloads_emptyBody), icon = SlouchcatIcon.Download)
            links.start?.let { SlouchcatButton(stringResource(R.string.app_downloads_toStart), onClick = it, leadingIcon = SlouchcatIcon.Home) }
        }
        return
    }
    for ((section, title) in listOf(Section.Active to R.string.app_downloads_sectionActive, Section.Waiting to R.string.app_downloads_sectionWaiting, Section.Done to R.string.app_downloads_sectionDone)) {
        val rows = state.section(section)
        if (rows.isEmpty()) continue
        Row(Modifier.fillMaxWidth().padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(title), style = SlouchcatType.barTitle, color = SlouchcatTheme.colors.tx, modifier = Modifier.weight(1f).semantics { heading() })
            Text(stringResource(R.string.app_downloads_sectionCount, rows.size, fileSize(rows.sumOf { it.size })), style = SlouchcatType.caption, color = SlouchcatTheme.colors.tx2)
        }
        val blocks = rows.groupBy { it.groupId ?: it.id }
        for ((_, block) in blocks) {
            val first = block.first()
            if (first.groupId == null) {
                RowCard { DownloadRow(first, grouped = false, state.offline, events, links) }
            } else {
                RowCard {
                    GroupHead(block, state.offline, events, links)
                    block.sortedWith(compareBy({ it.season ?: 0 }, { it.episode ?: 0 })).forEach { DownloadRow(it, grouped = true, state.offline, events, links) }
                }
            }
        }
    }
}

@Composable
private fun RowCard(content: @Composable () -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Column(Modifier.fillMaxWidth().clip(shape).background(c.s1).border(BorderStroke(1.dp, c.line), shape).padding(vertical = 6.dp)) { content() }
}

@Composable
private fun GroupHead(block: List<DownloadRowUi>, offline: Boolean, events: DownloadsEvents, links: DownloadsLinks) {
    val c = SlouchcatTheme.colors
    val first = block.first()
    val title = first.groupTitle ?: first.title
    val open = links.item?.takeIf { !offline && first.groupId != null }
    Row(Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(start = 12.dp, end = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Row(
            Modifier.weight(1f).then(if (open != null) Modifier.clickable(role = Role.Button) { open(first.groupId!!) } else Modifier).padding(vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Poster(first, wide = false)
            Column(Modifier.weight(1f)) {
                Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(pluralStringResource(R.plurals.app_downloads_episodes, block.size, block.size, fileSize(block.sumOf { it.size })), style = SlouchcatType.caption, color = c.tx2)
            }
        }
        SlouchcatIconButton(
            SlouchcatIcon.Trash,
            pluralStringResource(R.plurals.app_downloads_deleteGroupLabel, block.size, block.size, title),
            onClick = { events.onDeleteAsk(block) },
            tint = c.tx2,
        )
    }
}

/** `md-dl`: poster, title, the line of its state, the progress bar, the control and the delete. */
@Composable
private fun DownloadRow(row: DownloadRowUi, grouped: Boolean, offline: Boolean, events: DownloadsEvents, links: DownloadsLinks) {
    val c = SlouchcatTheme.colors
    val title = if (grouped && row.episode != null) stringResource(R.string.app_downloads_episodeLine, (row.season ?: 1).toString(), row.episode.toString(), row.title) else row.title
    val line = stateLine(row)
    val open: (() -> Unit)? = when {
        row.playable -> ({ links.play(row) })
        !offline && links.item != null -> ({ links.item.invoke(row.itemId) })
        else -> null
    }
    Row(
        Modifier.fillMaxWidth().heightIn(min = 64.dp).padding(start = if (grouped) 28.dp else 12.dp, end = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            Modifier.weight(1f).then(if (open != null) Modifier.clickable(role = Role.Button, onClick = open) else Modifier).padding(vertical = 6.dp)
                .semantics(mergeDescendants = true) { stateDescription = line },
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Poster(row, wide = grouped)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                if (!grouped && row.groupTitle != null) Text(row.groupTitle, style = SlouchcatType.caption, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(line, style = SlouchcatType.caption, color = if (row.state is RowState.Error || row.state == RowState.Failed) c.warn else c.tx2, maxLines = 2, overflow = TextOverflow.Ellipsis)
                val percent = when (val s = row.state) {
                    is RowState.Fetching -> s.percent
                    is RowState.Paused -> s.percent
                    else -> null
                }
                if (percent != null) {
                    Box(Modifier.fillMaxWidth(0.85f).padding(top = 4.dp).height(4.dp).clip(RoundedCornerShape(2.dp)).background(c.s3)) {
                        Box(Modifier.fillMaxWidth(percent.coerceAtLeast(2) / 100f).height(4.dp).background(if (row.state is RowState.Paused) c.tx3 else c.accent))
                    }
                }
            }
        }
        when (row.state) {
            RowState.Done -> SlouchcatIconButton(SlouchcatIcon.Play, stringResource(R.string.app_downloads_play, row.title), onClick = { links.play(row) })
            is RowState.Paused, is RowState.Error -> SlouchcatIconButton(SlouchcatIcon.Download, stringResource(R.string.app_downloads_resumeLabel, row.title), onClick = { events.onResume(row.id) }, enabled = !offline)
            else -> SlouchcatIconButton(SlouchcatIcon.Pause, stringResource(R.string.app_downloads_pauseLabel, row.title), onClick = { events.onPause(row.id) }, enabled = !offline)
        }
        SlouchcatIconButton(SlouchcatIcon.Trash, stringResource(R.string.app_downloads_deleteLabel, row.title), onClick = { events.onDeleteAsk(listOf(row)) }, tint = c.tx2)
    }
}

@Composable
private fun stateLine(row: DownloadRowUi): String {
    val size = fileSize(row.size)
    return when (val s = row.state) {
        is RowState.Fetching -> stringResource(R.string.app_downloads_lineFetching, s.percent.toString(), fileSize(s.bytes), fileSize(s.total.takeIf { it > 0 } ?: row.size))
        is RowState.Paused -> stringResource(R.string.app_downloads_linePaused, s.percent.toString(), size)
        is RowState.Preparing -> stringResource(R.string.app_downloads_linePreparing, s.percent.toString(), size)
        RowState.Queued -> stringResource(R.string.app_downloads_lineQueued, size)
        RowState.BudgetFull -> stringResource(R.string.app_downloads_lineBudget, size)
        RowState.ServerPaused -> stringResource(R.string.app_downloads_lineServerPaused, size)
        RowState.WaitsForWifi -> stringResource(R.string.app_downloads_lineWifi, size)
        RowState.Failed -> stringResource(R.string.app_downloads_lineFailed)
        is RowState.Error -> stringResource(errorText(s.code))
        RowState.Done -> stringResource(R.string.app_downloads_lineDone, qualityName(row.quality), size) +
            if (row.auto) " · " + stringResource(R.string.app_downloads_lineAuto) else ""
    }
}

private fun errorText(code: String): Int = when (code) {
    FetchError.QUOTA -> R.string.offline_state_quota
    FetchError.STORAGE -> R.string.offline_state_storage
    FetchError.RANGE -> R.string.offline_state_range
    FetchError.GONE, FetchError.UNAUTHORIZED -> R.string.offline_state_gone
    FetchError.NETWORK -> R.string.offline_state_network
    else -> R.string.offline_state_evicted
}

/**
 * The poster the fetch stored, through the app's image loader in the size of its box, off the main
 * thread (code review m-6); without one the name on a quiet field, as every card of the app does.
 */
@Composable
private fun Poster(row: DownloadRowUi, wide: Boolean) {
    val width = if (wide) 88.dp else 44.dp
    val height = if (wide) 50.dp else 64.dp
    val images = remember(row.poster) { ImageSet(listOfNotNull(row.poster?.let { ImageSource("file://" + it.absolutePath, POSTER_WIDTH) })) }
    MediaImage(images, row.groupTitle ?: row.title, Modifier.size(width, height).clip(RoundedCornerShape(6.dp)))
}

private const val POSTER_WIDTH = 200

/** The two folds of the mockup: «Automatische Downloads» and «Einstellungen» (closed by default). */
@Composable
private fun Folds(state: DownloadsUiState, events: DownloadsEvents) {
    val rule = state.rule
    Fold(
        icon = SlouchcatIcon.Refresh,
        title = stringResource(R.string.app_downloads_autoTitle),
        subtitle = stringResource(if (state.ruleOn) R.string.app_downloads_autoOn else R.string.app_downloads_autoOff),
        open = state.autoOpen,
        enabled = !state.offline,
        onToggle = events::onToggleAuto,
    ) {
        when {
            rule != null -> RuleFields(rule, state.profileName.orEmpty(), events)
            state.ruleFailed -> Note(stringResource(R.string.app_downloads_autoUnavailable), warning = true)
        }
    }
    Fold(
        icon = SlouchcatIcon.Settings,
        title = stringResource(R.string.app_downloads_settingsTitle),
        subtitle = stringResource(if (state.wifiOnly) R.string.app_downloads_settingsWifi else R.string.app_downloads_settingsAny),
        open = state.settingsOpen,
        enabled = true,
        onToggle = events::onToggleSettings,
    ) {
        SwitchRow(stringResource(R.string.app_downloads_wifiOnly), state.wifiOnly, events::onWifiOnly, subtitle = stringResource(R.string.app_downloads_wifiOnlyHint))
    }
}

@Composable
private fun Fold(icon: SlouchcatIcon, title: String, subtitle: String, open: Boolean, enabled: Boolean, onToggle: () -> Unit, body: @Composable () -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    val state = stringResource(if (open) R.string.app_downloads_expanded else R.string.app_downloads_collapsed)
    Column(Modifier.fillMaxWidth().clip(shape).background(c.s1).border(BorderStroke(1.dp, c.line), shape)) {
        ListRow(
            title,
            icon = icon,
            subtitle = subtitle,
            onClick = if (enabled) onToggle else null,
            end = { SlouchcatIconImage(if (open) SlouchcatIcon.Up else SlouchcatIcon.Down, contentDescription = null, size = 20.dp, tint = c.tx3) },
            modifier = Modifier.semantics { stateDescription = state },
        )
        if (open && enabled) Column(Modifier.padding(start = 4.dp, end = 4.dp, bottom = 12.dp)) { body() }
    }
}

/** The rule of the contract (`DownloadRules`, 9.6): on/off, next episodes, new podcast episodes, Wi-Fi, count, quality. */
@Composable
private fun RuleFields(form: RuleForm, name: String, events: DownloadsEvents) {
    val c = SlouchcatTheme.colors
    val r = form.rules
    SwitchRow(stringResource(R.string.offline_rules_enabled), r.enabled, { on -> events.onRule { it.copy(enabled = on) } })
    SwitchRow(stringResource(R.string.offline_rules_nextEpisodes), r.nextEpisodes, { on -> events.onRule { it.copy(nextEpisodes = on) } }, enabled = r.enabled)
    SwitchRow(stringResource(R.string.offline_rules_newPodcastEpisodes), r.newPodcastEpisodes, { on -> events.onRule { it.copy(newPodcastEpisodes = on) } }, enabled = r.enabled)
    SwitchRow(stringResource(R.string.offline_rules_wifiOnly), r.wifiOnly, { on -> events.onRule { it.copy(wifiOnly = on) } }, enabled = r.enabled)
    Row(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(stringResource(R.string.offline_rules_maxCount), style = SlouchcatType.rowTitle, color = c.tx)
            Text(pluralStringResource(R.plurals.app_downloads_maxCountValue, r.maxCount, r.maxCount), style = SlouchcatType.caption, color = c.tx2)
        }
        SlouchcatIconButton(SlouchcatIcon.Rew, stringResource(R.string.app_downloads_maxCountLess), onClick = { events.onRule { it.copy(maxCount = it.maxCount - 1) } }, enabled = r.enabled && r.maxCount > DownloadsViewModel.MIN_COUNT)
        SlouchcatIconButton(SlouchcatIcon.Plus, stringResource(R.string.app_downloads_maxCountMore), onClick = { events.onRule { it.copy(maxCount = it.maxCount + 1) } }, enabled = r.enabled && r.maxCount < DownloadsViewModel.MAX_COUNT)
    }
    Text(stringResource(R.string.offline_rules_quality), style = SlouchcatType.label, color = c.tx2, modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp))
    Column(Modifier.selectableGroup()) {
        for (q in listOf(DownloadQuality.HIGH, DownloadQuality.MEDIUM, DownloadQuality.LOW)) {
            RadioRow(qualityName(q), null, r.quality == q, enabled = r.enabled) { events.onRule { it.copy(quality = q) } }
        }
    }
    Text(stringResource(R.string.app_downloads_autoHint, name), style = SlouchcatType.caption, color = c.tx3, modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp))
    SlouchcatButton(
        stringResource(R.string.offline_rules_save),
        onClick = events::onSaveRule,
        busy = form.saving,
        block = true,
        modifier = Modifier.padding(horizontal = 12.dp),
    )
    if (form.failed) Note(stringResource(R.string.app_downloads_ruleFailed), warning = true, modifier = Modifier.padding(12.dp))
}

/** `.su-opt`: a radio row (the dialog and the rule use it). */
@Composable
fun RadioRow(label: String, end: String?, selected: Boolean, enabled: Boolean = true, detail: String? = null, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier.fillMaxWidth().heightIn(min = SlouchcatDimens.touch)
            .then(if (selected) Modifier.border(BorderStroke(1.dp, c.tx), RoundedCornerShape(0.dp)) else Modifier)
            .selectable(selected = selected, enabled = enabled, role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = gutter(), vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(22.dp).clip(RoundedCornerShape(11.dp)).border(BorderStroke(2.dp, if (selected) c.accent else c.ctl), RoundedCornerShape(11.dp)), contentAlignment = Alignment.Center) {
            if (selected) Box(Modifier.size(11.dp).clip(RoundedCornerShape(6.dp)).background(c.accent))
        }
        Column(Modifier.weight(1f)) {
            Text(label, style = SlouchcatType.rowTitle, color = if (enabled) c.tx else c.tx3)
            if (detail != null) Text(detail, style = SlouchcatType.caption, color = c.tx2)
        }
        if (end != null) Text(end, style = SlouchcatType.label, color = c.tx)
    }
}

@Composable
private fun DeleteSheet(state: DownloadsUiState, events: DownloadsEvents) {
    val rows = state.deleting
    if (rows.isEmpty()) return
    val title = pluralStringResource(R.plurals.app_downloads_deleteTitle, rows.size)
    SlouchcatSheet(title, onDismiss = events::onDismissSheet) {
        SheetHeadIcon(SlouchcatIcon.Trash, warning = true)
        SheetTitle(title, centered = true)
        val first = rows.first()
        SheetText(
            if (rows.size > 1) {
                pluralStringResource(R.plurals.app_downloads_deleteBodyMany, rows.size, rows.size, first.groupTitle ?: first.title)
            } else {
                stringResource(R.string.app_downloads_deleteBody, first.title)
            },
            centered = true,
        )
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismissSheet, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.app_downloads_deleteConfirm), onClick = events::onDeleteConfirm, kind = ButtonKind.Danger, leadingIcon = SlouchcatIcon.Trash, modifier = Modifier.weight(1f))
        }
    }
}

/** «Wer schaut offline?»: the profiles with downloads on this device (9.5). */
@Composable
private fun ProfilesSheet(state: DownloadsUiState, events: DownloadsEvents) {
    val title = stringResource(R.string.app_downloads_switchTitle)
    SlouchcatSheet(title, onDismiss = events::onDismissSheet) {
        SheetTitle(title)
        Column(Modifier.selectableGroup()) {
            state.offlineProfiles.forEach { p: StoredProfile ->
                RadioRow(p.snapshot.name, null, p.snapshot.name == state.profileName, detail = pinDetail(p)) { events.onPickProfile(p) }
            }
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismissSheet, kind = ButtonKind.Ghost, block = true, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun pinDetail(p: StoredProfile): String? = when (OfflinePin.requirement(p)) {
    OfflinePin.Requirement.OfflinePin -> stringResource(R.string.app_downloads_pinOffline)
    OfflinePin.Requirement.Pin -> stringResource(R.string.app_downloads_pinLabel)
    else -> null
}

/** The PIN of a profile, checked on the device with the brake (9.5); the hint on short PINs (review L-5). */
@Composable
private fun PinSheet(pin: OfflinePinUi, events: DownloadsEvents) {
    val c = SlouchcatTheme.colors
    val title = stringResource(R.string.app_downloads_pinTitle, pin.profile.snapshot.name)
    SlouchcatSheet(title, onDismiss = events::onDismissSheet) {
        SheetHeadIcon(SlouchcatIcon.Lock)
        SheetTitle(title, centered = true)
        if (pin.unavailable) {
            SheetText(stringResource(R.string.app_downloads_pinUnavailable), centered = true)
        } else {
            val label = stringResource(if (pin.offlinePin) R.string.app_downloads_pinOffline else R.string.app_downloads_pinLabel)
            Text(
                "\u25CF".repeat(pin.digits.length).ifEmpty { "\u2013" },
                style = SlouchcatType.pinKey,
                color = c.tx,
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp).semantics { contentDescription = label; stateDescription = pin.digits.length.toString() },
                textAlign = androidx.compose.ui.text.style.TextAlign.Center,
            )
            val message = when {
                pin.lockedSeconds > 0 -> stringResource(R.string.app_downloads_pinLocked, pin.lockedSeconds.toString())
                pin.wrong -> stringResource(R.string.app_downloads_pinWrong)
                else -> null
            }
            if (message != null) Text(message, style = SlouchcatType.caption, color = c.warn, textAlign = androidx.compose.ui.text.style.TextAlign.Center, modifier = Modifier.fillMaxWidth().semantics { liveRegion = LiveRegionMode.Polite })
            PinPad(events, enabled = !pin.checking && pin.lockedSeconds == 0L)
            if (!pin.offlinePin) Text(stringResource(R.string.app_downloads_pinWeakCopy), style = SlouchcatType.caption, color = c.tx3, modifier = Modifier.padding(horizontal = gutter(), vertical = 8.dp))
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismissSheet, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            if (!pin.unavailable) {
                SlouchcatButton(stringResource(R.string.app_downloads_pinOpen), onClick = events::onPinSubmit, busy = pin.checking, enabled = pin.digits.length >= DownloadsViewModel.MIN_PIN && pin.lockedSeconds == 0L, modifier = Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun PinPad(events: DownloadsEvents, enabled: Boolean) {
    val keys = listOf("123", "456", "789")
    Column(Modifier.fillMaxWidth().padding(horizontal = gutter()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        keys.forEach { line ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                line.forEach { d -> SlouchcatButton(d.toString(), onClick = { events.onPinDigit(d) }, kind = ButtonKind.Secondary, enabled = enabled, modifier = Modifier.weight(1f)) }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Spacer(Modifier.weight(1f))
            SlouchcatButton("0", onClick = { events.onPinDigit('0') }, kind = ButtonKind.Secondary, enabled = enabled, modifier = Modifier.weight(1f))
            Box(Modifier.weight(1f), contentAlignment = Alignment.Center) {
                SlouchcatIconButton(SlouchcatIcon.Back, stringResource(R.string.app_downloads_pinDelete), onClick = events::onPinDelete, enabled = enabled)
            }
        }
    }
}
