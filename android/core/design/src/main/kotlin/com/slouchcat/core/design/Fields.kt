package com.slouchcat.core.design

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.background
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.error
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

/**
 * `.field` with `.inp`: a label above, the input (54 dp, rounded), and under it either the error
 * (red, with the alert icon) or the [hint]. The label is the field's name for a screen reader, the
 * error its error state.
 */
@Composable
fun SlouchcatTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    hint: String? = null,
    error: String? = null,
    placeholder: String? = null,
    readOnly: Boolean = false,
    enabled: Boolean = true,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    keyboardActions: KeyboardActions = KeyboardActions.Default,
    fieldModifier: Modifier = Modifier,
) {
    val c = SlouchcatTheme.colors
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val shape = RoundedCornerShape(SlouchcatDimens.radiusField)
    val borderColor = when {
        error != null -> c.danger
        focused && !readOnly -> c.tx
        else -> c.ctl
    }
    Column(modifier.fillMaxWidth()) {
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            readOnly = readOnly,
            enabled = enabled,
            singleLine = true,
            textStyle = SlouchcatType.body.copy(color = if (readOnly) c.tx2 else c.tx),
            cursorBrush = SolidColor(c.tx),
            keyboardOptions = keyboardOptions,
            keyboardActions = keyboardActions,
            interactionSource = interaction,
            modifier = fieldModifier
                .fillMaxWidth()
                .semantics { if (error != null) error(error) },
            // The label sits in the decoration, so it belongs to the field's node: a screen reader
            // says label and content together, and reads the content, not a description over it.
            decorationBox = { inner ->
                Column {
                Text(label, style = SlouchcatType.label, color = c.tx2, modifier = Modifier.padding(start = 2.dp, bottom = 6.dp))
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(min = SlouchcatDimens.fieldHeight)
                        .clip(shape)
                        .background(if (readOnly) c.bg else c.s1)
                        .border(BorderStroke(if (focused && !readOnly) 2.dp else 1.dp, borderColor), shape)
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (value.isEmpty() && placeholder != null) {
                        Text(placeholder, style = SlouchcatType.body, color = c.tx3)
                    }
                    inner()
                }
                }
            },
        )
        if (error != null) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.padding(start = 2.dp, top = 8.dp),
            ) {
                SlouchcatIconImage(SlouchcatIcon.Alert, contentDescription = null, size = 16.dp, tint = c.danger, modifier = Modifier.padding(top = 2.dp))
                Text(error, style = SlouchcatType.label.copy(fontSize = SlouchcatType.bodySmall.fontSize), color = c.danger)
            }
        } else if (hint != null) {
            // As drawn in the mockup: `.field > span` gives the hint the weight and colour of the label.
            Text(hint, style = SlouchcatType.label, color = c.tx2, modifier = Modifier.padding(start = 2.dp, top = 8.dp))
        }
    }
}
