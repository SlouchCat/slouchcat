package com.slouchcat.app.tv.audio

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.IconAction
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.ProgressBar
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.kindName
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.player.timecode
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.browse.item.ItemEvents
import com.slouchcat.core.browse.item.ItemUiState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.PlayTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.audioPlayTarget
import com.slouchcat.core.media.listable
import com.slouchcat.core.media.minutesLeft
import com.slouchcat.core.media.share
import com.slouchcat.core.media.started
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R

/** Where an audio detail page leads; `null`: that page is not registered (no control). */
data class AudioItemLinks(
    /** The audio player for a title. */
    val play: ((itemId: String) -> Unit)?,
    /** The audio player from a chapter's start. */
    val playAt: ((itemId: String, startMs: Long) -> Unit)?,
    /** Another detail page (the podcast of an episode). */
    val item: ((String) -> Unit)?,
    /** The series of an audiobook. */
    val series: ((libraryId: Long, seriesId: String) -> Unit)?,
)

/** The episodes of a podcast and what a row can change ([com.slouchcat.core.browse.audio.PodcastEpisodesViewModel]). */
data class EpisodesUi(
    val episodes: Paged<ItemRef>,
    val busy: Set<String>,
    val onNearEnd: () -> Unit,
    val onRetry: () -> Unit,
    val onHeard: (ItemRef) -> Unit,
)

/** Test tags and focus keys of the audio detail pages. */
object AudioItemKeys {
    const val PLAY = "audio-play"
    const val WATCHLIST = "audio-watchlist"
    const val SERIES = "audio-series"
    const val PODCAST = "audio-podcast"
    const val RETRY = "audio-retry"
    fun chapter(index: Int) = "chapter-$index"
    fun episode(id: String) = "episode-$id"
    fun heard(id: String) = "heard-$id"
}

/**
 * The detail page of an audiobook, a podcast or an episode (mockup `/book/:i`, `/podcast/:i`,
 * photos TV 25 to 28): two zones. On the left the square cover, the progress («38 % · noch 9 Std.
 * 58 Min.»), the big button (Weiterhören, Anhören, «Neueste Folge»), «Merken», the series or the
 * podcast. On the right the kind, the title, the author, the facts and the list: the chapters (a
 * check mark for the heard ones, OK plays from the chapter's start) or the episodes, newest first
 * (a dot for new ones, a check for heard ones, the progress of a begun one; OK plays it, the check
 * at the end of a row marks it heard). An episode shows its summary.
 */
@Composable
fun AudioItemScreen(state: ItemUiState, images: ServerImages, links: AudioItemLinks, events: ItemEvents, episodes: EpisodesUi?) {
    val focus = rememberFocusMemory()
    val detail = state.detail
    if (detail == null) {
        LoadableContent(
            state.item,
            onRetry = events::retry,
            modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop),
            retryModifier = Modifier.focusKey(focus, AudioItemKeys.RETRY).testTag(AudioItemKeys.RETRY),
        ) {}
        focus.StartFocus(AudioItemKeys.RETRY, state.item::class)
        return
    }
    val target = audioPlayTarget(detail)
    Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.spacedBy(A.DetailGap)) {
        Side(detail, target, images, links, events, focus)
        val list = rememberLazyListState()
        LazyColumn(
            state = list,
            modifier = Modifier.weight(1f).fillMaxHeight(),
            contentPadding = PaddingValues(top = TvDimens.safeY, end = TvDimens.safeX, bottom = TvDimens.safeY),
        ) {
            item(key = "head") { Head(detail) }
            when (detail.kind) {
                ItemKind.AUDIOBOOK -> chapters(detail, links, focus)
                ItemKind.PODCAST -> if (episodes != null) episodes(episodes, links, focus)
                else -> item(key = "overview") {
                    detail.overview?.takeIf { it.isNotBlank() }?.let { Text(it, style = TvType.large, color = SlouchcatTheme.colors.tx2, modifier = Modifier.padding(top = A.ListTitleTop)) }
                }
            }
        }
    }
    focus.StartFocus(if (target != null && links.play != null) AudioItemKeys.PLAY else AudioItemKeys.WATCHLIST, detail.id)
}

/** The left zone: cover, progress, the buttons. */
@Composable
private fun Side(detail: ItemDetail, target: PlayTarget?, images: ServerImages, links: AudioItemLinks, events: ItemEvents, focus: FocusMemory) {
    val c = SlouchcatTheme.colors
    Column(Modifier.padding(start = LocalContentStart.current, top = TvDimens.safeY).width(A.DetailCover), verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        Cover(images.posters(detail.images), A.DetailCover)
        val resume = detail.progress.started().takeIf { detail.kind == ItemKind.AUDIOBOOK || detail.kind == ItemKind.PODCAST_EPISODE }
        if (resume != null) {
            Column(verticalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                ProgressBar(share(resume), A.DetailCover)
                Text(
                    stringResource(R.string.tv_audio_progressLeft, (share(resume) * PERCENT).toInt().toString(), runtimeText(minutesLeft(resume))),
                    style = TvType.body,
                    color = c.tx2,
                )
            }
        }
        val play = links.play
        if (play != null && target != null) {
            TvButton(playLabel(target), onClick = { play(target.id) }, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, AudioItemKeys.PLAY).testTag(AudioItemKeys.PLAY))
        }
        if (listable(detail.kind)) {
            TvButton(
                stringResource(if (detail.inWatchlist) R.string.item_watchlist_added else R.string.item_watchlist_add),
                onClick = events::onWatchlist,
                icon = if (detail.inWatchlist) SlouchcatIcon.BookmarkOn.vector else SlouchcatIcon.Bookmark.vector,
                modifier = Modifier.focusKey(focus, AudioItemKeys.WATCHLIST).testTag(AudioItemKeys.WATCHLIST),
            )
        }
        val series = detail.seriesId
        val library = detail.libraryId
        val name = detail.seriesName
        val openSeries = links.series
        if (series != null && library != null && name != null && openSeries != null) {
            val label = detail.seriesIndex?.let { stringResource(R.string.item_audio_volume, name, it.stripTrailingZeros().toPlainString()) } ?: name
            TvButton(label, onClick = { openSeries(library, series) }, icon = SlouchcatIcon.Book.vector, kind = TvButtonKind.Ghost, small = true, modifier = Modifier.focusKey(focus, AudioItemKeys.SERIES).testTag(AudioItemKeys.SERIES))
        }
        val podcast = detail.podcast
        val open = links.item
        if (detail.kind == ItemKind.PODCAST_EPISODE && podcast != null && open != null) {
            TvButton(stringResource(R.string.item_podcast_link), onClick = { open(podcast.id) }, icon = SlouchcatIcon.Mic.vector, kind = TvButtonKind.Ghost, small = true, modifier = Modifier.focusKey(focus, AudioItemKeys.PODCAST).testTag(AudioItemKeys.PODCAST))
        }
    }
}

/** A square cover with its ground while it loads. */
@Composable
fun Cover(images: ImageSet, size: Dp, modifier: Modifier = Modifier) {
    val image = images.forWidth(size)
    Box(modifier.size(size).clip(RoundedCornerShape(TvDimens.radiusDialog)).background(SlouchcatTheme.colors.s2)) {
        if (image != null) TvImage(image, Modifier.fillMaxSize())
    }
}

/** The right zone's head: kind with its icon, title, author, facts. */
@Composable
private fun Head(detail: ItemDetail) {
    val c = SlouchcatTheme.colors
    Column {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
            TvIcon(if (detail.kind == ItemKind.AUDIOBOOK) SlouchcatIcon.Headphones else SlouchcatIcon.Mic, TvDimens.Browse.Icon, tint = c.accent)
            kindName(detail.kind)?.let { KindLine(it) }
        }
        Text(detail.title, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).semantics { heading() })
        val byline = when (detail.kind) {
            ItemKind.PODCAST_EPISODE -> detail.podcast?.title ?: detail.seriesTitle
            else -> detail.author
        }
        byline?.takeIf { it.isNotBlank() }?.let { Text(it, style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.xs)) }
        val facts = facts(detail)
        if (facts.isNotEmpty()) Text(facts.joinToString(stringResource(R.string.format_separator)), style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s))
        detail.narrator?.takeIf { it.isNotBlank() && detail.kind == ItemKind.AUDIOBOOK }?.let {
            Text(stringResource(R.string.item_audio_narratedBy, it), style = TvType.body, color = c.tx3, modifier = Modifier.padding(top = TvDimens.xs))
        }
    }
}

/** «2013 · 16 Std. 4 Min. · 16 Kapitel», «8 Folgen», «17. November 2026 · 32 Min.». */
@Composable
private fun facts(detail: ItemDetail): List<String> = buildList {
    when (detail.kind) {
        ItemKind.AUDIOBOOK -> {
            detail.year?.let { add(it.toString()) }
            (detail.durationMs?.let { (it / MINUTE_MS).toInt() } ?: detail.runtimeMinutes)?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
            detail.chapters?.size?.takeIf { it > 0 }?.let { add(chaptersText(it)) }
        }
        ItemKind.PODCAST -> (detail.episodeCount ?: detail.childCount)?.takeIf { it > 0 }?.let { add(pluralCount(R.plurals.app_item_episodes, it)) }
        else -> {
            detail.publishedAt?.let { add(dateText(it)) }
            (detail.durationMs?.let { (it / MINUTE_MS).toInt() } ?: detail.runtimeMinutes)?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
        }
    }
}

/** The words of the big button (web `playTarget` for sound). */
@Composable
private fun playLabel(target: PlayTarget): String = when (target.kind) {
    PlayTarget.Kind.Resume, PlayTarget.Kind.EpisodeResume -> stringResource(R.string.item_action_resumeListening)
    PlayTarget.Kind.Again -> stringResource(R.string.item_action_againListening)
    PlayTarget.Kind.Episode -> stringResource(R.string.app_audio_newest)
    PlayTarget.Kind.Play -> stringResource(R.string.tv_audio_listen)
}

/** «Kapitel»: number or check, title, «gehört», length; OK plays from the chapter's start. */
private fun LazyListScope.chapters(detail: ItemDetail, links: AudioItemLinks, focus: FocusMemory) {
    val chapters = detail.chapters.orEmpty()
    if (chapters.isEmpty()) return
    item(key = "chapters-title") { ListTitle(stringResource(R.string.item_audio_chapters)) }
    val position = if (detail.progress?.completed == true) Long.MAX_VALUE else detail.progress?.positionMs ?: 0
    val playable = detail.mediaFiles.any { it.available }
    itemsIndexed(chapters, key = { index, _ -> "chapter-$index" }) { index, chapter ->
        ChapterRow(detail.id, chapter, index, position, links.playAt?.takeIf { playable }, focus)
    }
}

@Composable
private fun ChapterRow(itemId: String, chapter: Chapter, index: Int, position: Long, playAt: ((String, Long) -> Unit)?, focus: FocusMemory) {
    val title = chapterTitle(chapter, index)
    val heard = position >= chapter.endMs
    AudioRow(
        title,
        onClick = { playAt?.invoke(itemId, chapter.startMs) },
        subtitle = if (heard) stringResource(R.string.tv_audio_heard) else null,
        end = timecode(chapter.endMs - chapter.startMs),
        strong = !heard,
        label = stringResource(R.string.item_audio_chapterPlay, title, timecode(chapter.startMs)),
        modifier = Modifier.focusKey(focus, AudioItemKeys.chapter(index)).testTag(AudioItemKeys.chapter(index)),
    ) {
        if (heard) HeardMark() else NumberMark(index + 1)
    }
}

/** «Folgen», newest first, in pages. */
private fun LazyListScope.episodes(ui: EpisodesUi, links: AudioItemLinks, focus: FocusMemory) {
    item(key = "episodes-title") { ListTitle(stringResource(R.string.item_episodes_title)) }
    when (val items = ui.episodes.items) {
        is Loadable.Ready -> {
            val list = items.value
            itemsIndexed(list, key = { _, episode -> episode.id }) { index, episode ->
                EpisodeRow(episode, episode.id in ui.busy, links, ui.onHeard, focus)
                if (index == list.lastIndex && ui.episodes.hasMore) LaunchedEffect(list.size) { ui.onNearEnd() }
            }
        }
        Loadable.Empty -> item(key = "episodes-empty") { Text(stringResource(R.string.item_podcast_empty), style = TvType.large, color = SlouchcatTheme.colors.tx2) }
        else -> item(key = "episodes-state") {
            LoadableContent(items, onRetry = ui.onRetry, retryModifier = Modifier.focusKey(focus, AudioItemKeys.RETRY).testTag(AudioItemKeys.RETRY)) {}
        }
    }
}

@Composable
private fun EpisodeRow(episode: ItemRef, busy: Boolean, links: AudioItemLinks, onHeard: (ItemRef) -> Unit, focus: FocusMemory) {
    val heard = episode.watchedMark == WatchedMark.Full
    val begun = episode.progress.started()
    val state = when {
        heard -> stringResource(R.string.tv_audio_heard)
        begun != null -> stringResource(R.string.app_item_left, runtimeText(minutesLeft(begun)))
        else -> stringResource(R.string.tv_audio_new)
    }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
        AudioRow(
            episode.title,
            onClick = { links.play?.invoke(episode.id) },
            subtitle = listOfNotNull(episode.publishedAt?.let { dateText(it) }, state).joinToString(stringResource(R.string.format_separator)),
            end = episodeLength(episode),
            strong = !heard,
            progress = begun?.let(::share),
            label = stringResource(R.string.item_podcast_play, episode.title),
            modifier = Modifier.weight(1f).focusKey(focus, AudioItemKeys.episode(episode.id)).testTag(AudioItemKeys.episode(episode.id)),
        ) {
            if (heard) HeardMark() else NewMark()
        }
        IconAction(
            SlouchcatIcon.Check.vector,
            stringResource(if (heard) R.string.app_audio_episodeUnheard else R.string.app_audio_episodeHeard, episode.title),
            onClick = { onHeard(episode) },
            pressed = heard,
            enabled = !busy,
            modifier = Modifier.focusKey(focus, AudioItemKeys.heard(episode.id)).testTag(AudioItemKeys.heard(episode.id)),
        )
    }
}

@Composable
private fun ListTitle(text: String) =
    Text(text, style = TvType.h2, modifier = Modifier.fillMaxWidth().padding(top = A.ListTitleTop, bottom = TvDimens.m).semantics { heading() })

private const val PERCENT = 100
private const val MINUTE_MS = 60_000L
