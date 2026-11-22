package com.slouchcat.app.tv.audio

import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.kindName
import com.slouchcat.app.tv.player.timecode
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.NoteTone
import com.slouchcat.app.tv.ui.OnRemoteKey
import com.slouchcat.app.tv.ui.RemoteKey
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvRemoteKeys
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioRules
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay

/** What the audio player can be asked to do; [PlaybackListenEvents] passes it on, tests record it. */
interface ListenEvents {
    fun onTogglePlay()
    fun onBack()
    fun onForward()

    /** The chapter before (or its start), else the episode before. */
    fun onPrevious()

    /** The next chapter, else the next episode. */
    fun onNext()
    fun onSpeed(speed: Float)
    fun onChapter(chapter: Chapter)
    fun onEpisode(episode: ItemRef)
    fun onRetry()

    /** The Stop key of the remote: the title ends, the page closes. */
    fun onStop()

    /** Back: the page closes, the sound plays on («Läuft gerade»). */
    fun onLeave()
}

/** [ListenEvents] on the app's [AudioPlayback]. */
class PlaybackListenEvents(private val playback: AudioPlayback, private val source: AudioSource, private val leave: () -> Boolean) : ListenEvents {
    private val chapters get() = playback.title?.state?.value?.chapters.orEmpty()

    override fun onTogglePlay() = playback.togglePlay()

    override fun onBack() {
        playback.title?.seekBy(-AudioRules.BACK_MS)
    }

    override fun onForward() {
        playback.title?.seekBy(AudioRules.FORWARD_MS)
    }

    override fun onPrevious() {
        if (chapters.isNotEmpty()) playback.previousChapter() else playback.title?.playPrevious()
    }

    override fun onNext() {
        if (chapters.isNotEmpty()) playback.nextChapter() else playback.title?.playNext()
    }

    override fun onSpeed(speed: Float) {
        playback.title?.setSpeed(AudioRules.clampSpeed(speed))
    }

    override fun onChapter(chapter: Chapter) {
        playback.title?.seekTo(chapter.startMs)
    }

    override fun onEpisode(episode: ItemRef) = playback.playTitle(episode.id, source)

    override fun onRetry() {
        playback.title?.retry()
    }

    override fun onStop() {
        playback.stop()
        leave()
    }

    override fun onLeave() {
        leave()
    }
}

/** Test tags of the audio player. */
object ListenKeys {
    const val PLAY = "listen-play"
    const val SEEK = "listen-seek"
    const val BACK = "listen-back"
    const val FORWARD = "listen-forward"
    const val PREVIOUS = "listen-previous"
    const val NEXT = "listen-next"
    const val SPEED = "listen-speed"
    const val LIST = "listen-list"
    const val RETRY = "listen-retry"
    const val CALM = "listen-calm"
    const val MAIN = "listen-main"
}

/** The side menus of the audio player (mockup photos TV 58 and 59). */
private enum class ListenMenu { Speed, List }

/** The speeds of the menu «Tempo» (mockup `mdSpeedMenu`). */
private val SPEEDS = listOf(0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f)

/**
 * The audio player of the TV (concept 26 5.6; mockup `/listen/:i`, photos TV 29 and 30): a page
 * without the rail. The cover on the left; on the right the kind with the chapter («Hörbuch ·
 * Kapitel 7 von 16») or the date of an episode, the title, the author, the chapter or the episode,
 * the timeline of the chapter (or episode) with the time played and left, the buttons (chapter or
 * episode before, −15 s, play or pause, +30 s, next) and the chips «Tempo» and «Kapitel» or
 * «Folgen». Left and right on the timeline jump back 15 s and ahead 30 s; the media keys of the
 * remote do the same, Next and Previous change the chapter. After [calmAfterMs] without a key the
 * screen turns calm (a smaller picture that drifts slowly against burn-in); the TV's screen saver
 * may come after it (the page does not keep the screen on). Back leaves the page, the sound plays on.
 */
@Composable
fun ListenScreen(
    state: PlayerState?,
    images: ServerImages?,
    episodes: Paged<ItemRef>?,
    remoteKeys: TvRemoteKeys,
    events: ListenEvents,
    calmAfterMs: Long = CALM_MS,
) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    var menu by rememberSaveable { mutableStateOf<ListenMenu?>(null) }
    val calm = rememberCalm(calmAfterMs, hold = menu != null)
    val item = state?.item
    OnRemoteKey(remoteKeys) { key ->
        calm.wake()
        when (key) {
            RemoteKey.PlayPause, RemoteKey.Play, RemoteKey.Pause -> events.onTogglePlay()
            RemoteKey.Next -> events.onNext()
            RemoteKey.Previous -> events.onPrevious()
            RemoteKey.FastForward -> events.onForward()
            RemoteKey.Rewind -> events.onBack()
            RemoteKey.Stop -> events.onStop()
        }
    }
    BackHandler(enabled = menu == null) {
        if (!calm.wake()) events.onLeave()
    }
    val chapters = state?.chapters.orEmpty()
    val position = state?.positionMs ?: 0
    val index = AudioRules.chapterIndex(chapters, position)
    val accent = remember(item?.accent) { accentOf(item?.accent) ?: c.s2 }
    val cover = images?.posters(item?.images)?.forWidth(A.Cover)
    val calmShare by animateFloatAsState(if (calm.calm) 1f else 0f, tween(CALM_FADE_MS))
    Box(
        Modifier
            .fillMaxSize()
            .background(c.bg)
            .background(Brush.linearGradient(listOf(accent.copy(alpha = BACKDROP_ALPHA), c.bg)))
            .wakes(calm),
    ) {
        if (state == null || item == null) {
            val error = state?.error
            if (error != null) {
                ListenError(error, focus, events, Modifier.align(Alignment.Center))
            } else {
                TvBusy(stringResource(R.string.app_state_loading), Modifier.align(Alignment.Center))
            }
        } else {
            Row(
                Modifier
                    .fillMaxSize()
                    .alpha(1f - calmShare)
                    .padding(horizontal = TvDimens.safeX, vertical = TvDimens.safeY)
                    .testTag(ListenKeys.MAIN),
                horizontalArrangement = Arrangement.spacedBy(A.Gap),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(A.Cover).clip(RoundedCornerShape(TvDimens.radiusDialog)).background(c.s2)) {
                    if (cover != null) TvImage(cover, Modifier.fillMaxSize())
                }
                Column(Modifier.widthIn(max = A.Info)) {
                    Text(kindLine(item, chapters, index).uppercase(), style = TvType.section, color = c.tx2, maxLines = 1)
                    Text(
                        titleOf(item),
                        style = TvType.xl,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(top = TvDimens.s).semantics { heading() },
                    )
                    if (item.kind != ItemKind.PODCAST_EPISODE) {
                        item.author?.takeIf { it.isNotBlank() }?.let { Text(it, style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.xs)) }
                    }
                    lineOf(item, chapters, index)?.let { line ->
                        Row(Modifier.padding(top = A.TitleGap), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                            TvIcon(if (item.kind == ItemKind.PODCAST_EPISODE) SlouchcatIcon.Mic else SlouchcatIcon.Book, TvDimens.Browse.Icon, tint = c.accent)
                            Text(line, style = TvType.body, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                    val error = state.error
                    if (error != null) {
                        TvNote(errorText(error), tone = NoteTone.Warn, modifier = Modifier.padding(top = TvDimens.l))
                        if (com.slouchcat.app.tv.player.retryable(error)) {
                            TvButton(stringResource(R.string.common_retry), onClick = events::onRetry, icon = SlouchcatIcon.Refresh.vector, modifier = Modifier.padding(top = TvDimens.m).focusKey(focus, ListenKeys.RETRY).testTag(ListenKeys.RETRY))
                        }
                    }
                    val (played, length) = span(state, chapters, index)
                    Seek(played, length, ready(state), events, Modifier.padding(top = A.SeekTop).focusKey(focus, ListenKeys.SEEK).testTag(ListenKeys.SEEK))
                    Row(Modifier.fillMaxWidth().padding(top = TvDimens.xs)) {
                        Text(timecode(played), style = TvType.body, color = c.tx2, modifier = Modifier.weight(1f))
                        Text(remainingText(length - played), style = TvType.body, color = c.tx2)
                    }
                    Transport(state, chapters, focus, events)
                    Row(Modifier.padding(top = A.ChipsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                        TvButton(
                            stringResource(R.string.app_audio_speedTitle),
                            onClick = { menu = ListenMenu.Speed },
                            icon = SlouchcatIcon.Speed.vector,
                            trailing = rateText(state.speed),
                            small = true,
                            modifier = Modifier.focusKey(focus, ListenKeys.SPEED).testTag(ListenKeys.SPEED),
                        )
                        val list = when {
                            chapters.isNotEmpty() -> R.string.player_chapters_button
                            episodes != null -> R.string.item_episodes_title
                            else -> null
                        }
                        if (list != null) {
                            TvButton(stringResource(list), onClick = { menu = ListenMenu.List }, icon = SlouchcatIcon.List.vector, small = true, modifier = Modifier.focusKey(focus, ListenKeys.LIST).testTag(ListenKeys.LIST))
                        }
                    }
                }
            }
            if (calmShare > 0f) {
                val (played, length) = span(state, chapters, index)
                CalmScreen(
                    cover = cover,
                    paused = !state.playing && !state.buffering,
                    title = titleOf(item),
                    line = lineOf(item, chapters, index),
                    share = played.toFloat() / length,
                    modifier = Modifier.alpha(calmShare).testTag(ListenKeys.CALM),
                )
            }
        }
    }
    focus.StartFocus(if (item == null) ListenKeys.RETRY else ListenKeys.PLAY, item?.id, state?.error != null)
    val close = {
        menu = null
        focus.restore()
    }
    when (menu) {
        ListenMenu.Speed -> SpeedMenu(state?.speed ?: 1f, onPick = { events.onSpeed(it); close() }, onDismiss = close)
        ListenMenu.List -> if (chapters.isNotEmpty()) {
            ChaptersMenu(chapters, position, onPick = { events.onChapter(it); close() }, onDismiss = close)
        } else if (episodes != null) {
            EpisodesMenu(episodes, item?.id, onPick = { events.onEpisode(it); close() }, onDismiss = close)
        }
        null -> Unit
    }
}

/** «Hörbuch · Kapitel 7 von 16», «Podcast · 17. November 2026». */
@Composable
private fun kindLine(item: ItemRef, chapters: List<Chapter>, index: Int?): String {
    val parts = buildList {
        if (item.kind == ItemKind.PODCAST_EPISODE) {
            add(stringResource(R.string.kind_podcast))
            item.publishedAt?.let { add(dateText(it)) }
        } else {
            kindName(item.kind)?.let(::add)
            if (index != null) add(stringResource(R.string.app_audio_chapterOf, (index + 1).toString(), chapters.size.toString()))
        }
    }
    return parts.joinToString(stringResource(R.string.format_separator))
}

/** The big title: the book, or the podcast of an episode. */
private fun titleOf(item: ItemRef): String = if (item.kind == ItemKind.PODCAST_EPISODE) item.seriesTitle ?: item.title else item.title

/** The line under the title: the chapter, or the episode. */
@Composable
private fun lineOf(item: ItemRef, chapters: List<Chapter>, index: Int?): String? = when {
    item.kind == ItemKind.PODCAST_EPISODE -> item.title
    index != null -> chapterTitle(chapters[index], index)
    else -> null
}

/** Played and length of the chapter that plays (mockup `rel`, `len`), else of the whole title. */
private fun span(state: PlayerState, chapters: List<Chapter>, index: Int?): Pair<Long, Long> {
    val chapter = index?.let { chapters[it] }
    return if (chapter != null) {
        (state.positionMs - chapter.startMs).coerceAtLeast(0) to (chapter.endMs - chapter.startMs).coerceAtLeast(1)
    } else {
        state.positionMs to state.durationMs.coerceAtLeast(1)
    }
}

private fun ready(state: PlayerState): Boolean = !state.loading && state.error == null

/**
 * `.md-seek`: the timeline, a control of its own. Left jumps back 15 s, right ahead 30 s; a screen
 * reader hears «14:31 von 41:56».
 */
@Composable
private fun Seek(played: Long, length: Long, enabled: Boolean, events: ListenEvents, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    var focused by remember { mutableStateOf(false) }
    val label = stringResource(R.string.player_seek)
    val time = stringResource(R.string.player_time, timecode(played), timecode(length))
    val track by animateDpAsState(if (focused) A.TrackFocus else A.Track, tween(TvMotion.FOCUS_MS))
    Box(
        modifier
            .fillMaxWidth()
            .height(A.TrackFocus * 2)
            .onFocusChanged { focused = it.isFocused }
            .onKeyEvent { event ->
                if (event.type != KeyEventType.KeyDown || !enabled) return@onKeyEvent false
                when (event.key) {
                    Key.DirectionLeft -> events.onBack()
                    Key.DirectionRight -> events.onForward()
                    else -> return@onKeyEvent false
                }
                true
            }
            .focusable()
            .semantics {
                contentDescription = label
                stateDescription = time
            },
        contentAlignment = Alignment.CenterStart,
    ) {
        val share = (played.toFloat() / length.coerceAtLeast(1)).coerceIn(0f, 1f)
        Box(Modifier.fillMaxWidth().height(track).clip(CircleShape).background(SlouchcatTokens.Tv.Color.GlassStrong)) {
            Box(Modifier.fillMaxWidth(share).height(track).background(c.accent))
        }
        if (focused) {
            Row(Modifier.fillMaxWidth()) {
                Box(Modifier.weight(share.coerceAtLeast(MIN_WEIGHT)))
                Box(Modifier.size(A.TrackFocus * 2).clip(CircleShape).background(c.tx))
                Box(Modifier.weight((1f - share).coerceAtLeast(MIN_WEIGHT)))
            }
        }
    }
}

/** `.md-ctl`: before, −15 s, play or pause, +30 s, next. */
@Composable
private fun Transport(state: PlayerState, chapters: List<Chapter>, focus: FocusMemory, events: ListenEvents) {
    val ready = ready(state)
    val byChapter = chapters.isNotEmpty()
    Row(Modifier.padding(top = A.ButtonsTop), horizontalArrangement = Arrangement.spacedBy(A.ButtonGap), verticalAlignment = Alignment.CenterVertically) {
        RoundButton(
            SlouchcatIcon.Prev.vector,
            stringResource(if (byChapter) R.string.player_audio_previousChapter else R.string.app_audio_previousEpisode),
            onClick = events::onPrevious,
            enabled = ready && (byChapter || state.previous != null),
            modifier = Modifier.focusKey(focus, ListenKeys.PREVIOUS).testTag(ListenKeys.PREVIOUS),
        )
        RoundButton(SlouchcatIcon.Rew.vector, stringResource(R.string.player_audio_back15), onClick = events::onBack, enabled = ready, number = (AudioRules.BACK_MS / SECOND_MS).toString(), modifier = Modifier.focusKey(focus, ListenKeys.BACK).testTag(ListenKeys.BACK))
        val playing = state.playing || (state.buffering && !state.ended)
        RoundButton(
            if (playing) SlouchcatIcon.Pause.vector else SlouchcatIcon.Play.vector,
            stringResource(if (playing) R.string.player_pause else R.string.player_play),
            onClick = events::onTogglePlay,
            main = true,
            enabled = state.error == null,
            modifier = Modifier.focusKey(focus, ListenKeys.PLAY).testTag(ListenKeys.PLAY),
        )
        RoundButton(SlouchcatIcon.Fwd.vector, stringResource(R.string.player_audio_forward30), onClick = events::onForward, enabled = ready, number = (AudioRules.FORWARD_MS / SECOND_MS).toString(), modifier = Modifier.focusKey(focus, ListenKeys.FORWARD).testTag(ListenKeys.FORWARD))
        RoundButton(
            SlouchcatIcon.Next.vector,
            stringResource(if (byChapter) R.string.player_audio_nextChapter else R.string.app_audio_nextEpisode),
            onClick = events::onNext,
            enabled = ready && (if (byChapter) AudioRules.nextChapterStart(chapters, state.positionMs) != null else state.next != null),
            modifier = Modifier.focusKey(focus, ListenKeys.NEXT).testTag(ListenKeys.NEXT),
        )
    }
}

/** `.md-cb`: a round button; [number] under the icon («15», «30»); [main] the big light one. */
@Composable
private fun RoundButton(icon: ImageVector, label: String, onClick: () -> Unit, modifier: Modifier, enabled: Boolean = true, main: Boolean = false, number: String? = null) {
    val c = SlouchcatTheme.colors
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .size(if (main) A.ButtonMain else A.Button)
            .alpha(if (enabled) 1f else TvMotion.disabled)
            .semantics { contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = if (main) c.tx else SlouchcatTokens.Tv.Color.Glass,
            contentColor = if (main) c.bg else c.tx,
            focusedContainerColor = if (main) c.accent else c.tx,
            focusedContentColor = if (main) c.onAccent else c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = SlouchcatTokens.Tv.Color.Glass,
            disabledContentColor = c.tx3,
        ),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = CircleShape)),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Column(Modifier.align(Alignment.Center), horizontalAlignment = Alignment.CenterHorizontally) {
            TvIcon(icon, if (main) A.ButtonMainIcon else A.ButtonIcon)
            if (number != null) Text(number, style = TvType.small.copy(fontWeight = androidx.compose.ui.text.font.FontWeight.Bold), modifier = Modifier.clearAndSetSemantics { })
        }
    }
}

/** Whether a page of sound is calm (mockup `MDL.calm`); [wake] on every key. */
@Stable
class CalmControl {
    var calm by mutableStateOf(false)
        internal set
    internal var touched by mutableIntStateOf(0)

    /** The release of the key that woke the page (OK clicks on release): it is taken as well. */
    internal var swallowRelease = false

    /** A key: the page wakes. `true` when it was calm (the key does nothing else). */
    fun wake(): Boolean {
        val was = calm
        calm = false
        touched++
        return was
    }
}

/** Calm after [after] without a key, not while [hold] (a menu is open). */
@Composable
fun rememberCalm(after: Long, hold: Boolean): CalmControl {
    val control = remember { CalmControl() }
    LaunchedEffect(control.touched, hold, after) {
        if (hold) return@LaunchedEffect
        delay(after)
        control.calm = true
    }
    return control
}

/** Every key wakes the calm page; the first one on a calm page does nothing else. */
fun Modifier.wakes(control: CalmControl): Modifier = onPreviewKeyEvent { event ->
    when (event.type) {
        KeyEventType.KeyDown -> control.wake().also { woke -> if (woke) control.swallowRelease = true }
        KeyEventType.KeyUp -> control.swallowRelease.also { control.swallowRelease = false }
        else -> false
    }
}

/** `.md-calm`: the calm screen; it drifts slowly (burn-in) until a key wakes the page. [share]: the bar, `null` for none. */
@Composable
fun CalmScreen(cover: Any?, paused: Boolean, title: String, line: String?, share: Float?, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    var step by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(DRIFT_MS)
            step++
        }
    }
    val x by animateDpAsState(if (step % 2 == 0) -A.CalmDrift else A.CalmDrift, tween(DRIFT_MS.toInt()))
    val y by animateDpAsState(if (step % 4 < 2) -A.CalmDrift / 2 else A.CalmDrift / 2, tween(DRIFT_MS.toInt()))
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Row(Modifier.offset(x, y), horizontalArrangement = Arrangement.spacedBy(A.CalmGap), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(A.CalmCover).clip(RoundedCornerShape(TvDimens.radiusDialog)).background(c.s2)) {
                if (cover != null) TvImage(cover, Modifier.fillMaxSize())
            }
            Column(Modifier.width(A.CalmText), verticalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                    TvIcon(AudioIcons.Bars, TvDimens.Browse.Icon, tint = c.accent)
                    Text(stringResource(if (paused) R.string.radio_page_paused else R.string.tv_audio_nowPlaying).uppercase(), style = TvType.section, color = c.tx2)
                }
                Text(title, style = TvType.h2, maxLines = 2, overflow = TextOverflow.Ellipsis)
                line?.let { Text(it, style = TvType.body, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                if (share != null) {
                    Box(Modifier.width(A.CalmBar).height(TvDimens.Card.Progress).clip(CircleShape).background(SlouchcatTokens.Tv.Color.GlassStrong)) {
                        Box(Modifier.fillMaxWidth(share.coerceIn(0f, 1f)).height(TvDimens.Card.Progress).background(c.accent))
                    }
                }
            }
        }
    }
}

/** «Tempo» (photo TV 58): the speeds, the current one checked and focused. */
@Composable
private fun SpeedMenu(current: Float, onPick: (Float) -> Unit, onDismiss: () -> Unit) {
    val chosen = SPEEDS.minBy { kotlin.math.abs(it - current) }
    val initial = remember { FocusRequester() }
    TvSideMenu(stringResource(R.string.app_audio_speedTitle), onDismiss = onDismiss, initialFocus = initial) {
        Column(verticalArrangement = Arrangement.spacedBy(TvDimens.Menu.ListGap)) {
            for (speed in SPEEDS) {
                TvMenuItem(
                    rateText(speed),
                    onClick = { onPick(speed) },
                    subtitle = if (speed == 1f) stringResource(R.string.tv_audio_normal) else null,
                    chevron = false,
                    selected = speed == chosen,
                    modifier = if (speed == chosen) Modifier.focusRequester(initial) else Modifier,
                )
            }
        }
    }
}

/** «Kapitel» (photo TV 59): number, title, length, heard or the time left; the one that plays is focused. */
@Composable
private fun ChaptersMenu(chapters: List<Chapter>, position: Long, onPick: (Chapter) -> Unit, onDismiss: () -> Unit) {
    val current = AudioRules.chapterIndex(chapters, position)
    val initial = remember { FocusRequester() }
    val heard = stringResource(R.string.tv_audio_heard)
    TvSideMenu(stringResource(R.string.player_chapters_button), onDismiss = onDismiss, initialFocus = initial) {
        chapters.forEachIndexed { index, chapter ->
            val length = timecode(chapter.endMs - chapter.startMs)
            val state = when {
                index == current -> stringResource(R.string.app_item_left, com.slouchcat.app.tv.browse.runtimeText(maxOf(1, ((chapter.endMs - position) / MINUTE_MS).toInt())))
                position >= chapter.endMs -> heard
                else -> null
            }
            AudioRow(
                chapterTitle(chapter, index),
                onClick = { onPick(chapter) },
                subtitle = listOfNotNull(length, state).joinToString(stringResource(R.string.format_separator)),
                strong = true,
                modifier = if (index == (current ?: 0)) Modifier.focusRequester(initial) else Modifier,
            ) {
                if (index == current) TvIcon(AudioIcons.Bars, TvDimens.Browse.Icon, tint = SlouchcatTheme.colors.accent) else NumberMark(index + 1)
            }
        }
    }
}

/** «Folgen» of a podcast: newest first, the one that plays focused. */
@Composable
private fun EpisodesMenu(episodes: Paged<ItemRef>, current: String?, onPick: (ItemRef) -> Unit, onDismiss: () -> Unit) {
    val initial = remember { FocusRequester() }
    val list = (episodes.items as? Loadable.Ready)?.value.orEmpty()
    val focused = list.indexOfFirst { it.id == current }.coerceAtLeast(0)
    TvSideMenu(stringResource(R.string.item_episodes_title), onDismiss = onDismiss, initialFocus = initial.takeIf { list.isNotEmpty() }) {
        list.forEachIndexed { index, episode ->
            val heard = episode.watchedMark == WatchedMark.Full
            AudioRow(
                episode.title,
                onClick = { onPick(episode) },
                subtitle = listOfNotNull(episode.publishedAt?.let { dateText(it) }, episodeLength(episode)).joinToString(stringResource(R.string.format_separator)),
                modifier = if (index == focused) Modifier.focusRequester(initial) else Modifier,
            ) {
                when {
                    episode.id == current -> TvIcon(AudioIcons.Bars, TvDimens.Browse.Icon, tint = SlouchcatTheme.colors.accent)
                    heard -> HeardMark()
                    else -> NewMark()
                }
            }
        }
    }
}

/** A title that could not start: what happened, «Erneut versuchen» where it helps, «Zurück». */
@Composable
private fun ListenError(error: PlayerError, focus: FocusMemory, events: ListenEvents, modifier: Modifier) {
    Column(modifier.widthIn(max = TvDimens.Dialog.Width), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        TvNote(errorText(error), tone = NoteTone.Warn)
        Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            if (com.slouchcat.app.tv.player.retryable(error)) {
                TvButton(stringResource(R.string.common_retry), onClick = events::onRetry, icon = SlouchcatIcon.Refresh.vector, modifier = Modifier.focusKey(focus, ListenKeys.RETRY).testTag(ListenKeys.RETRY))
            }
            TvButton(stringResource(R.string.common_back), onClick = events::onLeave, icon = SlouchcatIcon.Back.vector, modifier = Modifier.focusKey(focus, "leave"))
        }
    }
}

@Composable
private fun errorText(error: PlayerError): String = stringResource(
    when (error) {
        PlayerError.NotFound -> R.string.app_player_error_notFoundBody
        PlayerError.WatchLimit -> R.string.player_watchLimit_title
        PlayerError.StreamLimit -> R.string.player_streamLimit_title
        PlayerError.StoppedElsewhere -> R.string.app_player_error_stoppedBody
        PlayerError.SignedOut -> R.string.player_ended_title
        is PlayerError.Network -> if (error.retrying) R.string.app_player_error_networkRetrying else R.string.app_player_error_networkBody
        is PlayerError.Busy -> R.string.player_busy_title
        PlayerError.Unplayable -> R.string.app_player_error_unplayableBody
        PlayerError.Failed -> R.string.player_error_title
    },
)

/** The title's colour from its cover (`ItemRef.accent`, `#rrggbb`); `null` without one. */
private fun accentOf(hex: String?): Color? = hex?.let { runCatching { Color(android.graphics.Color.parseColor(it)) }.getOrNull() }

/** No key for this long and the page turns calm (mockup: 12 s). */
const val CALM_MS = 12_000L
private const val CALM_FADE_MS = 1_200
private const val DRIFT_MS = 20_000L
private const val BACKDROP_ALPHA = 0.32f
private const val MIN_WEIGHT = 0.0001f
private const val SECOND_MS = 1_000L
private const val MINUTE_MS = 60_000L
