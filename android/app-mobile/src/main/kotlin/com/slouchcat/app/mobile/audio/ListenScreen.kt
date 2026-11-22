package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.kindName
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.app.mobile.player.timecode
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.player.AudioRules
import com.slouchcat.core.player.LastAudio
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.SleepChoice
import com.slouchcat.core.player.SleepState
import com.slouchcat.core.strings.R
import java.text.NumberFormat

/** What the audio player page can be asked to do; [ListenFeature] passes it to `AudioPlayback`, tests record it. */
interface ListenEvents {
    fun onClose()
    fun onTogglePlay()
    fun onSeekTo(positionMs: Long)
    fun onBack()
    fun onForward()
    fun onPreviousChapter()
    fun onNextChapter()
    fun onPreviousEpisode()
    fun onNextEpisode()
    fun onSpeed(speed: Float)
    fun onSleep(choice: SleepChoice)
    fun onChapter(chapter: Chapter)
    fun onRetry()
}

/** What the page draws: the title that plays, or the one it was opened for before it plays. */
data class ListenUi(
    val state: PlayerState? = null,
    val sleep: SleepState? = null,
    val offline: Boolean = false,
    /** The title of a process that died, shown paused until a tap. */
    val restored: LastAudio? = null,
    val cover: ImageSet = ImageSet(emptyList()),
)

/** The sheets of the audio player (mockup photos handy 60 to 63). */
enum class ListenSheet { Speed, Sleep, Chapters }

/**
 * The audio player (mockup `/listen/:i`, photos handy 37 and 38; web `AudioPlayer`): a page of its
 * own without the shell. Cover, title and author, the chapter, the timeline with the time played
 * and left, the time left of the whole title, the buttons (previous chapter, −15 s, play, +30 s,
 * next chapter; previous and next episode for a podcast episode without chapters), and the round
 * buttons for speed, sleep timer and chapters. From 700 dp the cover stands left of the controls.
 */
@Composable
fun ListenScreen(ui: ListenUi, events: ListenEvents, initialSheet: ListenSheet? = null) {
    val c = SlouchcatTheme.colors
    var sheet by rememberSaveable { mutableStateOf(initialSheet) }
    val wide = isWideLayout()
    Box(
        Modifier
            .fillMaxSize()
            .background(c.bg)
            .safeDrawingPadding(),
    ) {
        Column(Modifier.fillMaxSize()) {
            Header(ui, events)
            if (wide) {
                Row(
                    Modifier.fillMaxSize().padding(horizontal = 40.dp, vertical = 16.dp),
                    horizontalArrangement = Arrangement.spacedBy(48.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Cover(ui, Modifier.weight(1f).widthIn(max = 460.dp))
                    Column(Modifier.weight(1f).verticalScroll(rememberScrollState())) { Controls(ui, events) { sheet = it } }
                }
            } else {
                Column(
                    Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = gutter(), vertical = 8.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Cover(ui, Modifier.fillMaxWidth(0.82f).widthIn(max = 420.dp))
                    Controls(ui, events) { sheet = it }
                }
            }
        }
        val state = ui.state
        when (sheet) {
            ListenSheet.Speed -> SpeedSheet(state?.speed ?: 1f, events) { sheet = null }
            ListenSheet.Sleep -> SleepSheet(ui.sleep, hasChapters = state?.chapters.orEmpty().isNotEmpty(), events) { sheet = null }
            ListenSheet.Chapters -> ChaptersSheet(state?.chapters.orEmpty(), state?.positionMs ?: 0, events) { sheet = null }
            null -> Unit
        }
    }
}

/** «Hörbuch · Kapitel 8 von 20» between the close button and the end (mockup `.lp-top`). */
@Composable
private fun Header(ui: ListenUi, events: ListenEvents) {
    val c = SlouchcatTheme.colors
    val state = ui.state
    val chapters = state?.chapters.orEmpty()
    val index = state?.let { AudioRules.chapterIndex(chapters, it.positionMs) }
    val kind = state?.item?.kind?.let { kindName(it) } ?: stringResource(R.string.kind_audiobook)
    val text = buildList {
        add(kind)
        if (index != null) add(stringResource(R.string.app_audio_chapterOf, (index + 1).toString(), chapters.size.toString()))
        if (ui.offline) add(stringResource(R.string.app_audio_offline))
    }.joinToString(stringResource(R.string.format_separator))
    Row(Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        SlouchcatIconButton(SlouchcatIcon.Down, contentDescription = stringResource(R.string.app_audio_close), onClick = events::onClose)
        Text(
            text,
            style = SlouchcatType.label.copy(fontSize = 14.sp),
            color = c.tx2,
            textAlign = TextAlign.Center,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f).padding(end = 48.dp),
        )
    }
}

@Composable
private fun Cover(ui: ListenUi, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(20.dp)
    val title = ui.state?.item?.title ?: ui.restored?.title.orEmpty()
    Box(modifier.aspectRatio(1f).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
        MediaImage(ui.cover, title, Modifier.fillMaxSize())
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Controls(ui: ListenUi, events: ListenEvents, openSheet: (ListenSheet) -> Unit) {
    val c = SlouchcatTheme.colors
    val state = ui.state
    val item = state?.item
    val title = item?.title ?: ui.restored?.title.orEmpty()
    val byline = when {
        item?.kind == ItemKind.PODCAST_EPISODE -> item.seriesTitle ?: item.author
        item != null -> item.author
        else -> ui.restored?.byline
    }
    val chapters = state?.chapters.orEmpty()
    val position = state?.positionMs ?: 0
    val duration = state?.durationMs ?: 0
    val index = AudioRules.chapterIndex(chapters, position)
    Column(Modifier.fillMaxWidth().padding(top = 20.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(title, style = SlouchcatType.pageTitle.copy(fontSize = 26.sp, lineHeight = 30.sp, fontWeight = FontWeight.ExtraBold), color = c.tx, modifier = Modifier.semantics { heading() })
        byline?.takeIf { it.isNotBlank() }?.let { Text(it, style = SlouchcatType.rowTitle.copy(fontSize = 17.sp), color = c.tx2) }
        if (index != null) {
            val chapter = chapters[index]
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                SlouchcatIconImage(SlouchcatIcon.Book, contentDescription = null, size = 16.dp, tint = c.tx3)
                Text(chapterTitle(chapter, index), style = SlouchcatType.caption.copy(fontSize = 14.sp), color = c.tx3, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
    }
    val error = state?.error
    if (error != null) {
        Column(Modifier.fillMaxWidth().padding(top = 16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Note(errorText(error), warning = true)
            if (error.retryable) SlouchcatButton(stringResource(R.string.app_state_retry), onClick = events::onRetry, kind = ButtonKind.Secondary, block = true)
        }
    }
    Timeline(position, duration, state?.bufferedMs ?: 0, enabled = state != null && !state.loading && error == null, onSeek = events::onSeekTo)
    if (duration > 0) {
        val percent = (position * 100 / duration).toInt()
        Text(
            stringResource(R.string.app_audio_leftTotal, runtimeText(maxOf(1, ((duration - position) / 60_000).toInt())), percent.toString()),
            style = SlouchcatType.caption.copy(fontSize = 13.sp),
            color = c.tx3,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
    }
    Transport(ui, events)
    FlowRow(
        Modifier.fillMaxWidth().padding(top = 18.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.CenterHorizontally),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        val rate = rateText(state?.speed ?: 1f)
        val speedLabel = stringResource(R.string.player_audio_speedButton, rate)
        SlouchcatButton(rate, onClick = { openSheet(ListenSheet.Speed) }, kind = ButtonKind.Ghost, leadingIcon = SlouchcatIcon.Speed, enabled = state != null, modifier = Modifier.semantics { contentDescription = speedLabel })
        val sleep = ui.sleep
        SlouchcatButton(
            if (sleep != null) stringResource(R.string.app_audio_sleepLeft, timecode(sleep.leftMs)) else stringResource(R.string.app_audio_sleepTitle),
            onClick = { openSheet(ListenSheet.Sleep) },
            kind = if (sleep != null) ButtonKind.Secondary else ButtonKind.Ghost,
            leadingIcon = SlouchcatIcon.Moon,
            enabled = state != null,
        )
        if (chapters.isNotEmpty()) {
            SlouchcatButton(stringResource(R.string.player_chapters_button), onClick = { openSheet(ListenSheet.Chapters) }, kind = ButtonKind.Ghost, leadingIcon = SlouchcatIcon.List)
        }
    }
}

/** The bar with the time played and the time left (`.lp-bar`); TalkBack hears «27:46 von 1:06:25». */
@Composable
private fun Timeline(position: Long, duration: Long, buffered: Long, enabled: Boolean, onSeek: (Long) -> Unit) {
    val c = SlouchcatTheme.colors
    var dragging by remember { mutableStateOf<Float?>(null) }
    val shown = dragging?.toLong() ?: position
    val label = stringResource(R.string.player_seek)
    val time = stringResource(R.string.player_time, timecode(shown), timecode(duration))
    Column(Modifier.fillMaxWidth().padding(top = 14.dp)) {
        Slider(
            value = shown.toFloat().coerceIn(0f, duration.coerceAtLeast(1).toFloat()),
            onValueChange = { dragging = it },
            onValueChangeFinished = {
                dragging?.let { onSeek(it.toLong()) }
                dragging = null
            },
            valueRange = 0f..duration.coerceAtLeast(1).toFloat(),
            enabled = enabled && duration > 0,
            colors = SliderDefaults.colors(thumbColor = c.tx, activeTrackColor = c.accent, inactiveTrackColor = c.s3),
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp).semantics {
                contentDescription = label
                stateDescription = time
            },
        )
        Row(Modifier.fillMaxWidth()) {
            Text(timecode(shown), style = SlouchcatType.caption, color = c.tx2, modifier = Modifier.weight(1f))
            Text(stringResource(R.string.app_player_remaining, timecode(duration - shown)), style = SlouchcatType.caption, color = c.tx2)
        }
    }
}

/** Previous, −15 s, play or pause, +30 s, next (`.lp-ctl`). */
@Composable
private fun Transport(ui: ListenUi, events: ListenEvents) {
    val c = SlouchcatTheme.colors
    val state = ui.state
    val chapters = state?.chapters.orEmpty()
    val position = state?.positionMs ?: 0
    val ready = state != null && !state.loading && state.error == null
    Row(
        Modifier.fillMaxWidth().padding(top = 14.dp),
        horizontalArrangement = Arrangement.SpaceEvenly,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (chapters.isNotEmpty()) {
            SlouchcatIconButton(SlouchcatIcon.Prev, stringResource(R.string.player_audio_previousChapter), events::onPreviousChapter, enabled = ready)
        } else if (state?.previous != null) {
            SlouchcatIconButton(SlouchcatIcon.Prev, stringResource(R.string.app_audio_previousEpisode), events::onPreviousEpisode, enabled = ready)
        }
        SlouchcatIconButton(SlouchcatIcon.Rew, stringResource(R.string.player_audio_back15), events::onBack, enabled = ready)
        val playing = state?.let { it.playing || (it.buffering && !it.ended) } == true
        val label = stringResource(if (playing) R.string.player_pause else R.string.player_play)
        Box(
            Modifier
                .size(76.dp)
                .clip(CircleShape)
                .background(c.tx)
                .clickable(role = Role.Button, onClickLabel = label, onClick = events::onTogglePlay)
                .semantics { contentDescription = label },
            contentAlignment = Alignment.Center,
        ) {
            SlouchcatIconImage(if (playing) SlouchcatIcon.Pause else SlouchcatIcon.Play, contentDescription = null, size = 34.dp, tint = c.bg)
        }
        SlouchcatIconButton(SlouchcatIcon.Fwd, stringResource(R.string.player_audio_forward30), events::onForward, enabled = ready)
        if (chapters.isNotEmpty()) {
            SlouchcatIconButton(SlouchcatIcon.Next, stringResource(R.string.player_audio_nextChapter), events::onNextChapter, enabled = ready && AudioRules.nextChapterStart(chapters, position) != null)
        } else if (state?.next != null) {
            SlouchcatIconButton(SlouchcatIcon.Next, stringResource(R.string.app_audio_nextEpisode), events::onNextEpisode, enabled = ready)
        }
    }
}

/** «Kapitel 3» for a chapter without a title (web `player.chapters.untitled`). */
@Composable
fun chapterTitle(chapter: Chapter, index: Int): String =
    chapter.title?.takeIf { it.isNotBlank() } ?: stringResource(R.string.player_chapters_untitled, (index + 1).toString())

/** «1,0×» in the format of the language (web `player.audio.rate`). */
@Composable
fun rateText(speed: Float): String {
    val format = NumberFormat.getNumberInstance(LocalConfiguration.current.locales[0]).apply {
        minimumFractionDigits = 1
        maximumFractionDigits = 2
    }
    return stringResource(R.string.player_audio_rate, format.format(speed.toDouble()))
}

private val PlayerError.retryable: Boolean
    get() = this is PlayerError.Failed || this is PlayerError.Network || this is PlayerError.Busy || this is PlayerError.StoppedElsewhere

@Composable
private fun errorText(error: PlayerError): String = when (error) {
    PlayerError.NotFound -> stringResource(R.string.app_player_error_notFoundBody)
    PlayerError.WatchLimit -> stringResource(R.string.player_watchLimit_title)
    PlayerError.StreamLimit -> stringResource(R.string.player_streamLimit_title)
    PlayerError.StoppedElsewhere -> stringResource(R.string.app_player_error_stoppedBody)
    PlayerError.SignedOut -> stringResource(R.string.player_ended_title)
    is PlayerError.Network -> stringResource(if (error.retrying) R.string.app_player_error_networkRetrying else R.string.app_player_error_networkBody)
    is PlayerError.Busy -> stringResource(R.string.player_busy_title)
    PlayerError.Unplayable -> stringResource(R.string.player_error_title)
    PlayerError.Failed -> stringResource(R.string.player_error_title)
}

/** The count of the chapters with the length («20 · 16 Std. 4 Min.»), for the detail page. */
@Composable
fun chaptersMeta(count: Int, durationMs: Long?): String {
    val length = durationMs?.let { runtimeText(maxOf(1, (it / 60_000).toInt())) }
    return if (length != null) stringResource(R.string.app_audio_chaptersMeta, count.toString(), length) else count.toString()
}

/** «noch 3 Minuten» of the sleep sheet («15 Minuten»). */
@Composable
fun sleepMinutes(minutes: Int): String = pluralStringResource(R.plurals.player_audio_sleep_minutes, minutes, minutes)
