package com.slouchcat.app.tv.ui

import androidx.activity.compose.BackHandler
import androidx.annotation.StringRes
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.paneTitle
import androidx.compose.ui.semantics.semantics
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * The focus of a page (BAUKASTEN «Fokus-Engine»): the start focus, and on the way back with Back the
 * element that had it before (saved with the page, like the scroll position of its lists). Mark each
 * focusable element of a page with [focusKey]; call [StartFocus] once with the element that gets the
 * focus when there is nothing to restore (`data-default`).
 *
 * ```
 * val focus = rememberFocusMemory()
 * TvButton(…, modifier = Modifier.focusKey(focus, "next"))
 * focus.StartFocus("next")
 * ```
 */
@Stable
class FocusMemory internal constructor(initial: String?) {
    /** The key of the element that had the focus last. */
    var last: String? by mutableStateOf(initial)
        internal set

    private val requesters = HashMap<String, FocusRequester>()

    internal var tick by mutableIntStateOf(0)

    internal fun requester(key: String): FocusRequester = requesters.getOrPut(key) { FocusRequester() }

    /** Moves the focus back to the remembered element (after a menu closed or a step changed). */
    fun restore() {
        tick++
    }

    /** Moves the focus to [key] now and remembers it. */
    fun focus(key: String) {
        last = key
        tick++
    }

    internal fun tryFocus(fallback: String?): Boolean {
        for (candidate in listOfNotNull(last, fallback).distinct()) {
            val requester = requesters[candidate] ?: continue
            // `false` (or an exception, by version) when nothing with this key is on the page now.
            if (runCatching { requester.requestFocus(FocusDirection.Enter) }.getOrDefault(false)) return true
        }
        return false
    }
}

/** A [FocusMemory] that survives the way forward and back (saved with the destination). */
@Composable
fun rememberFocusMemory(): FocusMemory =
    rememberSaveable(saver = Saver(save = { it.last.orEmpty() }, restore = { FocusMemory(it.ifEmpty { null }) })) { FocusMemory(null) }

/**
 * Makes this element known to [memory] under [key]. On a group (a field with its label, a row) the
 * focus goes to its first focusable child, and the key counts while anything in it has the focus.
 */
fun Modifier.focusKey(memory: FocusMemory, key: String): Modifier =
    focusRequester(memory.requester(key)).onFocusChanged { if (it.hasFocus) memory.last = key }

/**
 * The start focus of a page after its first frame (K26 stage 0: earlier, nothing is placed yet):
 * the remembered element, else [fallback]. [keys] start it again (content that arrives later).
 */
@Composable
fun FocusMemory.StartFocus(fallback: String, vararg keys: Any?) {
    LaunchedEffect(tick, *keys) {
        // Not placed yet (a page that just opened, a list that fills a frame later): a few frames more.
        repeat(START_FOCUS_FRAMES) {
            withFrameNanos { }
            if (tryFocus(fallback)) return@LaunchedEffect
        }
    }
}

/** How many frames [StartFocus] tries before it gives up (the page has no such element). */
private const val START_FOCUS_FRAMES = 10

/** The layer above the whole screen (rail included) where menus and dialogs are drawn. */
class TvOverlayHost {
    internal val entries = mutableStateListOf<OverlayEntry>()
}

internal class OverlayEntry(val content: @Composable () -> Unit)

val LocalTvOverlayHost = staticCompositionLocalOf<TvOverlayHost?> { null }

/** Draws what [TvOverlay]s put into [host]; the shell (and a screen test) puts it above everything. */
@Composable
fun TvOverlayLayer(host: TvOverlayHost) {
    for (entry in host.entries) key(entry) { entry.content() }
}

/** Puts [content] into the overlay layer of the screen while this composable is shown. */
@Composable
fun TvOverlay(content: @Composable () -> Unit) {
    val host = LocalTvOverlayHost.current
    if (host == null) {
        content()
        return
    }
    val current by rememberUpdatedState(content)
    val entry = remember { OverlayEntry { current() } }
    DisposableEffect(entry) {
        host.entries += entry
        onDispose { host.entries -= entry }
    }
}

/**
 * Wraps a screen with its overlay layer: the shell does it once for the app, a screen test for
 * itself. Menus and dialogs of [content] draw above it.
 */
@Composable
fun TvScreenHost(modifier: Modifier = Modifier, content: @Composable BoxScope.() -> Unit) {
    val host = remember { TvOverlayHost() }
    androidx.compose.runtime.CompositionLocalProvider(LocalTvOverlayHost provides host) {
        Box(modifier.fillMaxSize().background(SlouchcatTheme.colors.bg)) {
            content()
            TvOverlayLayer(host)
        }
    }
}

/** The frame of a menu or dialog: the focus stays inside (BAUKASTEN «Menüs und Dialoge»), Back closes. */
@Composable
private fun TrappedPanel(
    title: String,
    onDismiss: () -> Unit,
    initialFocus: FocusRequester?,
    panel: @Composable BoxScope.(Modifier) -> Unit,
) {
    val c = SlouchcatTheme.colors
    TvOverlay {
        BackHandler(onBack = onDismiss)
        Box(Modifier.fillMaxSize()) {
            Box(Modifier.fillMaxSize().background(c.scrim))
            panel(
                Modifier
                    .semantics { paneTitle = title }
                    .focusProperties { onExit = { cancelFocusChange() } }
                    .focusGroup(),
            )
        }
        LaunchedEffect(Unit) {
            withFrameNanos { }
            initialFocus?.let { runCatching { it.requestFocus() } }
        }
    }
}

/**
 * `menu(html)`: a side panel from the right with a title, its own scroll and the focus caught
 * inside. Back closes it; [initialFocus] is its `data-default`.
 */
@Composable
fun TvSideMenu(
    title: String,
    onDismiss: () -> Unit,
    initialFocus: FocusRequester?,
    content: @Composable ColumnScope.() -> Unit,
) {
    val c = SlouchcatTheme.colors
    TrappedPanel(title, onDismiss, initialFocus) { trap ->
        Column(
            trap
                .align(Alignment.CenterEnd)
                .width(TvDimens.Menu.Width)
                .fillMaxHeight()
                .background(c.elev)
                .border(BorderStroke(TvDimens.borderThin, c.line))
                .verticalScroll(rememberScrollState())
                .padding(start = TvDimens.Menu.PadStart, end = TvDimens.safeX, top = TvDimens.safeY, bottom = TvDimens.safeY),
        ) {
            Text(title, style = TvType.h2, modifier = Modifier.padding(bottom = TvDimens.Menu.TitleGap).semantics { heading() })
            content()
        }
    }
}

/** `menu(html, { dialog: true })`: a dialog in the middle, focus caught inside, Back closes it. */
@Composable
fun TvDialog(
    title: String,
    onDismiss: () -> Unit,
    initialFocus: FocusRequester?,
    modifier: Modifier = Modifier,
    showTitle: Boolean = true,
    content: @Composable ColumnScope.() -> Unit,
) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusDialog)
    TrappedPanel(title, onDismiss, initialFocus) { trap ->
        Column(
            trap
                .then(modifier)
                .align(Alignment.Center)
                .width(TvDimens.Dialog.Width)
                .heightIn(max = TvDimens.Dialog.MaxHeight)
                .background(c.elev, shape)
                .border(BorderStroke(TvDimens.borderThin, c.line), shape)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = TvDimens.Dialog.PadX, vertical = TvDimens.Dialog.PadY),
        ) {
            if (showTitle) Text(title, style = TvType.h2, modifier = Modifier.padding(bottom = TvDimens.Menu.TitleGap).semantics { heading() })
            content()
        }
    }
}

/** `.acts`: the row of buttons at the end of a menu or dialog. */
@Composable
fun TvActions(modifier: Modifier = Modifier, vertical: Boolean = false, content: @Composable () -> Unit) {
    if (vertical) {
        Column(modifier.padding(top = TvDimens.Menu.ActionsTop), verticalArrangement = Arrangement.spacedBy(TvDimens.l)) { content() }
    } else {
        Row(modifier.padding(top = TvDimens.Menu.ActionsTop), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) { content() }
    }
}

/** A short message of the app (`toast()`); one at a time, for 2.6 s. */
class TvToasts {
    sealed interface Message {
        data class Res(@StringRes val id: Int, val args: List<Any> = emptyList()) : Message

        data class Plain(val text: String) : Message
    }

    private val _current = MutableStateFlow<Pair<Int, Message>?>(null)
    val current: StateFlow<Pair<Int, Message>?> = _current.asStateFlow()
    private var serial = 0

    fun show(@StringRes id: Int, vararg args: Any) = show(Message.Res(id, args.toList()))

    fun show(text: String) = show(Message.Plain(text))

    private fun show(message: Message) {
        _current.value = ++serial to message
    }

    internal fun clear(id: Int) {
        if (_current.value?.first == id) _current.value = null
    }
}

/** `#toast`: the message at the bottom centre, above everything, told to a screen reader. */
@Composable
fun TvToastHost(toasts: TvToasts, modifier: Modifier = Modifier) {
    val shown by toasts.current.collectAsStateWithLifecycle()
    val resources = LocalResources.current
    val text = when (val message = shown?.second) {
        is TvToasts.Message.Res -> resources.getString(message.id, *message.args.toTypedArray())
        is TvToasts.Message.Plain -> message.text
        null -> null
    }
    LaunchedEffect(shown?.first) {
        val id = shown?.first ?: return@LaunchedEffect
        delay(TvMotion.TOAST_MS.toLong())
        toasts.clear(id)
    }
    Box(modifier.fillMaxSize().padding(bottom = TvDimens.Toast.Bottom), contentAlignment = Alignment.BottomCenter) {
        AnimatedVisibility(visible = text != null, enter = fadeIn(tween(TvMotion.FOCUS_MS)), exit = fadeOut(tween(TvMotion.FOCUS_MS))) {
            var last by remember { mutableStateOf("") }
            if (text != null) last = text
            Text(
                last,
                style = TvType.bodyStrong,
                color = SlouchcatTokens.Tv.Color.OnToast,
                modifier = Modifier
                    .widthIn(max = TvDimens.Toast.Max)
                    .background(SlouchcatTokens.Tv.Color.Toast, RoundedCornerShape(TvDimens.radiusToast))
                    .padding(horizontal = TvDimens.Toast.PadX, vertical = TvDimens.Toast.PadY)
                    .semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
    }
}

/** How the art layer lies behind the page: `hero` (full) or `dim` (darkened). */
enum class BackdropMode { Hero, Dim }

/**
 * `art(file, mode)`: an image behind the page that fades over to the next one, darkened towards the
 * left and the bottom so text stays readable. [model] is anything Coil loads (`null`: nothing).
 */
@Composable
fun TvBackdrop(model: Any?, modifier: Modifier = Modifier, mode: BackdropMode = BackdropMode.Hero) {
    val c = SlouchcatTheme.colors
    val scrim = SlouchcatTokens.Color.Scrim
    Box(modifier.fillMaxSize()) {
        Crossfade(targetState = model, animationSpec = tween(SlouchcatTokens.Motion.Duration.Crossfade)) { image ->
            if (image != null) AsyncImage(model = image, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to scrim.Solid, BACKDROP_MID to scrim.Medium, BACKDROP_CLEAR to scrim.Clear)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(BACKDROP_BOTTOM to scrim.Clear, 1f to c.bg)))
        if (mode == BackdropMode.Dim) Box(Modifier.fillMaxSize().background(c.bg.copy(alpha = TvMotion.dim)))
    }
}

/** Where the gradients of the art layer turn (the stops of `#art::after` in app.css). */
private const val BACKDROP_MID = 0.36f
private const val BACKDROP_CLEAR = 0.68f
private const val BACKDROP_BOTTOM = 0.58f

/** A row of [RowScope] content in the focus group of a menu: nothing more than a Row with gaps. */
@Composable
fun TvRow(modifier: Modifier = Modifier, content: @Composable RowScope.() -> Unit) =
    Row(modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically, content = content)
