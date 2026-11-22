package com.slouchcat.app.tv.search

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.Key as KeyCode
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.key.utf16CodePoint
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.ui.FocusMemory
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvIcons
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R

/*
 * The own keyboard of the TV (mockup `/search`, `KB_ROWS`): the field, the device's keyboard and
 * voice input next to it, eleven columns of keys. The search of stage 2b and the search of
 * «Wünsche» (stage 5) type with it.
 */

/** The letters of the keyboard (mockup `KB_ROWS`): digits, then QWERTZ with the signs of addresses and names. */
private val LETTERS = listOf("1234567890", "qwertzuiop-", "asdfghjkl._", "yxcvbnm@:/&")
private val SIGNS = listOf("1234567890", "!?#$%*+=()~", "[]{}<>'\";,|", "\\^`€§°äöüéè")

/** The focus key of the key the page starts on («q»). */
const val TYPING_START = "k-1-0"
private const val DEVICE_KEYBOARD = "kbd"
private const val DELETE = 0x7f

/** Whether the device's own input has the field (then a plugged-in keyboard types there, not through [hardwareTyping]). */
class TypingMode(initial: Boolean = false) {
    var system by mutableStateOf(initial)
}

/** A [TypingMode] kept with the page (the device's input stays open across a configuration change). */
@Composable
fun rememberTypingMode(): TypingMode =
    rememberSaveable(saver = Saver(save = { it.system }, restore = { TypingMode(it) })) { TypingMode() }

/**
 * A keyboard plugged into the TV: a printable sign types it (the D-pad and OK are no signs),
 * Backspace deletes (code review m-4 of stage 2b). Not while the device's input has the field.
 */
fun Modifier.hardwareTyping(mode: TypingMode, text: () -> String, onText: (String) -> Unit): Modifier = onPreviewKeyEvent { event ->
    if (mode.system || event.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
    if (event.key == KeyCode.Backspace) {
        onText(text().dropLast(1))
        return@onPreviewKeyEvent true
    }
    val code = event.utf16CodePoint
    if (code < ' '.code || code == DELETE) return@onPreviewKeyEvent false
    onText(text() + code.toChar())
    true
}

/**
 * The column of typing (`.br-kb` with its field): [title] above, the field with [placeholder], the
 * key «Tastatur des Geräts» and [voice] where the TV has it, the keys, [hint] below. Every key is
 * known to [focus] (`k-<row>-<column>`, start [TYPING_START]).
 */
@Composable
fun TvTyping(
    text: String,
    onText: (String) -> Unit,
    onClear: () -> Unit,
    focus: FocusMemory,
    mode: TypingMode,
    placeholder: String,
    modifier: Modifier = Modifier,
    title: (@Composable () -> Unit)? = null,
    voice: (() -> Unit)? = null,
    hint: String? = stringResource(R.string.tv_search_hint),
) {
    var signs by rememberSaveable { mutableStateOf(false) }
    val c = SlouchcatTheme.colors
    Column(modifier.width(TvDimens.Browse.SearchKeys)) {
        title?.invoke()
        Row(Modifier.padding(top = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
            if (mode.system) {
                SystemField(text, onText, Modifier.weight(1f)) {
                    mode.system = false
                    focus.focus(DEVICE_KEYBOARD)
                }
            } else {
                Field(text, placeholder, Modifier.weight(1f))
            }
            Key(
                stringResource(R.string.tv_search_deviceKeyboard),
                onClick = { mode.system = true },
                modifier = Modifier.width(TvDimens.Browse.SearchKey * 2).focusKey(focus, DEVICE_KEYBOARD),
                icon = TvIcons.Keyboard,
            )
            voice?.let { listen ->
                Key(stringResource(R.string.tv_search_voice), onClick = listen, modifier = Modifier.width(TvDimens.Browse.SearchKey * 2).focusKey(focus, "mic"), icon = SlouchcatIcon.Mic.vector)
            }
        }
        Keyboard(if (signs) SIGNS else LETTERS, signs, focus, onType = { onText(text + it) }, onDelete = { onText(text.dropLast(1)) }, onSigns = { signs = !signs }, onClear = onClear)
        if (hint != null) Text(hint, style = TvType.small, color = c.tx3, modifier = Modifier.padding(top = TvDimens.l))
    }
}

/** `.br-sf`: the field (no control: the keyboard types into it), the caret in the accent. */
@Composable
private fun Field(text: String, hint: String, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    Row(
        modifier
            .height(TvDimens.Field.Height)
            .border(BorderStroke(TvDimens.borderThick, c.tx), RoundedCornerShape(TvDimens.radiusField))
            .padding(horizontal = TvDimens.Field.Pad)
            .semantics(mergeDescendants = true) { contentDescription = text.ifEmpty { hint } },
        horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TvIcon(SlouchcatIcon.Search, TvDimens.Field.Icon, tint = c.tx2)
        Text(text.ifEmpty { hint }, style = TvType.field, color = if (text.isEmpty()) c.tx3 else c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
        Box(Modifier.width(TvDimens.borderThick * 2).height(TvDimens.Field.Icon).background(c.accent))
    }
}

/**
 * The field with the device's input: it takes the focus and opens the keyboard of the system;
 * every change searches at once; «Fertig» or Back closes it ([onClose]).
 */
@Composable
private fun SystemField(text: String, onText: (String) -> Unit, modifier: Modifier, onClose: () -> Unit) {
    val c = SlouchcatTheme.colors
    val requester = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    val label = stringResource(R.string.search_label)
    val close = {
        keyboard?.hide()
        onClose()
    }
    BackHandler(onBack = close)
    BasicTextField(
        value = text,
        onValueChange = onText,
        singleLine = true,
        textStyle = TvType.field.copy(color = c.tx),
        cursorBrush = SolidColor(c.accent),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search, autoCorrectEnabled = false),
        keyboardActions = KeyboardActions(onSearch = { close() }, onDone = { close() }),
        modifier = modifier
            .height(TvDimens.Field.Height)
            .border(BorderStroke(TvDimens.borderThick, c.accent), RoundedCornerShape(TvDimens.radiusField))
            .padding(horizontal = TvDimens.Field.Pad)
            .focusRequester(requester)
            .semantics { contentDescription = label },
        decorationBox = { inner -> Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.CenterStart) { inner() } },
    )
    LaunchedEffect(Unit) {
        withFrameNanos { }
        runCatching { requester.requestFocus() }.getOrDefault(Unit)
        keyboard?.show()
    }
}

/** `.br-kb`: eleven columns of keys, the last row with the signs, the space and «Leeren». */
@Composable
private fun Keyboard(rows: List<String>, signs: Boolean, focus: FocusMemory, onType: (String) -> Unit, onDelete: () -> Unit, onSigns: () -> Unit, onClear: () -> Unit) {
    val gap = TvDimens.s
    Column(Modifier.padding(top = TvDimens.l), verticalArrangement = Arrangement.spacedBy(gap)) {
        rows.forEachIndexed { r, keys ->
            Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
                keys.forEachIndexed { k, ch ->
                    Key(ch.toString(), onClick = { onType(ch.toString()) }, modifier = Modifier.weight(1f).focusKey(focus, "k-$r-$k"))
                }
                if (r == 0) Key(stringResource(R.string.tv_search_delete), onClick = onDelete, modifier = Modifier.weight(1f).focusKey(focus, "k-0-10"), icon = TvIcons.Backspace)
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
            Key(stringResource(if (signs) R.string.tv_search_letters else R.string.tv_search_symbols), onClick = onSigns, modifier = Modifier.weight(2f).focusKey(focus, "k-4-0"), small = true)
            Key(stringResource(R.string.tv_search_space), onClick = { onType(" ") }, modifier = Modifier.weight(6f).focusKey(focus, "k-4-1"), small = true)
            Key(stringResource(R.string.tv_search_clear), onClick = onClear, modifier = Modifier.weight(3f).focusKey(focus, "k-4-2"), small = true, icon = SlouchcatIcon.Close.vector, showText = true)
        }
    }
}

/** `.kb-k`: a key; with an [icon] only its description is said (and shown with [showText]). */
@Composable
private fun Key(text: String, onClick: () -> Unit, modifier: Modifier, icon: ImageVector? = null, small: Boolean = false, showText: Boolean = icon == null) {
    val c = SlouchcatTheme.colors
    Surface(
        onClick = onClick,
        modifier = modifier.height(TvDimens.Browse.SearchKey).semantics { contentDescription = text },
        shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(TvDimens.radiusMenuItem)),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = c.s2,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx))),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(Modifier.align(Alignment.Center), horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically) {
            if (icon != null) TvIcon(icon, TvDimens.Browse.Icon)
            if (showText) Text(text, style = if (small) TvType.bodyStrong else TvType.large.copy(fontWeight = FontWeight.SemiBold), maxLines = 1)
        }
    }
}
