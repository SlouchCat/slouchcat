package com.slouchcat.app.tv.ui

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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.core.design.SlouchcatTheme

/** The character a hidden password shows per letter (the mockup's `•`). */
private const val MASK = '•'

/**
 * `.tf` of the mockup, with the input of the system instead of the mockup's own keyboard (build
 * decision A-7: the keyboard of the device, with voice where it has one). The field is a button
 * that shows the value; OK opens the input, «Fertig» of the keyboard ([ImeAction.Done]) takes the
 * text, Back closes it without a change. [onDone] runs after the text was taken (the next step); it
 * answers whether it moved the focus itself, else the focus comes back to the field.
 *
 * The value changes only when the input is taken: [onValueChange] gets the whole new text once.
 */
@Composable
fun TvTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    placeholder: String,
    modifier: Modifier = Modifier,
    keyboardType: KeyboardType = KeyboardType.Text,
    password: Boolean = false,
    showPassword: Boolean = false,
    focusRequester: FocusRequester = remember { FocusRequester() },
    onDone: () -> Boolean = { false },
) {
    val c = SlouchcatTheme.colors
    var editing by rememberSaveable { mutableStateOf(false) }
    var draft by remember { mutableStateOf(value) }
    // The editor reports «not focused» before it got the focus; only a loss after that closes it.
    var editorFocused by remember { mutableStateOf(false) }
    // The field comes back after the input closed; it takes the focus once it is placed again.
    var refocus by remember { mutableStateOf(false) }
    val editor = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    val shape = RoundedCornerShape(TvDimens.radiusField)

    fun close(take: Boolean) {
        if (!editing) return
        editing = false
        editorFocused = false
        keyboard?.hide()
        if (take && draft != value) onValueChange(draft)
        refocus = !(take && onDone())
    }

    BackHandler(enabled = editing) { close(take = false) }
    Column(modifier.widthIn(max = TvDimens.Field.Max).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(TvDimens.Field.LabelGap)) {
        Text(label, style = TvType.label, color = c.tx2, modifier = Modifier.padding(start = TvDimens.xs / 2))
        if (editing) {
            BasicTextField(
                value = draft,
                onValueChange = { draft = it },
                singleLine = true,
                textStyle = TvType.field.copy(color = c.tx),
                cursorBrush = SolidColor(c.accent),
                visualTransformation = if (password && !showPassword) PasswordVisualTransformation(MASK) else VisualTransformation.None,
                keyboardOptions = KeyboardOptions(keyboardType = keyboardType, imeAction = ImeAction.Done, autoCorrectEnabled = false),
                keyboardActions = KeyboardActions(onDone = { close(take = true) }),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(TvDimens.Field.Height)
                    .background(c.s2, shape)
                    .border(BorderStroke(TvDimens.borderThick, c.tx), shape)
                    .padding(horizontal = TvDimens.Field.Pad)
                    .focusRequester(editor)
                    .semantics { contentDescription = label }
                    .onFocusChanged {
                        if (it.isFocused) {
                            editorFocused = true
                        } else if (editorFocused && editing) {
                            editorFocused = false
                            close(take = true)
                        }
                    },
                decorationBox = { inner -> Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.CenterStart) { inner() } },
            )
            LaunchedEffect(Unit) {
                draft = value
                withFrameNanos { }
                runCatching { editor.requestFocus() }
                keyboard?.show()
            }
        } else {
            LaunchedEffect(refocus) {
                if (!refocus) return@LaunchedEffect
                withFrameNanos { }
                runCatching { focusRequester.requestFocus() }
                refocus = false
            }
            Surface(
                onClick = {
                    draft = value
                    editing = true
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(TvDimens.Field.Height)
                    .focusRequester(focusRequester)
                    .semantics { contentDescription = label },
                shape = ClickableSurfaceDefaults.shape(shape = shape),
                colors = ClickableSurfaceDefaults.colors(
                    containerColor = c.s1,
                    contentColor = c.tx,
                    focusedContainerColor = c.s2,
                    focusedContentColor = c.tx,
                    pressedContainerColor = c.s2,
                    pressedContentColor = c.tx,
                ),
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
                border = ClickableSurfaceDefaults.border(
                    border = Border(BorderStroke(TvDimens.borderThick, c.ctl), shape = shape),
                    focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
                ),
            ) {
                Row(
                    Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = TvDimens.Field.Pad),
                    horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    val shown = if (password && !showPassword) MASK.toString().repeat(value.length) else value
                    Text(
                        shown.ifEmpty { placeholder },
                        style = TvType.field,
                        color = if (value.isEmpty()) c.tx3 else c.tx,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    TvIcon(TvIcons.Keyboard, TvDimens.Field.Icon, tint = c.tx3)
                }
            }
        }
    }
}
