package com.slouchcat.core.design

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** The five kinds of `.btn` in the mockup. */
enum class ButtonKind { Primary, Accent, Secondary, Ghost, Danger }

/**
 * `.btn`: a pill, at least 48 dp high (40 dp for [small], with a 48 dp tap target), label and an
 * optional icon after it. [block] fills the width. [busy] shows a spinner before the label and
 * keeps the button from a second tap.
 */
@Composable
fun SlouchcatButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: ButtonKind = ButtonKind.Primary,
    block: Boolean = false,
    small: Boolean = false,
    enabled: Boolean = true,
    busy: Boolean = false,
    leadingIcon: SlouchcatIcon? = null,
    trailingIcon: SlouchcatIcon? = null,
) {
    val c = SlouchcatTheme.colors
    val (container, content, border) = when (kind) {
        ButtonKind.Primary -> Triple(c.tx, c.bg, null)
        ButtonKind.Accent -> Triple(c.accent, c.onAccent, null)
        ButtonKind.Secondary -> Triple(c.s3, c.tx, c.ctl)
        ButtonKind.Ghost -> Triple(Color.Transparent, c.tx2, c.frame)
        ButtonKind.Danger -> Triple(Color.Transparent, c.danger, c.danger)
    }
    val height = if (small) 40.dp else SlouchcatDimens.touch
    Button(
        onClick = { if (!busy) onClick() },
        modifier = modifier
            .then(if (block) Modifier.fillMaxWidth() else Modifier)
            .defaultMinSize(minHeight = height),
        enabled = enabled,
        shape = CircleShape,
        colors = ButtonDefaults.buttonColors(
            containerColor = container,
            contentColor = content,
            disabledContainerColor = container.copy(alpha = container.alpha * DISABLED),
            disabledContentColor = content.copy(alpha = DISABLED),
        ),
        border = border?.let { BorderStroke(1.dp, if (enabled) it else it.copy(alpha = DISABLED)) },
        contentPadding = PaddingValues(horizontal = if (small) 16.dp else 22.dp),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            if (busy) {
                CircularProgressIndicator(modifier = Modifier.size(18.dp), color = content, strokeWidth = 2.5.dp)
            } else if (leadingIcon != null) {
                SlouchcatIconImage(leadingIcon, contentDescription = null, size = 18.dp)
            }
            Text(text, style = if (small) SlouchcatType.buttonSmall else SlouchcatType.button, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (trailingIcon != null && !busy) SlouchcatIconImage(trailingIcon, contentDescription = null, size = 18.dp)
        }
    }
}

/** `.ib`: a round 48 dp button with an icon; [contentDescription] is what a screen reader says. */
@Composable
fun SlouchcatIconButton(
    icon: SlouchcatIcon,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    tint: Color = SlouchcatTheme.colors.tx,
    enabled: Boolean = true,
) {
    IconButton(
        onClick = onClick,
        modifier = modifier.size(SlouchcatDimens.touch),
        enabled = enabled,
        colors = IconButtonDefaults.iconButtonColors(contentColor = tint),
    ) {
        SlouchcatIconImage(icon, contentDescription = contentDescription)
    }
}

/** One icon of [SlouchcatIcon]; `null` description for icons next to a text that says the same. */
@Composable
fun SlouchcatIconImage(
    icon: SlouchcatIcon,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    tint: Color = androidx.compose.material3.LocalContentColor.current,
) {
    Icon(icon.vector, contentDescription = contentDescription, modifier = modifier.size(size), tint = tint)
}

/** `.chip`: a filter or choice, 36 dp high with a 48 dp tap target; [selected] fills it. */
@Composable
fun SlouchcatChip(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: SlouchcatIcon? = null,
) {
    val c = SlouchcatTheme.colors
    androidx.compose.material3.FilterChip(
        selected = selected,
        onClick = onClick,
        label = { Text(text, style = SlouchcatType.chip) },
        modifier = modifier.defaultMinSize(minHeight = 36.dp),
        leadingIcon = icon?.let { { SlouchcatIconImage(it, contentDescription = null, size = 18.dp) } },
        shape = RoundedCornerShape(10.dp),
        colors = androidx.compose.material3.FilterChipDefaults.filterChipColors(
            containerColor = Color.Transparent,
            labelColor = c.tx2,
            iconColor = c.tx2,
            selectedContainerColor = c.tx,
            selectedLabelColor = c.bg,
            selectedLeadingIconColor = c.bg,
        ),
        border = androidx.compose.material3.FilterChipDefaults.filterChipBorder(
            enabled = true,
            selected = selected,
            borderColor = c.frame,
            selectedBorderColor = c.tx,
        ),
    )
}

private const val DISABLED = 0.45f
