package com.slouchcat.app.mobile.player

import android.content.Context
import android.view.accessibility.AccessibilityManager
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.setProgress
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.TrickplayTile
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay
import kotlin.math.roundToInt

/** What the player page can do; the feature binds it to the controller and the navigation. */
interface PlayerEvents {
    fun onBack()
    fun onTogglePlay()
    fun onSeekTo(positionMs: Long)
    fun onSeekBy(deltaMs: Long)
    fun onSpeed(speed: Float)
    fun onAudio(track: Track)
    fun onSubtitle(track: Track?)
    fun onQuality(quality: Quality)
    fun onSkipIntro()
    fun onSkipOutro()
    fun onPrevious()
    fun onNext()
    fun onEpisode(id: String)
    fun onCancelCountdown()
    fun onRetry()
    fun onEpisodesOpened()
}

/** The sheets of the player (mockup `plMenu`). */
enum class PlayerSheet { Audio, Subtitles, Quality, Speed, Episodes, Info }

/** The speeds of the sheet (mockup `SPEEDS`). */
val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)

/** The controls hide after this long while the video plays (mockup `hideAt`). */
const val CONTROLS_HIDE_MS = 4_000L

private const val JUMP_SECONDS = 10
private const val JUMP_MS = JUMP_SECONDS * 1000L

/** Whether TalkBack (or another screen reader) explores by touch: then the controls never hide. */
@Composable
private fun touchExploration(): Boolean {
    val context = LocalContext.current
    return remember(context) {
        (context.getSystemService(Context.ACCESSIBILITY_SERVICE) as? AccessibilityManager)?.isTouchExplorationEnabled == true
    }
}

/**
 * The video player (mockup `/play/:i`, reference photos handy 29 to 31 and 52 to 59): the picture
 * over the whole screen, a tap shows or hides the controls, a double tap on the left or right
 * half jumps 10 s. Controls: back, title and episode, info; 10 s back, play/pause, 10 s forward;
 * the seek bar with times, chapter marks and the preview tile; the tools Ton, Untertitel,
 * Qualität, Geschwindigkeit and for an episode Folgen, previous and next. «Vorspann überspringen»
 * and the card of the next episode sit bottom right. Errors cover the picture with their own text.
 *
 * [surface] draws the video (the feature puts Media3's view there; tests leave it empty),
 * [thumbnail] one trickplay tile, [images] the stills of the episode list. With a screen reader
 * ([screenReader], TalkBack's touch exploration) the controls stay; without, the picture itself is
 * a control «Bedienung einblenden».
 */
@Composable
fun PlayerScreen(
    state: PlayerState,
    events: PlayerEvents,
    modifier: Modifier = Modifier,
    images: ServerImages? = null,
    surface: @Composable () -> Unit = {},
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit = { _, _ -> },
    initialSheet: PlayerSheet? = null,
    initialControls: Boolean = true,
    screenReader: Boolean = touchExploration(),
) {
    val c = SlouchcatTheme.colors
    var controls by rememberSaveable { mutableStateOf(initialControls) }
    var sheet by rememberSaveable { mutableStateOf(initialSheet) }
    var wake by remember { mutableIntStateOf(0) }
    var dragging by remember { mutableStateOf<Float?>(null) }
    var tap by remember { mutableStateOf<Pair<Boolean, Int>?>(null) }
    val idle = !screenReader && state.playing && sheet == null && dragging == null && state.error == null && state.countdown == null
    LaunchedEffect(controls, wake, idle) {
        if (controls && idle) {
            delay(CONTROLS_HIDE_MS)
            controls = false
        }
    }
    LaunchedEffect(tap) {
        if (tap != null) {
            delay(700)
            tap = null
        }
    }
    val poke: () -> Unit = { wake++ }
    val showLabel = stringResource(R.string.app_player_showControls)

    Box(modifier.fillMaxSize().background(c.bg)) {
        surface()
        // Taps on the picture: show or hide the controls; a double tap jumps (left back, right forward).
        BoxWithConstraints(Modifier.fillMaxSize()) {
            val half = with(LocalDensity.current) { maxWidth.toPx() / 2 }
            val hidden = !(controls || !state.playing)
            Box(
                Modifier
                    .fillMaxSize()
                    // While the controls are hidden the picture is the one control: a screen reader
                    // finds it and brings them back.
                    .semantics {
                        if (hidden && state.error == null) {
                            contentDescription = showLabel
                            onClick(label = showLabel) {
                                controls = true
                                poke()
                                true
                            }
                        }
                    }
                    .pointerInput(half) {
                        detectTapGestures(
                            onTap = {
                                controls = !controls
                                poke()
                            },
                            onDoubleTap = { offset ->
                                val back = offset.x < half
                                events.onSeekBy(if (back) -JUMP_MS else JUMP_MS)
                                tap = back to ((tap?.second ?: 0) + 1)
                            },
                        )
                    },
            )
        }
        tap?.let { (back, _) ->
            Text(
                stringResource(if (back) R.string.player_chrome_tapBack else R.string.player_chrome_tapForward, JUMP_SECONDS.toString()),
                style = SlouchcatType.strong,
                color = c.tx,
                modifier = Modifier
                    .align(if (back) Alignment.CenterStart else Alignment.CenterEnd)
                    .padding(horizontal = 64.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(c.scrim)
                    .padding(horizontal = 14.dp, vertical = 8.dp)
                    .semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        val busy = state.loading || state.switching || (state.buffering && state.error == null)
        if (busy && state.error == null) {
            CircularProgressIndicator(Modifier.align(Alignment.Center).size(48.dp), color = c.tx, strokeWidth = 3.dp)
        }
        AnimatedVisibility(visible = (controls || !state.playing) && state.error == null, enter = fadeIn(), exit = fadeOut()) {
            Controls(state, events, busy, dragging, onDrag = { dragging = it; poke() }, onSheet = { sheet = it; poke() }, thumbnail = thumbnail, poke = poke)
        }
        val bottomGap = if (controls || !state.playing) 124.dp else 28.dp
        Column(
            Modifier
                .align(Alignment.BottomEnd)
                .windowInsetsPadding(WindowInsets.safeDrawing)
                .padding(end = 24.dp, bottom = bottomGap)
                .widthIn(max = 460.dp),
            horizontalAlignment = Alignment.End,
        ) {
            state.countdown?.let { countdown -> NextCard(countdown, images, events) }
            if (state.countdown == null && state.error == null && state.showSkipIntro) {
                SkipButton(stringResource(R.string.app_player_skipIntro), events::onSkipIntro)
            }
            if (state.countdown == null && state.error == null && state.showSkipOutro) {
                SkipButton(stringResource(R.string.player_skipOutro), events::onSkipOutro)
            }
        }
        state.error?.let { ErrorPicture(it, events) }
    }
    sheet?.let { open ->
        PlayerSheetHost(open, state, events, images, onClose = {
            sheet = null
            poke()
        })
    }
}

@Composable
private fun Controls(
    state: PlayerState,
    events: PlayerEvents,
    busy: Boolean,
    dragging: Float?,
    onDrag: (Float?) -> Unit,
    onSheet: (PlayerSheet) -> Unit,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
    poke: () -> Unit,
) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(0f to c.scrim, 0.3f to c.scrim.copy(alpha = 0f), 0.6f to c.scrim.copy(alpha = 0f), 1f to c.scrim)),
    ) {
        Column(Modifier.fillMaxSize().windowInsetsPadding(WindowInsets.safeDrawing)) {
            TopRow(state, events, onInfo = { onSheet(PlayerSheet.Info) })
            Spacer(Modifier.weight(1f))
            BottomBar(state, events, dragging, onDrag, onSheet, thumbnail)
        }
        if (!busy) {
            Row(
                Modifier.align(Alignment.Center),
                horizontalArrangement = Arrangement.spacedBy(56.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                JumpButton(back = true) {
                    events.onSeekBy(-JUMP_MS)
                    poke()
                }
                PlayPause(state.playing && !state.ended) {
                    events.onTogglePlay()
                    poke()
                }
                JumpButton(back = false) {
                    events.onSeekBy(JUMP_MS)
                    poke()
                }
            }
        }
    }
}

@Composable
private fun TopRow(state: PlayerState, events: PlayerEvents, onInfo: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .padding(start = 8.dp, end = 8.dp, top = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        SlouchcatIconButton(SlouchcatIcon.Back, stringResource(R.string.app_player_close), events::onBack)
        Column(Modifier.weight(1f)) {
            state.item?.let { item ->
                Text(titleLine(item), style = SlouchcatType.strong.copy(fontSize = 17.sp), color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.semantics { heading() })
                subtitleLine(item, state.durationMs)?.let { Text(it, style = SlouchcatType.caption, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis) }
            }
        }
        if (state.info != null) SlouchcatIconButton(SlouchcatIcon.Info, stringResource(R.string.app_player_infoButton), onInfo)
    }
}

@Composable
private fun JumpButton(back: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .size(64.dp)
            .clip(CircleShape)
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        SlouchcatIconImage(
            if (back) SlouchcatIcon.Rew else SlouchcatIcon.Fwd,
            contentDescription = stringResource(if (back) R.string.player_chrome_back10 else R.string.player_chrome_forward10),
            size = 34.dp,
            tint = c.tx,
        )
        // The number inside the arrow (`.vp-jump span`); the icon's description says it already.
        Text(stringResource(R.string.app_player_jumpSeconds, JUMP_SECONDS.toString()), style = SlouchcatType.badge.copy(fontSize = 11.sp, fontWeight = FontWeight.ExtraBold), color = c.tx, modifier = Modifier.padding(top = 4.dp).clearAndSetSemantics {})
    }
}

@Composable
private fun PlayPause(playing: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Box(
        Modifier
            .size(84.dp)
            .clip(CircleShape)
            .background(c.bg.copy(alpha = 0.55f))
            .border(BorderStroke(1.dp, c.frame), CircleShape)
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        SlouchcatIconImage(
            if (playing) SlouchcatIcon.Pause else SlouchcatIcon.Play,
            contentDescription = stringResource(if (playing) R.string.player_pause else R.string.player_play),
            size = 40.dp,
            tint = c.tx,
        )
    }
}

@Composable
private fun BottomBar(
    state: PlayerState,
    events: PlayerEvents,
    dragging: Float?,
    onDrag: (Float?) -> Unit,
    onSheet: (PlayerSheet) -> Unit,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val duration = state.durationMs.coerceAtLeast(1)
    val shown = dragging?.let { (it * duration).toLong() } ?: state.positionMs
    Column(Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, bottom = 10.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            Text(timecode(shown), style = SlouchcatType.label, color = c.tx2, modifier = Modifier.widthIn(min = 52.dp))
            SeekBar(state, dragging, onDrag, events, thumbnail, Modifier.weight(1f))
            Text(stringResource(R.string.app_player_remaining, timecode(duration - shown)), style = SlouchcatType.label, color = c.tx2, textAlign = TextAlign.End, modifier = Modifier.widthIn(min = 52.dp))
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Tool(SlouchcatIcon.Audio, stringResource(R.string.player_tracks_audio), on = state.audio.firstOrNull()?.selected == false) { onSheet(PlayerSheet.Audio) }
            Tool(SlouchcatIcon.Subs, stringResource(R.string.player_tracks_subtitles), on = !state.subtitlesOff) { onSheet(PlayerSheet.Subtitles) }
            Tool(SlouchcatIcon.Hd, stringResource(R.string.player_settings_quality), on = state.qualities.any { it.current && it.quality != Quality.AUTO }) { onSheet(PlayerSheet.Quality) }
            Tool(SlouchcatIcon.Speed, if (state.speed == 1f) stringResource(R.string.player_settings_speed) else speedLabel(state.speed), on = state.speed != 1f) { onSheet(PlayerSheet.Speed) }
            Spacer(Modifier.weight(1f))
            if (state.isEpisode) {
                Tool(SlouchcatIcon.List, stringResource(R.string.app_player_episodes), on = false) {
                    events.onEpisodesOpened()
                    onSheet(PlayerSheet.Episodes)
                }
                SlouchcatIconButton(SlouchcatIcon.Prev, stringResource(R.string.player_chrome_previousEpisode), events::onPrevious, enabled = state.previous != null, tint = c.tx)
                SlouchcatIconButton(SlouchcatIcon.Next, stringResource(R.string.player_chrome_nextEpisode), events::onNext, enabled = state.next != null, tint = c.tx)
            }
        }
    }
}

/** A tool of the bar (`.vp-tool`): icon and word. */
@Composable
private fun Tool(icon: SlouchcatIcon, label: String, on: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    val color = if (on) c.accent else c.tx2
    Row(
        Modifier
            .defaultMinSize(minHeight = 48.dp, minWidth = 48.dp)
            .clip(RoundedCornerShape(24.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .padding(start = 12.dp, end = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        SlouchcatIconImage(icon, contentDescription = null, size = 22.dp, tint = color)
        Text(label, style = SlouchcatType.label.copy(fontSize = 14.sp), color = color, maxLines = 1)
    }
}

/**
 * `.vp-track`: rail, buffered part, played part, knob and chapter marks; drag or tap to seek, the
 * bubble with the preview tile and the time while dragging. A slider for the screen reader.
 */
@Composable
private fun SeekBar(
    state: PlayerState,
    dragging: Float?,
    onDrag: (Float?) -> Unit,
    events: PlayerEvents,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
    modifier: Modifier,
) {
    val c = SlouchcatTheme.colors
    val duration = state.durationMs.coerceAtLeast(1)
    var width by remember { mutableFloatStateOf(1f) }
    val played = dragging ?: (state.positionMs.toFloat() / duration).coerceIn(0f, 1f)
    val buffered = (state.bufferedMs.toFloat() / duration).coerceIn(0f, 1f)
    val label = stringResource(R.string.player_seek)
    val value = stringResource(R.string.player_time, timecode((played * duration).toLong()), timecode(duration))
    val density = LocalDensity.current
    Box(
        modifier
            .height(48.dp)
            .onSizeChanged { width = it.width.toFloat().coerceAtLeast(1f) }
            .pointerInput(duration) {
                detectTapGestures { offset -> events.onSeekTo(((offset.x / width).coerceIn(0f, 1f) * duration).toLong()) }
            }
            .pointerInput(duration) {
                var last = 0f
                detectHorizontalDragGestures(
                    onDragStart = { offset ->
                        last = (offset.x / width).coerceIn(0f, 1f)
                        onDrag(last)
                    },
                    onDragEnd = {
                        events.onSeekTo((last * duration).toLong())
                        onDrag(null)
                    },
                    onDragCancel = { onDrag(null) },
                ) { change, _ ->
                    last = (change.position.x / width).coerceIn(0f, 1f)
                    onDrag(last)
                }
            }
            .semantics {
                contentDescription = label
                stateDescription = value
                progressBarRangeInfo = ProgressBarRangeInfo(played, 0f..1f)
                setProgress { target ->
                    events.onSeekTo((target.coerceIn(0f, 1f) * duration).toLong())
                    true
                }
            },
        contentAlignment = Alignment.CenterStart,
    ) {
        Box(Modifier.fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp)).background(c.tx.copy(alpha = 0.28f)))
        Box(Modifier.fillMaxWidth(buffered.coerceAtLeast(played)).height(4.dp).clip(RoundedCornerShape(2.dp)).background(c.tx.copy(alpha = 0.45f)))
        Box(Modifier.fillMaxWidth(played).height(4.dp).clip(RoundedCornerShape(2.dp)).background(c.accent))
        // Chapter marks: a short gap in the rail at each chapter start but the first.
        state.chapters.filter { it.startMs > 0 && it.startMs < duration }.forEach { chapter ->
            val x = with(density) { (width * chapter.startMs / duration).toDp() }
            Box(Modifier.offset(x = x - 1.dp).width(2.dp).height(8.dp).background(c.bg))
        }
        val knob = with(density) { (width * played).toDp() }
        Box(
            Modifier
                .offset(x = knob - 8.dp)
                .size(if (dragging != null) 20.dp else 16.dp)
                .clip(CircleShape)
                .background(c.accent),
        )
        if (dragging != null) {
            val time = (dragging * duration).toLong()
            val bubbleWidth = 176.dp
            val bubbleX = with(density) {
                val half = bubbleWidth.toPx() / 2
                (width * dragging).coerceIn(half, (width - half).coerceAtLeast(half)) - half
            }
            Column(
                Modifier
                    .offset { IntOffset(bubbleX.roundToInt(), -(with(density) { 136.dp.toPx() }).roundToInt()) }
                    .width(bubbleWidth)
                    .clip(RoundedCornerShape(10.dp))
                    .background(c.elev)
                    .border(BorderStroke(2.dp, c.tx), RoundedCornerShape(10.dp)),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f).background(c.s2)) {
                    state.trickplay?.at(time)?.let { tile -> thumbnail(tile, Modifier.fillMaxSize()) }
                    Text(
                        timecode(time),
                        style = SlouchcatType.label,
                        color = c.tx,
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .padding(bottom = 6.dp)
                            .clip(RoundedCornerShape(6.dp))
                            .background(c.scrim)
                            .padding(horizontal = 8.dp, vertical = 2.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun SkipButton(label: String, onClick: () -> Unit) {
    SlouchcatButton(label, onClick = onClick, kind = ButtonKind.Secondary, trailingIcon = SlouchcatIcon.Next)
}

/** `.vp-next`: the still of the next episode with the ring, the countdown, «Jetzt abspielen» and «Abbrechen». */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun NextCard(countdown: com.slouchcat.core.player.Countdown, images: ServerImages?, events: PlayerEvents) {
    val c = SlouchcatTheme.colors
    val next = countdown.next
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(18.dp))
            .background(c.elev)
            .border(BorderStroke(1.dp, c.frame), RoundedCornerShape(18.dp))
            .padding(12.dp)
            .semantics(mergeDescendants = false) { liveRegion = LiveRegionMode.Polite },
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.width(150.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(10.dp))) {
            if (images != null) {
                com.slouchcat.app.mobile.media.MediaImage(images.backdrops(next.images).or(images.posters(next.images)), next.title, Modifier.fillMaxSize())
            }
            countdown.secondsLeft?.let { left ->
                val share = ((com.slouchcat.core.player.PlaybackController.AUTO_NEXT_MS / 1000 - left).toFloat() / (com.slouchcat.core.player.PlaybackController.AUTO_NEXT_MS / 1000)).coerceIn(0f, 1f)
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(4.dp).background(c.tx.copy(alpha = 0.3f)))
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth(share).height(4.dp).background(c.accent))
            }
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                countdown.secondsLeft?.let { pluralStringResource(R.plurals.app_player_nextIn, it, it) } ?: stringResource(R.string.player_next_title),
                style = SlouchcatType.label,
                color = c.accent,
            )
            Text(titleLine(next), style = SlouchcatType.strong.copy(fontSize = 15.sp), color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            // The words of a button never break (long languages, large font): a button that does not
            // fit moves to the next line, the card grows.
            FlowRow(
                Modifier.padding(top = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                SlouchcatButton(stringResource(R.string.app_player_playNow), onClick = events::onNext, small = true, leadingIcon = SlouchcatIcon.Play)
                SlouchcatButton(stringResource(R.string.player_next_cancel), onClick = events::onCancelCountdown, small = true, kind = ButtonKind.Ghost)
            }
        }
    }
}

/** The picture of an error: icon, title, text, «Erneut versuchen» where it helps, «Zurück». */
@Composable
private fun ErrorPicture(error: PlayerError, events: PlayerEvents) {
    val c = SlouchcatTheme.colors
    val (title, body) = when (error) {
        PlayerError.NotFound -> stringResource(R.string.app_player_error_notFoundTitle) to stringResource(R.string.app_player_error_notFoundBody)
        PlayerError.WatchLimit -> stringResource(R.string.player_watchLimit_title) to stringResource(R.string.player_watchLimit_bodyUnknown)
        PlayerError.StreamLimit -> stringResource(R.string.player_streamLimit_title) to stringResource(R.string.player_streamLimit_bodyUnknown)
        PlayerError.StoppedElsewhere -> stringResource(R.string.player_ended_title) to stringResource(R.string.app_player_error_stoppedBody)
        PlayerError.SignedOut -> stringResource(R.string.player_ended_title) to stringResource(R.string.player_ended_body)
        is PlayerError.Network -> stringResource(R.string.app_player_error_networkTitle) to
            stringResource(if (error.retrying) R.string.app_player_error_networkRetrying else R.string.app_player_error_networkBody)
        is PlayerError.Busy -> stringResource(R.string.player_busy_title) to
            stringResource(if (error.retrying) R.string.player_busy_body else R.string.player_error_body)
        PlayerError.Unplayable -> stringResource(R.string.app_player_error_unplayableTitle) to stringResource(R.string.app_player_error_unplayableBody)
        PlayerError.Failed -> stringResource(R.string.player_error_title) to stringResource(R.string.player_error_body)
    }
    val waits = (error is PlayerError.Busy && error.retrying) || (error is PlayerError.Network && error.retrying)
    val retry = error == PlayerError.StreamLimit || error == PlayerError.StoppedElsewhere || error == PlayerError.Failed ||
        (error is PlayerError.Network && !error.retrying) || (error is PlayerError.Busy && !error.retrying)
    Box(Modifier.fillMaxSize().background(c.scrim).windowInsetsPadding(WindowInsets.safeDrawing)) {
        SlouchcatIconButton(SlouchcatIcon.Back, stringResource(R.string.app_player_close), events::onBack, modifier = Modifier.padding(8.dp))
        Column(
            Modifier
                .align(Alignment.Center)
                .widthIn(max = 480.dp)
                .padding(24.dp)
                .semantics(mergeDescendants = false) { liveRegion = LiveRegionMode.Polite },
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (waits) {
                CircularProgressIndicator(Modifier.size(36.dp), color = c.tx, strokeWidth = 3.dp)
            } else {
                SlouchcatIconImage(SlouchcatIcon.Alert, contentDescription = null, size = 36.dp, tint = c.warn)
            }
            Text(title, style = SlouchcatType.sheetTitle, color = c.tx, textAlign = TextAlign.Center, modifier = Modifier.semantics { heading() })
            Text(body, style = SlouchcatType.lede, color = c.tx2, textAlign = TextAlign.Center)
            Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                if (retry) SlouchcatButton(stringResource(R.string.app_state_retry), onClick = events::onRetry)
                SlouchcatButton(stringResource(R.string.common_back), onClick = events::onBack, kind = ButtonKind.Ghost)
            }
        }
    }
}
