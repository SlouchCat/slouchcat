package com.slouchcat.app.tv.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.material3.Icon
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens

/** The look of a TV button (`.tb` of the mockup). */
enum class TvButtonKind {
    /** At rest glass; the standard. */
    Standard,

    /** `.tb.primary`: the accent; the main action of a page. */
    Primary,

    /** `.tb.ghost`: transparent with a frame; the second way. */
    Ghost,
}

/** An icon of the TV app: one of the mockup's (`SlouchcatIcon`) or one of [TvIcons]. */
@Composable
fun TvIcon(vector: ImageVector, size: Dp, modifier: Modifier = Modifier, tint: Color = LocalContentColor.current) {
    Icon(vector, contentDescription = null, modifier = modifier.size(size), tint = tint)
}

@Composable
fun TvIcon(icon: SlouchcatIcon, size: Dp, modifier: Modifier = Modifier, tint: Color = LocalContentColor.current) =
    TvIcon(icon.vector, size, modifier, tint)

/**
 * `.tb`: a pill of 80 px (40 dp) with icon and text. In focus it turns light with dark text and
 * grows by `--focus-scale`: the focus is always visible. [enabled] `false` is `aria-disabled`:
 * visible, dimmed, not focusable and without effect.
 */
@Composable
fun TvButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    kind: TvButtonKind = TvButtonKind.Standard,
    small: Boolean = false,
    enabled: Boolean = true,
    trailing: String? = null,
) {
    val c = SlouchcatTheme.colors
    val shape = CircleShape
    val rest = when (kind) {
        TvButtonKind.Standard -> SlouchcatTokens.Tv.Color.Glass
        TvButtonKind.Primary -> c.accent
        TvButtonKind.Ghost -> Color.Transparent
    }
    val restText = when (kind) {
        TvButtonKind.Primary -> c.onAccent
        TvButtonKind.Ghost -> c.tx2
        TvButtonKind.Standard -> c.tx
    }
    val frame = if (kind == TvButtonKind.Ghost) c.frame else Color.Transparent
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .focusProperties { canFocus = enabled }
            .height(if (small) TvDimens.Button.SmallHeight else TvDimens.Button.Height)
            .alpha(if (enabled) 1f else TvMotion.disabled)
            .semantics { if (!enabled) disabled() },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = rest,
            contentColor = restText,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = rest,
            disabledContentColor = restText,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, frame), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
    ) {
        Row(
            Modifier.align(Alignment.Center).padding(horizontal = if (small) TvDimens.Button.SmallPad else TvDimens.Button.Pad),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.Button.Gap),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (icon != null) TvIcon(icon, TvDimens.Button.Icon)
            Text(text, style = if (small) TvType.buttonSmall else TvType.button, maxLines = 1)
            if (trailing != null) Text(trailing, style = if (small) TvType.buttonSmall else TvType.button, maxLines = 1)
        }
    }
}

/**
 * A row of a list or menu (`.mi`): icon in a box, title, second line, chevron. In focus it turns
 * light. [selected] marks the current choice (`aria-current`).
 */
@Composable
fun TvMenuItem(
    title: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    subtitle: String? = null,
    chevron: Boolean = true,
    enabled: Boolean = true,
    /** The current choice of a list (`role=menuitemradio`, `aria-checked`): a check mark instead of the chevron. */
    selected: Boolean? = null,
) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusMenuItem)
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .focusProperties { canFocus = enabled }
            .fillMaxWidth()
            .heightIn(min = TvDimens.Menu.Item)
            .alpha(if (enabled) 1f else TvMotion.disabled)
            .semantics { if (selected != null) this.selected = selected },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
            disabledContainerColor = Color.Transparent,
            disabledContentColor = c.tx,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
    ) {
        val focusedText = LocalContentColor.current
        val focused = focusedText == c.bg
        Row(
            Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = TvDimens.Menu.ItemPad, vertical = TvDimens.xs),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (icon != null) {
                Box(
                    Modifier.size(TvDimens.Menu.IconBox).clip(RoundedCornerShape(TvDimens.radiusIconBox)).background(if (focused) c.bg else c.s3),
                    contentAlignment = Alignment.Center,
                ) { TvIcon(icon, TvDimens.Menu.Icon, tint = c.tx) }
            }
            Column(Modifier.weight(1f)) {
                Text(title, style = TvType.largeStrong, maxLines = 2, overflow = TextOverflow.Ellipsis)
                if (subtitle != null) Text(subtitle, style = TvType.small, color = if (focused) c.bg else c.tx3, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            if (selected == true) {
                TvIcon(SlouchcatIcon.Check, TvDimens.Menu.Icon, tint = if (focused) c.bg else c.accent)
            } else if (chevron && selected == null) {
                TvIcon(SlouchcatIcon.Right, TvDimens.Button.Icon, tint = if (focused) c.bg else c.tx3)
            }
        }
    }
}

/**
 * `.tsw`: a row with text and a switch. OK toggles it; the state is told as a switch to a screen
 * reader.
 */
@Composable
fun TvSwitch(text: String, checked: Boolean, onCheckedChange: (Boolean) -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Surface(
        onClick = { onCheckedChange(!checked) },
        modifier = modifier
            .widthIn(max = TvDimens.Note.Max)
            .fillMaxWidth()
            .heightIn(min = TvDimens.Switch.Height)
            .semantics(mergeDescendants = true) {
                role = Role.Switch
                toggleableState = ToggleableState(checked)
            },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = c.s1,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, c.line), shape = shape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape),
        ),
    ) {
        Row(
            Modifier.fillMaxWidth().align(Alignment.CenterStart).padding(horizontal = TvDimens.Switch.Pad, vertical = TvDimens.s),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.Switch.Pad),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(text, style = TvType.body, modifier = Modifier.weight(1f))
            val track = RoundedCornerShape(TvDimens.Switch.TrackHeight / 2)
            Box(
                Modifier
                    .size(TvDimens.Switch.TrackWidth, TvDimens.Switch.TrackHeight)
                    .clip(track)
                    .background(if (checked) c.accent else Color.Transparent)
                    .border(BorderStroke(TvDimens.borderThick, if (checked) c.accent else c.ctl), track)
                    .padding(horizontal = TvDimens.Switch.ThumbInset),
                contentAlignment = if (checked) Alignment.CenterEnd else Alignment.CenterStart,
            ) {
                Box(Modifier.size(TvDimens.Switch.Thumb).clip(CircleShape).background(if (checked) c.onAccent else c.tx3))
            }
        }
    }
}

/** The tone of a [TvNote] (`.tnote`, `.tnote.warn`, `.tnote.bad`). */
enum class NoteTone { Plain, Warn, Bad }

/** `.tnote`: a framed note with an icon; not focusable (it is read, not used). */
@Composable
fun TvNote(text: String, modifier: Modifier = Modifier, tone: NoteTone = NoteTone.Plain, icon: ImageVector = SlouchcatIcon.Alert.vector, small: Boolean = false) {
    val c = SlouchcatTheme.colors
    val frame = when (tone) {
        NoteTone.Plain -> c.frame
        NoteTone.Warn -> c.warn
        NoteTone.Bad -> c.danger
    }
    val shape = RoundedCornerShape(TvDimens.radiusField)
    Row(
        modifier
            .widthIn(max = TvDimens.Note.Max)
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(TvDimens.borderThick, frame), shape)
            .padding(horizontal = if (small) TvDimens.l else TvDimens.Note.PadX, vertical = if (small) TvDimens.m else TvDimens.Note.PadY)
            .semantics(mergeDescendants = true) {},
        horizontalArrangement = Arrangement.spacedBy(if (small) TvDimens.m else TvDimens.l),
    ) {
        TvIcon(icon, if (small) TvDimens.Badge.Icon else TvDimens.Note.Icon, tint = if (tone == NoteTone.Plain) c.tx2 else frame)
        Text(text, style = if (small) TvType.small else TvType.body, color = c.tx2, modifier = Modifier.weight(1f))
    }
}

/** The tone of a [TvBadge] (`.badge`, `.badge.acc`, `.badge.ok`, `.badge.warn`). */
enum class BadgeTone { Plain, Accent, Ok, Warn }

/** `.badge`: a small framed label, optionally with an icon or the accent dot. */
@Composable
fun TvBadge(text: String, modifier: Modifier = Modifier, tone: BadgeTone = BadgeTone.Plain, icon: ImageVector? = null) {
    val c = SlouchcatTheme.colors
    val frame = when (tone) {
        BadgeTone.Plain -> c.frame
        BadgeTone.Accent -> c.accent
        BadgeTone.Ok -> c.ok
        BadgeTone.Warn -> c.warn
    }
    val shape = RoundedCornerShape(TvDimens.radiusBadge)
    Row(
        modifier
            .height(TvDimens.Badge.Height)
            .border(BorderStroke(TvDimens.borderThin, frame), shape)
            .padding(horizontal = TvDimens.Badge.Pad),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (tone == BadgeTone.Accent) Box(Modifier.size(TvDimens.Badge.Dot).clip(CircleShape).background(c.accent))
        if (icon != null) TvIcon(icon, TvDimens.Badge.Icon, tint = frame)
        Text(text, style = TvType.badge, color = if (tone == BadgeTone.Plain) c.tx2 else c.tx, maxLines = 1)
    }
}

/** The connection of a server as a badge: «verschlüsselt» with a lock or «nicht verschlüsselt». */
@Composable
fun ConnectionBadge(encrypted: Boolean, encryptedText: String, plainText: String, modifier: Modifier = Modifier) {
    if (encrypted) {
        TvBadge(encryptedText, modifier, BadgeTone.Ok, SlouchcatIcon.Lock.vector)
    } else {
        TvBadge(plainText, modifier, BadgeTone.Warn, TvIcons.Unlock)
    }
}

/** `wordmark()`: «Slouch» in text colour, «cat» in the accent. [name] is the app's name (`app.name`). */
@Composable
fun TvWordmark(name: String, modifier: Modifier = Modifier, small: Boolean = false) {
    val c = SlouchcatTheme.colors
    val split = if (name.endsWith("cat")) name.length - 3 else name.length
    Text(
        buildAnnotatedString {
            withStyle(SpanStyle(color = c.tx)) { append(name.substring(0, split)) }
            withStyle(SpanStyle(color = c.accent)) { append(name.substring(split)) }
        },
        style = if (small) TvType.railWordmark else TvType.wordmark,
        modifier = modifier,
        maxLines = 1,
    )
}

/** A key–value list (`dl.md-dl`, `.su-dl`): the key muted, the value strong, a line between rows. */
@Composable
fun TvDetailRows(rows: List<Pair<String, @Composable RowScope.() -> Unit>>, keyWidth: Dp, modifier: Modifier = Modifier, rowHeight: Dp = TvDimens.Settings.Row) {
    val c = SlouchcatTheme.colors
    Column(modifier) {
        rows.forEachIndexed { index, (key, value) ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = rowHeight)
                    .semantics(mergeDescendants = true) {},
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(key, style = TvType.body, color = c.tx3, modifier = Modifier.widthIn(min = keyWidth, max = keyWidth))
                Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(TvDimens.s), verticalAlignment = Alignment.CenterVertically, content = value)
            }
            if (index < rows.lastIndex) Box(Modifier.fillMaxWidth().height(TvDimens.borderThin).background(c.line))
        }
    }
}

/** The text of a value in [TvDetailRows]. */
@Composable
fun DetailValue(text: String) = Text(text, style = TvType.bodyStrong, color = SlouchcatTheme.colors.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)

/** A spinner (`.spin`), told as busy to a screen reader through [label]. */
@Composable
fun TvBusy(label: String, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Row(modifier.semantics(mergeDescendants = true) { stateDescription = label }, horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
        androidx.compose.material3.CircularProgressIndicator(color = c.tx, trackColor = c.ctl, modifier = Modifier.size(TvDimens.Menu.Icon))
        Text(label, style = TvType.body, color = c.tx2)
    }
}
