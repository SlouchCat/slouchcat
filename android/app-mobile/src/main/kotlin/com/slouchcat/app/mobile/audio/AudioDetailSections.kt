package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.selection.toggleable
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.common.LoadableContent
import com.slouchcat.app.mobile.item.Section
import com.slouchcat.app.mobile.media.PagingFooter
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.app.mobile.player.timecode
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** Where the audio sections of a detail page lead; `null` while the page behind it is not there. */
data class AudioDetailLinks(
    /** Plays [itemId] at a position (a chapter's start). */
    val playAt: ((itemId: String, startMs: Long) -> Unit)?,
    val play: ((itemId: String) -> Unit)?,
    val item: (String) -> (() -> Unit)?,
    val series: ((libraryId: Long, seriesId: String) -> Unit)?,
)

/** The episodes of a podcast and what a row can change. */
data class EpisodesUi(
    val episodes: Paged<ItemRef>,
    val busy: Set<String>,
    val onNearEnd: () -> Unit,
    val onRetry: () -> Unit,
    val onHeard: (ItemRef) -> Unit,
)

/**
 * The audio parts of a detail page (mockup `/book/:i`, `/podcast/:i`; web `AudiobookChapters`,
 * `PodcastEpisodes`): the series of an audiobook («Harbour Tales · Band 2»), its chapters with their
 * length and a check mark for the heard ones, each playing from its start; the episodes of a
 * podcast, newest first, with play, date and length, and the check mark «gehört».
 */
@Composable
fun AudioDetailSections(item: ItemDetail, links: AudioDetailLinks, episodes: EpisodesUi?) {
    when (item.kind) {
        ItemKind.AUDIOBOOK -> {
            SeriesLink(item, links)
            Chapters(item, links)
        }
        ItemKind.PODCAST -> episodes?.let { Episodes(it, links) }
        else -> Unit
    }
}

@Composable
private fun SeriesLink(item: ItemDetail, links: AudioDetailLinks) {
    val series = item.seriesId ?: return
    val library = item.libraryId ?: return
    val name = item.seriesName ?: return
    val open = links.series ?: return
    val label = item.seriesIndex?.let { stringResource(R.string.item_audio_volume, name, it.stripTrailingZeros().toPlainString()) } ?: name
    SlouchcatButton(label, onClick = { open(library, series) }, kind = ButtonKind.Ghost, small = true, leadingIcon = SlouchcatIcon.Book, modifier = Modifier.padding(top = 12.dp))
}

@Composable
private fun Chapters(item: ItemDetail, links: AudioDetailLinks) {
    val chapters = item.chapters.orEmpty()
    if (chapters.isEmpty()) return
    val c = SlouchcatTheme.colors
    val playable = item.mediaFiles.any { it.available }
    val position = if (item.progress?.completed == true) Long.MAX_VALUE else item.progress?.positionMs ?: 0
    val shape = RoundedCornerShape(16.dp)
    Section(stringResource(R.string.item_audio_chapters), end = chaptersMeta(chapters.size, item.durationMs)) {
        Column(Modifier.fillMaxWidth().padding(top = 8.dp).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
            chapters.forEachIndexed { index, chapter ->
                if (index > 0) HorizontalDivider(color = c.line)
                val title = chapterTitle(chapter, index)
                val heard = position >= chapter.endMs
                val play = links.playAt?.takeIf { playable }
                val label = stringResource(R.string.item_audio_chapterPlay, title, timecode(chapter.startMs))
                Row(
                    Modifier
                        .fillMaxWidth()
                        .heightIn(min = 56.dp)
                        .then(if (play != null) Modifier.clickable(role = Role.Button, onClickLabel = label) { play(item.id, chapter.startMs) } else Modifier)
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    Box(Modifier.size(36.dp).clip(RoundedCornerShape(10.dp)).background(c.s2), contentAlignment = Alignment.Center) {
                        if (heard) {
                            SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 18.dp, tint = c.ok)
                        } else {
                            Text((index + 1).toString(), style = SlouchcatType.label, color = c.tx3)
                        }
                    }
                    Column(Modifier.weight(1f)) {
                        Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        if (heard) Text(stringResource(R.string.watched_heard_button), style = SlouchcatType.caption, color = c.tx3)
                    }
                    Text(timecode(chapter.endMs - chapter.startMs), style = SlouchcatType.caption, color = c.tx2)
                }
            }
        }
    }
}

@Composable
private fun Episodes(ui: EpisodesUi, links: AudioDetailLinks) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(16.dp)
    Section(stringResource(R.string.item_podcast_episodes), end = stringResource(R.string.app_audio_newestFirst)) {
        LoadableContent(ui.episodes.items, onRetry = ui.onRetry, empty = { Text(stringResource(R.string.item_podcast_empty), style = SlouchcatType.body, color = c.tx3) }) { list ->
            Column(Modifier.fillMaxWidth().padding(top = 8.dp).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
                list.forEachIndexed { index, episode ->
                    if (index > 0) HorizontalDivider(color = c.line)
                    EpisodeRow(episode, episode.id in ui.busy, links, ui.onHeard)
                }
            }
            if (ui.episodes.hasMore) LaunchedEffect(list.size) { ui.onNearEnd() }
            PagingFooter(ui.episodes.more, onRetry = ui.onNearEnd)
        }
    }
}

@Composable
private fun EpisodeRow(episode: ItemRef, busy: Boolean, links: AudioDetailLinks, onHeard: (ItemRef) -> Unit) {
    val c = SlouchcatTheme.colors
    val heard = episode.watchedMark == WatchedMark.Full
    val locale = LocalConfiguration.current.locales[0]
    val date = episode.publishedAt?.let { DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withLocale(locale).format(it) }
    val length = (episode.durationMs?.let { (it / 60_000).toInt() } ?: episode.runtimeMinutes)?.let { runtimeText(maxOf(1, it)) }
    val meta = listOfNotNull(date, length).joinToString(stringResource(R.string.format_separator))
    val open = links.item(episode.id)
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        links.play?.let { play ->
            Box(Modifier.clip(CircleShape).border(BorderStroke(1.dp, c.frame), CircleShape)) {
                SlouchcatIconButton(SlouchcatIcon.Play, stringResource(R.string.item_podcast_play, episode.title), onClick = { play(episode.id) })
            }
        }
        Column(
            Modifier
                .weight(1f)
                .then(if (open != null) Modifier.clickable(role = Role.Button, onClick = open) else Modifier)
                .padding(vertical = 4.dp),
        ) {
            Text(episode.title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (meta.isNotEmpty()) Text(meta, style = SlouchcatType.caption, color = c.tx3)
        }
        val label = stringResource(if (heard) R.string.app_audio_episodeUnheard else R.string.app_audio_episodeHeard, episode.title)
        Box(
            Modifier
                .size(48.dp)
                .toggleable(value = heard, enabled = !busy, role = Role.Checkbox, onValueChange = { onHeard(episode) })
                .semantics { contentDescription = label },
            contentAlignment = Alignment.Center,
        ) {
            SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 22.dp, tint = if (heard) c.ok else c.tx3)
        }
    }
}
