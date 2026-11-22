package com.slouchcat.app.tv.player

import android.text.format.DateFormat
import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.Dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.compose.ui.layout.layout
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import androidx.compose.ui.focus.onFocusChanged
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.RemoteKey
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.OnRemoteKey
import com.slouchcat.app.tv.ui.TvBusy
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvRemoteKeys
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.TrickplayTile
import com.slouchcat.core.strings.R
import kotlinx.coroutines.delay
import java.util.Date
import kotlin.math.abs

/** The measures of the player (tokens `tv.size.player`, half the mockup's canvas). */
internal val P = SlouchcatTokens.Tv.Size.Player

/** The menus of the player (mockup `brPlMenu`), each a side menu from the right. */
enum class PlayerMenu { Audio, Subtitles, Quality, Episodes }

/** Focus keys of the page. */
internal object PlayerKeys {
    const val BAR = "bar"
    const val SKIP = "skip"
    const val NEXT_PLAY = "next-play"
    const val NEXT_CANCEL = "next-cancel"
    const val AUDIO = "btn-audio"
    const val SUBTITLES = "btn-subtitles"
    const val QUALITY = "btn-quality"
    const val EPISODES = "btn-episodes"
    const val PREVIOUS = "btn-previous"
    const val NEXT = "btn-next"
    const val INFO = "btn-info"
    const val RETRY = "retry"
    const val LEAVE = "leave"

    /** Test tags of the controls (shown or hidden; they stay composed for the focus). */
    const val CONTROLS_SHOWN = "player-controls"
    const val CONTROLS_HIDDEN = "player-controls-hidden"

    /** A chapter mark on the bar (stage 5). */
    const val CHAPTER_MARK = "chapter-mark"
    val ROW = setOf(AUDIO, SUBTITLES, QUALITY, EPISODES, PREVIOUS, NEXT, INFO)
}

/**
 * The video player of the TV (concept 26 5.1; mockup `/play/:i`, photos `22-play-0`, `23-play-2-e-0`,
 * `24-play-2-e-2-at-end`, `47`–`52`): the picture of [surface], above it the controls for the
 * remote. OK pauses and plays, Links and Rechts spool (steps that grow while a key is held, with
 * the trickplay picture of the target), Hoch and Runter show the controls; Zurück closes the info,
 * then hides the controls, then leaves. The menus Ton, Untertitel, Qualität and Folgen come
 * from the right. The skip button and the card of the next episode float above the picture.
 * [autoHide]: the controls hide after 4 s of playing without a key (never with TalkBack, where
 * they stay reachable). [now]: the clock of the media keys (keys of the D-pad bring their own time),
 * [wallClock]: the time of day.
 */
@Composable
fun PlayerScreen(
    state: PlayerState,
    sound: SoundState,
    seasons: SeasonsMenu,
    events: PlayerEvents,
    images: ServerImages?,
    modifier: Modifier = Modifier,
    remoteKeys: TvRemoteKeys? = null,
    autoHide: Boolean = true,
    initialControls: Boolean = true,
    /** The picture; `true` while the controls show (the subtitles move up above them). */
    surface: @Composable (Boolean) -> Unit = {},
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit = { _, _ -> },
    now: () -> Long = { android.os.SystemClock.uptimeMillis() },
    wallClock: () -> Long = System::currentTimeMillis,
) {
    val c = SlouchcatTheme.colors
    val focus = rememberFocusMemory()
    var controls by rememberSaveable { mutableStateOf(initialControls) }
    var info by rememberSaveable { mutableStateOf(false) }
    var menu by rememberSaveable { mutableStateOf<PlayerMenu?>(null) }
    var row by rememberSaveable { mutableStateOf(PlayerKeys.AUDIO) }
    // Every key counts as a sign of life: the controls stay 4 s after the last one.
    var touched by remember { mutableIntStateOf(0) }
    val scrub = remember { Scrub() }
    var scrubbed by remember { mutableIntStateOf(0) }
    // Links or Rechts held down: the seek waits until the key is let go.
    var holding by remember { mutableStateOf(false) }
    val current by rememberUpdatedState(state)

    val countdown = state.countdown
    val skipShown = state.error == null && !state.loading && (state.showSkipIntro || state.showSkipOutro)
    val nextShown = state.error == null && countdown != null

    fun show(key: String? = null) {
        controls = true
        touched++
        if (key != null) focus.focus(key)
    }

    fun hide() {
        controls = false
        focus.focus(
            when {
                nextShown -> PlayerKeys.NEXT_PLAY
                skipShown -> PlayerKeys.SKIP
                else -> PlayerKeys.BAR
            },
        )
    }

    fun spool(direction: Int, repeat: Boolean, at: Long = now()) {
        val s = current
        if (s.info == null || s.error != null) return
        if (scrub.step(direction, at, s.positionMs, s.durationMs, repeat)) scrubbed++
    }

    fun commit() {
        scrub.commit()?.let(events::onSeekTo)
        scrubbed++
    }

    fun toggle() {
        if (scrub.targetMs != null) commit() else events.onTogglePlay()
    }

    // The player seeks once the presses stop and no arrow is held (Scrub: not on every step).
    LaunchedEffect(scrubbed) {
        if (scrub.targetMs == null) return@LaunchedEffect
        delay(Scrub.COMMIT_MS)
        while (holding) delay(Scrub.REPEAT_MS)
        commit()
    }
    // The controls hide while it plays and nobody presses (mockup: 4 s).
    LaunchedEffect(controls, state.playing, touched, menu, info, scrubbed, autoHide) {
        if (!controls || !state.playing || menu != null || !autoHide || scrub.targetMs != null) return@LaunchedEffect
        delay(HIDE_MS)
        hide()
    }
    // The card of the next episode takes the focus when it comes (mockup `brPlDraw`).
    LaunchedEffect(nextShown, menu) {
        // Not behind an open menu (its focus stays caught there); when it closes.
        if (menu != null) return@LaunchedEffect
        if (nextShown) {
            focus.focus(PlayerKeys.NEXT_PLAY)
        } else if (focus.last == PlayerKeys.NEXT_PLAY || focus.last == PlayerKeys.NEXT_CANCEL) {
            focus.focus(PlayerKeys.BAR)
        }
    }
    LaunchedEffect(skipShown, controls) {
        if (skipShown && !controls && !nextShown) focus.focus(PlayerKeys.SKIP)
        if (!skipShown && focus.last == PlayerKeys.SKIP) focus.focus(PlayerKeys.BAR)
    }

    if (remoteKeys != null) {
        OnRemoteKey(remoteKeys) { key ->
            when (key) {
                RemoteKey.PlayPause -> {
                    toggle()
                    show()
                }
                RemoteKey.Play -> events.onPlay()
                RemoteKey.Pause -> {
                    events.onPause()
                    show()
                }
                RemoteKey.Stop -> events.onBack()
                RemoteKey.Next -> events.onNext()
                RemoteKey.Previous -> events.onPrevious()
                RemoteKey.FastForward, RemoteKey.Rewind -> {
                    spool(if (key == RemoteKey.FastForward) 1 else -1, repeat = false)
                    show()
                }
            }
        }
    }

    // Zurück: the info first, then the controls; then the page leaves (the NavHost).
    // Zurück while spooling drops the target first (no jump after the key).
    BackHandler(enabled = state.error == null && (info || controls || scrub.targetMs != null)) {
        when {
            scrub.targetMs != null -> {
                scrub.reset()
                scrubbed++
            }
            info -> info = false
            else -> hide()
        }
    }
    // A key held when the app leaves never sends its release.
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) { holding = false }

    val inTheWay = state.error != null
    Box(
        modifier
            .fillMaxSize()
            .background(c.bg)
            .onPreviewKeyEvent { event ->
                if (event.key == Key.DirectionLeft || event.key == Key.DirectionRight) holding = event.type == KeyEventType.KeyDown
                if (event.type != KeyEventType.KeyDown || inTheWay) return@onPreviewKeyEvent false
                touched++
                if (controls) return@onPreviewKeyEvent false
                hiddenKey(event, focus, onSpool = { spool(it, event.nativeKeyEvent.repeatCount > 0, event.nativeKeyEvent.eventTime) }, onToggle = ::toggle, show = ::show, row = row)
            },
    ) {
        surface(controls)
        val dim by animateFloatAsState(if (!state.playing && !state.loading && state.info != null && state.error == null) 1f else 0f, label = "")
        Box(Modifier.fillMaxSize().alpha(dim).background(SlouchcatTokens.Tv.Color.ShadeMid))

        if (state.error == null && state.info != null) {
            Controls(
                state = state,
                visible = controls,
                focus = focus,
                scrub = scrub,
                scrubbed = scrubbed,
                images = images,
                events = events,
                wallClock = wallClock,
                thumbnail = thumbnail,
                info = info,
                row = row,
                onSpool = { direction, repeat, at -> spool(direction, repeat, at) },
                onToggle = ::toggle,
                onRow = { row = it },
                onMenu = { menu = it },
                onInfo = { info = !info },
                onShow = ::show,
            )
        }

        if (skipShown && !nextShown) {
            val outro = !state.showSkipIntro
            TvButton(
                stringResource(if (outro) R.string.player_skipOutro else R.string.app_player_skipIntro),
                onClick = { if (outro) events.onSkipOutro() else events.onSkipIntro() },
                icon = SlouchcatIcon.Fwd.vector,
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = TvDimens.safeX, bottom = if (controls) P.FloatBottom else P.FloatBottomHidden)
                    .focusKey(focus, PlayerKeys.SKIP),
            )
        }
        if (countdown != null && nextShown) {
            NextCard(
                state = state,
                images = images,
                focus = focus,
                events = events,
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = TvDimens.safeX, bottom = if (controls) P.FloatBottom else P.FloatBottomHidden),
            )
        }
        if (info && state.error == null && state.info != null) InfoPanel(state, sound, Modifier.align(Alignment.TopEnd))

        val error = state.error
        when {
            error != null && waits(error) -> Waiting(error, Modifier.align(Alignment.Center))
            error != null -> ErrorPicture(error, focus, events, Modifier.fillMaxSize())
            state.loading || state.switching -> TvBusy(
                stringResource(if (state.switching) R.string.player_switching else R.string.common_loading),
                Modifier.align(Alignment.Center),
            )
        }
    }

    if (state.error != null) {
        focus.StartFocus(if (retryable(state.error)) PlayerKeys.RETRY else PlayerKeys.LEAVE, state.error)
    } else {
        focus.StartFocus(if (nextShown) PlayerKeys.NEXT_PLAY else PlayerKeys.BAR, state.info == null)
    }

    menu?.let { open ->
        PlayerMenuPanel(
            menu = open,
            state = state,
            sound = sound,
            seasons = seasons,
            images = images,
            events = events,
            onClose = {
                menu = null
                focus.restore()
            },
        )
    }
}

/** A key while the controls are hidden (mockup `brPlKey` without `ctl`): `true` when taken. */
private fun hiddenKey(event: KeyEvent, focus: FocusMemory, onSpool: (Int) -> Unit, onToggle: () -> Unit, show: (String?) -> Unit, row: String): Boolean {
    val onSkip = focus.last == PlayerKeys.SKIP
    val onNext = focus.last == PlayerKeys.NEXT_PLAY || focus.last == PlayerKeys.NEXT_CANCEL
    return when (event.key) {
        Key.DirectionLeft, Key.DirectionRight -> {
            if (onNext) return false
            onSpool(if (event.key == Key.DirectionLeft) -1 else 1)
            show(if (onSkip) null else PlayerKeys.BAR)
            true
        }
        Key.DirectionCenter, Key.Enter, Key.NumPadEnter -> {
            if (onSkip || onNext) return false
            onToggle()
            show(null)
            true
        }
        Key.DirectionDown -> {
            show(if (onSkip || onNext) null else row)
            true
        }
        Key.DirectionUp -> {
            show(if (onSkip || onNext) null else PlayerKeys.BAR)
            true
        }
        else -> false
    }
}

/** The controls (mockup `.br-ctl`): title and clock at the top, state, bar, times and buttons at the bottom. */
@Composable
private fun Controls(
    state: PlayerState,
    visible: Boolean,
    focus: FocusMemory,
    scrub: Scrub,
    scrubbed: Int,
    images: ServerImages?,
    events: PlayerEvents,
    wallClock: () -> Long,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
    info: Boolean,
    row: String,
    onSpool: (Int, Boolean, Long) -> Unit,
    onToggle: () -> Unit,
    onRow: (String) -> Unit,
    onMenu: (PlayerMenu) -> Unit,
    onInfo: () -> Unit,
    onShow: (String?) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val shown by animateFloatAsState(if (visible) 1f else 0f, label = "")
    val item = state.item
    Box(Modifier.fillMaxSize().alpha(shown).testTag(if (visible) PlayerKeys.CONTROLS_SHOWN else PlayerKeys.CONTROLS_HIDDEN)) {
        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    0f to SlouchcatTokens.Tv.Color.ShadeStrong,
                    TOP_MID to SlouchcatTokens.Tv.Color.ShadeMid,
                    TOP_END to SlouchcatTokens.Tv.Color.ShadeNone,
                    BOTTOM_START to SlouchcatTokens.Tv.Color.ShadeNone,
                    BOTTOM_MID to SlouchcatTokens.Tv.Color.ShadeMid,
                    1f to SlouchcatTokens.Tv.Color.ShadeStrong,
                ),
            ),
        )
        // Title and clock (mockup `.br-ptop`).
        Row(
            Modifier.fillMaxWidth().padding(horizontal = TvDimens.safeX, vertical = TvDimens.safeY),
            horizontalArrangement = Arrangement.spacedBy(P.TitleGap),
        ) {
            Column(Modifier.weight(1f)) {
                if (item != null) {
                    overLine(item, state.durationMs)?.let { Text(it, style = TvType.largeStrong, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis) }
                    Text(
                        titleLine(item),
                        style = TvType.h1,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(top = P.OverGap).semantics { heading() },
                    )
                }
            }
            Clock(state, wallClock)
        }
        // State, bar, times, buttons (mockup `.br-pbot`).
        Column(Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(horizontal = TvDimens.safeX, vertical = TvDimens.safeY)) {
            Row(Modifier.heightIn(min = P.State), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                val paused = !state.playing && !state.loading && !state.buffering
                if (state.ended || paused) {
                    TvIcon(if (state.ended) SlouchcatIcon.Check else SlouchcatIcon.Pause, P.StateIcon)
                    Text(
                        stringResource(if (state.ended) R.string.tv_player_ended else R.string.tv_player_paused),
                        style = TvType.largeStrong,
                        modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                    )
                }
            }
            SeekBar(state, focus, scrub, scrubbed, thumbnail, onSpool, onToggle, onDown = { onShow(row) })
            val position = scrub.targetMs ?: state.positionMs
            Row(Modifier.fillMaxWidth().padding(top = P.TimesTop)) {
                Text(timecode(position), style = TvType.bodyStrong, modifier = Modifier.weight(1f))
                Text(stringResource(R.string.app_player_remaining, timecode((state.durationMs - position).coerceAtLeast(0))), style = TvType.bodyStrong, color = c.tx2)
            }
            Buttons(state, focus, info, events, onRow, onMenu, onInfo, Modifier.padding(top = P.ButtonsTop))
        }
    }
}

/** The time of day and «Endet um …» (mockup `.br-pclock`). */
@Composable
private fun Clock(state: PlayerState, wallClock: () -> Long) {
    val c = SlouchcatTheme.colors
    val context = LocalContext.current
    val now by produceState(wallClock()) {
        while (true) {
            delay(CLOCK_MS)
            value = wallClock()
        }
    }
    val format = remember(context) { DateFormat.getTimeFormat(context) }
    val left = ((state.durationMs - state.positionMs).coerceAtLeast(0) / state.speed.coerceAtLeast(MIN_SPEED)).toLong()
    Column(horizontalAlignment = Alignment.End) {
        Text(format.format(Date(now)), style = TvType.largeStrong)
        if (state.durationMs > 0) Text(stringResource(R.string.tv_player_endsAt, format.format(Date(now + left))), style = TvType.small, color = c.tx2)
    }
}

/**
 * `.br-seek`: the bar with the accent up to the position and a knob in focus; Links and Rechts
 * spool, OK seeks at once or plays and pauses, the bubble shows the target with its trickplay
 * picture. Told as a slider to a screen reader.
 */
@Composable
private fun SeekBar(
    state: PlayerState,
    focus: FocusMemory,
    scrub: Scrub,
    scrubbed: Int,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
    onSpool: (Int, Boolean, Long) -> Unit,
    onToggle: () -> Unit,
    onDown: () -> Unit,
) {
    val c = SlouchcatTheme.colors
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    // Read scrubbed so the bar draws the new target.
    val position = if (scrubbed >= 0) scrub.targetMs ?: state.positionMs else state.positionMs
    val share = if (state.durationMs > 0) (position.toFloat() / state.durationMs).coerceIn(0f, 1f) else 0f
    val label = stringResource(R.string.player_seek)
    val value = stringResource(R.string.player_time, timecode(position), timecode(state.durationMs))
    BoxWithConstraints(
        Modifier
            .fillMaxWidth()
            .height(P.Seek)
            .focusKey(focus, PlayerKeys.BAR)
            .onKeyEvent { event ->
                if (event.type != KeyEventType.KeyDown) return@onKeyEvent false
                when (event.key) {
                    Key.DirectionLeft, Key.DirectionRight -> {
                        onSpool(if (event.key == Key.DirectionLeft) -1 else 1, event.nativeKeyEvent.repeatCount > 0, event.nativeKeyEvent.eventTime)
                        true
                    }
                    Key.DirectionCenter, Key.Enter, Key.NumPadEnter -> {
                        onToggle()
                        true
                    }
                    Key.DirectionDown -> {
                        onDown()
                        true
                    }
                    else -> false
                }
            }
            .semantics {
                contentDescription = label
                stateDescription = value
                progressBarRangeInfo = ProgressBarRangeInfo(share, 0f..1f)
                onClick(label) {
                    onToggle()
                    true
                }
            }
            .focusable(interactionSource = interaction),
        contentAlignment = Alignment.CenterStart,
    ) {
        val track = if (focused) P.TrackFocus else P.Track
        val shape = RoundedCornerShape(track / 2)
        Box(Modifier.fillMaxWidth().height(track).clip(shape).background(SlouchcatTokens.Tv.Color.GlassStrong))
        Box(Modifier.fillMaxWidth(share).height(track).clip(shape).background(c.accent))
        // The chapters (paper 5.1, web SeekBar `.mark`): a gap in the bar where each one begins.
        for (start in chapterMarks(state.chapters, state.durationMs)) {
            Box(
                Modifier
                    .offset(x = maxWidth * (start.toFloat() / state.durationMs) - P.ChapterMark / 2)
                    .size(P.ChapterMark, track)
                    .background(c.bg)
                    .testTag(PlayerKeys.CHAPTER_MARK),
            )
        }
        if (focused) {
            val x = maxWidth * share - P.Knob / 2
            Box(
                Modifier
                    .offset(x = x)
                    .size(P.Knob)
                    .border(BorderStroke(P.KnobRing, SlouchcatTokens.Tv.Color.GlassStrong), CircleShape)
                    .padding(P.KnobRing)
                    .clip(CircleShape)
                    .background(c.tx),
            )
        }
        val target = scrub.targetMs
        if (target != null) Bubble(state, target, scrub.deltaMs, share, maxWidth, thumbnail)
    }
}

/** `.br-bub`: the target of the spooling above the bar, with its trickplay picture and the way («+2:30»). */
@Composable
private fun Bubble(
    state: PlayerState,
    target: Long,
    delta: Long,
    share: Float,
    width: Dp,
    thumbnail: @Composable (TrickplayTile, Modifier) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val density = LocalDensity.current
    val tile = state.trickplay?.at(target)
    val half = P.Preview / 2
    val x = (width * share - half).coerceIn(0.dp, (width - P.Preview).coerceAtLeast(0.dp))
    Column(
        Modifier
            .width(P.Preview)
            // Above the bar, whatever its own height (the picture comes only with trickplay).
            .layout { measurable, constraints ->
                val placeable = measurable.measure(constraints.copy(minHeight = 0, maxHeight = Constraints.Infinity))
                layout(placeable.width, 0) {
                    placeable.place(with(density) { x.roundToPx() }, -placeable.height - with(density) { P.PreviewBottom.roundToPx() })
                }
            },
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(P.PreviewGap),
    ) {
        if (tile != null) {
            val shape = RoundedCornerShape(P.PreviewRadius)
            Box(
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(tile.width.toFloat() / tile.height.coerceAtLeast(1))
                    .border(BorderStroke(P.PreviewRing, c.tx), shape)
                    .clip(shape)
                    .background(c.s2),
            ) { thumbnail(tile, Modifier.fillMaxSize()) }
        }
        Text(timecode(target), style = TvType.largeStrong)
        // The chapter at the target (web SeekBar: its title, else «Kapitel n»).
        val chapter = chapterIndexAt(state.chapters, target)
        if (chapter >= 0) {
            Text(
                state.chapters[chapter].title?.takeIf { it.isNotBlank() } ?: stringResource(R.string.player_chapters_untitled, (chapter + 1).toString()),
                style = TvType.bodyStrong,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        val way = abs(delta)
        val time = if (way < MINUTE_MS) stringResource(R.string.format_seconds, (way / SECOND_MS).toString()) else timecode(way)
        Text(
            stringResource(if (delta < 0) R.string.tv_player_behind else R.string.tv_player_ahead, time),
            style = TvType.label,
            modifier = Modifier.clip(RoundedCornerShape(TvDimens.radiusField)).background(SlouchcatTokens.Tv.Color.ShadeStrong).padding(horizontal = TvDimens.m),
        )
    }
}

/** `.br-pbtns`: Ton, Untertitel, Qualität, Folgen, Vorige, Nächste (episodes), Info (no speed: concept 26 5.1 and the mockup have none). */
@Composable
private fun Buttons(
    state: PlayerState,
    focus: FocusMemory,
    info: Boolean,
    events: PlayerEvents,
    onRow: (String) -> Unit,
    onMenu: (PlayerMenu) -> Unit,
    onInfo: () -> Unit,
    modifier: Modifier,
) {
    // Runter from the bar comes back to the button that had the focus last (mockup `rowF`).
    fun Modifier.key(key: String) = focusKey(focus, key).onFocusChanged { if (it.isFocused) onRow(key) }
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(P.ButtonGap)) {
        PlayerButton(stringResource(R.string.player_tracks_audio), SlouchcatIcon.Audio.vector, { onMenu(PlayerMenu.Audio) }, Modifier.key(PlayerKeys.AUDIO))
        PlayerButton(stringResource(R.string.player_tracks_subtitles), SlouchcatIcon.Subs.vector, { onMenu(PlayerMenu.Subtitles) }, Modifier.key(PlayerKeys.SUBTITLES))
        if (state.qualities.isNotEmpty()) {
            PlayerButton(stringResource(R.string.player_settings_quality), SlouchcatIcon.Hd.vector, { onMenu(PlayerMenu.Quality) }, Modifier.key(PlayerKeys.QUALITY))
        }
        if (state.isEpisode) {
            PlayerButton(stringResource(R.string.app_player_episodes), SlouchcatIcon.List.vector, {
                events.onEpisodesOpened()
                onMenu(PlayerMenu.Episodes)
            }, Modifier.key(PlayerKeys.EPISODES))
            PlayerButton(stringResource(R.string.tv_player_previous), SlouchcatIcon.Prev.vector, events::onPrevious, Modifier.key(PlayerKeys.PREVIOUS), enabled = state.previous != null)
            PlayerButton(stringResource(R.string.tv_player_next), SlouchcatIcon.Next.vector, events::onNext, Modifier.key(PlayerKeys.NEXT), enabled = state.next != null)
        }
        PlayerButton(stringResource(R.string.tv_player_info), SlouchcatIcon.Info.vector, onInfo, Modifier.key(PlayerKeys.INFO), pressed = info)
    }
}

/** `.br-pb`: a pill of 36 dp with icon and text; light in focus, dimmed and out of the focus order when [enabled] is `false`, a frame of the accent when [pressed]. */
@Composable
private fun PlayerButton(
    text: String,
    icon: ImageVector,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    pressed: Boolean = false,
    trailing: String? = null,
) {
    val c = SlouchcatTheme.colors
    val frame = if (pressed) c.accent else Color.Transparent
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .focusProperties { canFocus = enabled }
            .height(P.Button)
            .alpha(if (enabled) 1f else TvMotion.disabled)
            .semantics { if (pressed) selected = true },
        shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = SlouchcatTokens.Tv.Color.Glass,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = SlouchcatTokens.Tv.Color.Glass,
            disabledContentColor = c.tx,
        ),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, frame), shape = CircleShape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = CircleShape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(
            Modifier.align(Alignment.Center).padding(horizontal = P.ButtonPad),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TvIcon(icon, P.ButtonIcon)
            Text(text, style = TvType.bodyStrong, maxLines = 1)
            if (trailing != null) Text(trailing, style = TvType.bodyStrong, maxLines = 1)
        }
    }
}

/** `.br-next`: the next episode with «Jetzt abspielen» and «Abbrechen», the countdown in its first line. */
@Composable
private fun NextCard(state: PlayerState, images: ServerImages?, focus: FocusMemory, events: PlayerEvents, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val countdown = state.countdown ?: return
    val next = countdown.next
    val shape = RoundedCornerShape(P.NextRadius)
    Row(
        modifier
            .width(P.Next)
            .clip(shape)
            .background(c.elev)
            .border(BorderStroke(TvDimens.borderThin, c.line), shape)
            .padding(P.NextPad),
        horizontalArrangement = Arrangement.spacedBy(P.NextGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        val image = images?.backdrops(next.images)?.forWidth(P.NextImage)
        Box(Modifier.width(P.NextImage).aspectRatio(WIDE).clip(RoundedCornerShape(P.PreviewRadius)).background(c.s2)) {
            if (image != null) TvImage(image, Modifier.fillMaxSize())
        }
        Column(Modifier.weight(1f)) {
            val left = countdown.secondsLeft
            Text(
                if (left != null) pluralStringResource(R.plurals.app_player_nextIn, left, left) else stringResource(R.string.player_next_title),
                style = TvType.label,
                color = c.tx2,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
            Text(titleLine(next), style = TvType.largeStrong, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(bottom = TvDimens.l))
            Row(horizontalArrangement = Arrangement.spacedBy(TvDimens.m)) {
                TvButton(stringResource(R.string.app_player_playNow), onClick = events::onNext, icon = SlouchcatIcon.Play.vector, kind = TvButtonKind.Primary, small = true, modifier = Modifier.focusKey(focus, PlayerKeys.NEXT_PLAY))
                TvButton(stringResource(R.string.player_next_cancel), onClick = events::onCancelCountdown, kind = TvButtonKind.Ghost, small = true, modifier = Modifier.focusKey(focus, PlayerKeys.NEXT_CANCEL))
            }
        }
    }
}

/** `.br-info`: the panel «Wiedergabe» with the rows and the line about the sound (mockup `brPlInfoHtml`). */
@Composable
private fun InfoPanel(state: PlayerState, sound: SoundState, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusDialog)
    Column(
        modifier
            .padding(top = P.InfoTop, end = TvDimens.safeX)
            .width(P.Info)
            .clip(shape)
            .background(c.scrim)
            .border(BorderStroke(TvDimens.borderThin, c.line), shape)
            .padding(horizontal = P.InfoPadX, vertical = P.InfoPadY),
    ) {
        Text(stringResource(R.string.app_playback_title), style = TvType.largeStrong, modifier = Modifier.semantics { heading() })
        Column(Modifier.padding(top = TvDimens.m)) {
            infoRows(state, sound).forEach { (key, value) ->
                Row(Modifier.fillMaxWidth().heightIn(min = P.InfoRow), verticalAlignment = Alignment.CenterVertically) {
                    Text(key, style = TvType.body, color = c.tx2, modifier = Modifier.width(P.InfoKey))
                    Text(value, style = TvType.body, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                }
            }
        }
        val through = sound.passthrough
        if (through != null) {
            Box(Modifier.padding(top = TvDimens.m).fillMaxWidth().height(TvDimens.borderThin).background(c.line))
            Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.m), verticalAlignment = Alignment.CenterVertically) {
                TvIcon(if (through) SlouchcatIcon.Check else SlouchcatIcon.Audio, TvDimens.Menu.Icon, tint = if (through) c.ok else c.tx3)
                Text(stringResource(if (through) R.string.tv_player_soundPassthrough else R.string.tv_player_soundDecoded), style = TvType.bodyStrong)
            }
        }
    }
}

/** Whether the controller is still at it (no connection, a busy server): a ring and a line, no buttons. */
private fun waits(error: PlayerError): Boolean = (error is PlayerError.Network && error.retrying) || (error is PlayerError.Busy && error.retrying)

/** Whether «Erneut versuchen» can help. */
internal fun retryable(error: PlayerError?): Boolean = when (error) {
    PlayerError.NotFound, PlayerError.WatchLimit, PlayerError.SignedOut, PlayerError.Unplayable, null -> false
    else -> true
}

@Composable
private fun Waiting(error: PlayerError, modifier: Modifier) {
    val text = if (error is PlayerError.Busy) stringResource(R.string.player_busy_body) else stringResource(R.string.app_player_error_networkRetrying)
    TvBusy(text, modifier.widthIn(max = P.Next))
}

/** The pictures of [PlayerError]: icon, title, body, «Erneut versuchen» where it helps, and «Zurück». */
@Composable
private fun ErrorPicture(error: PlayerError, focus: FocusMemory, events: PlayerEvents, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val (title, body) = when (error) {
        PlayerError.NotFound -> R.string.app_player_error_notFoundTitle to R.string.app_player_error_notFoundBody
        PlayerError.WatchLimit -> R.string.player_watchLimit_title to R.string.player_watchLimit_bodyUnknown
        PlayerError.StreamLimit -> R.string.player_streamLimit_title to R.string.player_streamLimit_bodyUnknown
        PlayerError.StoppedElsewhere -> R.string.player_ended_title to R.string.app_player_error_stoppedBody
        PlayerError.SignedOut -> R.string.player_ended_title to R.string.player_ended_body
        is PlayerError.Network -> R.string.app_player_error_networkTitle to R.string.app_player_error_networkBody
        is PlayerError.Busy -> R.string.player_busy_title to R.string.player_busy_body
        PlayerError.Unplayable -> R.string.app_player_error_unplayableTitle to R.string.app_player_error_unplayableBody
        PlayerError.Failed -> R.string.player_error_title to R.string.player_error_body
    }
    Box(modifier.background(c.bg), contentAlignment = Alignment.Center) {
        Column(Modifier.widthIn(max = TvDimens.Dialog.Width), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            TvIcon(SlouchcatIcon.Alert, P.FlashIcon, tint = c.warn)
            Text(stringResource(title), style = TvType.h2, modifier = Modifier.semantics { heading() })
            Text(stringResource(body), style = TvType.body, color = c.tx2)
            Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                if (retryable(error)) {
                    TvButton(stringResource(R.string.common_retry), onClick = events::onRetry, icon = SlouchcatIcon.Refresh.vector, kind = TvButtonKind.Primary, modifier = Modifier.focusKey(focus, PlayerKeys.RETRY))
                }
                TvButton(stringResource(R.string.common_back), onClick = events::onBack, icon = SlouchcatIcon.Back.vector, modifier = Modifier.focusKey(focus, PlayerKeys.LEAVE))
            }
        }
    }
}

/** The controls hide after this long without a key while the title plays (mockup: 4 s). */
const val HIDE_MS = 4_000L
private const val CLOCK_MS = 1_000L
private const val MIN_SPEED = 0.25f
private const val SECOND_MS = 1_000L
private const val MINUTE_MS = 60_000L
private const val WIDE = 16f / 9f
private const val TOP_MID = 0.12f
private const val TOP_END = 0.3f
private const val BOTTOM_START = 0.48f
private const val BOTTOM_MID = 0.76f
