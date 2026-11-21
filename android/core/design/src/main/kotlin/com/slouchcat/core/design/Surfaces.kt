package com.slouchcat.core.design

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.intl.Locale
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.toUpperCase
import androidx.compose.ui.unit.dp

/** How the leading icon of a bar is used. */
enum class BarNavigation { None, Back, Menu }

/**
 * `bar(...)`: the bar at the top of a page, 60 dp, with the back button (or the menu button of
 * the media choice), the title as heading and [actions] at the end.
 */
@Composable
fun TopBar(
    title: String,
    modifier: Modifier = Modifier,
    navigation: BarNavigation = BarNavigation.Back,
    navigationLabel: String? = null,
    onNavigation: () -> Unit = {},
    actions: @Composable RowScope.() -> Unit = {},
) {
    val c = SlouchcatTheme.colors
    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = SlouchcatDimens.barHeight)
            .background(c.bg)
            .padding(start = 4.dp, end = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        if (navigation != BarNavigation.None) {
            SlouchcatIconButton(
                icon = if (navigation == BarNavigation.Back) SlouchcatIcon.Back else SlouchcatIcon.More,
                contentDescription = requireNotNull(navigationLabel) { "a bar with navigation needs its label" },
                onClick = onNavigation,
            )
        }
        Text(
            title,
            style = SlouchcatType.barTitle,
            color = c.tx,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier
                .weight(1f)
                .padding(start = if (navigation == BarNavigation.None) 12.dp else 2.dp)
                .semantics { heading() },
        )
        actions()
    }
}

/**
 * `.sec`: the title of a section, in capitals as the mockup draws it. [inset]: the page's side
 * margin; off inside a column that has its margin already.
 */
@Composable
fun SectionTitle(text: String, modifier: Modifier = Modifier, inset: Boolean = true) {
    Text(
        // Capitals of the user's language (Turkish i), as CSS text-transform does.
        text.toUpperCase(Locale.current),
        style = SlouchcatType.section,
        color = SlouchcatTheme.colors.tx3,
        modifier = modifier
            .padding(start = if (inset) gutter() else 0.dp, end = if (inset) gutter() else 0.dp, top = 18.dp, bottom = 8.dp)
            .semantics { heading() },
    )
}

/** `.card`: a framed surface for a group of content. */
@Composable
fun SlouchcatCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Column(
        modifier = modifier
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, c.line), shape)
            .padding(16.dp),
        content = content,
    )
}

/**
 * `.note` (and `.note.warn`): a framed notice with an icon; [warning] draws it in the warning
 * colour. [title]: a first line in the text colour above [text] (`<b>…</b><br>…` of the mockup).
 */
@Composable
fun Note(text: String, modifier: Modifier = Modifier, warning: Boolean = false, icon: SlouchcatIcon = if (warning) SlouchcatIcon.Alert else SlouchcatIcon.Info, title: String? = null) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(SlouchcatDimens.radius)
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(shape)
            .background(c.s1)
            .border(BorderStroke(1.dp, if (warning) c.warn else c.frame), shape)
            .padding(14.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SlouchcatIconImage(icon, contentDescription = null, size = 20.dp, tint = if (warning) c.warn else c.tx2, modifier = Modifier.padding(top = 1.dp))
        val style = SlouchcatType.bodySmall.copy(fontSize = SlouchcatType.chip.fontSize)
        if (title == null) {
            Text(text, style = style, color = c.tx2)
        } else {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = style.copy(fontWeight = FontWeight.SemiBold), color = c.tx)
                Text(text, style = style, color = c.tx2)
            }
        }
    }
}

/** The tones of `.badge`. */
enum class BadgeTone { Neutral, Ok, Accent }

/** `.badge`: a small framed label, for example «Gefunden». */
@Composable
fun Badge(text: String, modifier: Modifier = Modifier, tone: BadgeTone = BadgeTone.Neutral, icon: SlouchcatIcon? = null) {
    val c = SlouchcatTheme.colors
    val color = when (tone) {
        BadgeTone.Neutral -> c.tx2
        BadgeTone.Ok -> c.ok
        BadgeTone.Accent -> c.accent
    }
    val shape = RoundedCornerShape(6.dp)
    Row(
        modifier = modifier
            .heightIn(min = 22.dp)
            .border(BorderStroke(1.dp, if (tone == BadgeTone.Neutral) c.frame else color), shape)
            .padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (icon != null) SlouchcatIconImage(icon, contentDescription = null, size = 14.dp, tint = color)
        Text(text, style = SlouchcatType.badge, color = color, maxLines = 1)
    }
}

/** `.empty`: what a page shows when it has nothing, centred, with an icon, a title and a line. */
@Composable
fun EmptyState(title: String, modifier: Modifier = Modifier, body: String? = null, icon: SlouchcatIcon = SlouchcatIcon.Info) {
    val c = SlouchcatTheme.colors
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 32.dp, vertical = 48.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        SlouchcatIconImage(icon, contentDescription = null, size = 32.dp, tint = c.tx3, modifier = Modifier.padding(bottom = 12.dp))
        Text(title, style = SlouchcatType.strong.copy(fontSize = SlouchcatType.barTitle.fontSize), color = c.tx, textAlign = TextAlign.Center)
        if (body != null) {
            Text(body, style = SlouchcatType.bodySmall, color = c.tx3, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 4.dp))
        }
    }
}

/**
 * `li(...)`: a row of a list, at least 60 dp, with a leading icon on a small surface, title, an
 * optional second line, and an end (a value, or the arrow when [onClick] leads somewhere).
 */
@Composable
fun ListRow(
    title: String,
    modifier: Modifier = Modifier,
    icon: SlouchcatIcon? = null,
    subtitle: String? = null,
    onClick: (() -> Unit)? = null,
    end: @Composable (() -> Unit)? = if (onClick != null) {
        { SlouchcatIconImage(SlouchcatIcon.Right, contentDescription = null, size = 18.dp, tint = SlouchcatTheme.colors.tx3) }
    } else {
        null
    },
) {
    val c = SlouchcatTheme.colors
    Row(
        modifier = modifier
            .fillMaxWidth()
            .defaultMinSize(minHeight = SlouchcatDimens.rowHeight)
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .padding(horizontal = gutter(), vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        if (icon != null) {
            Box(
                Modifier
                    .size(40.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(c.s2),
                contentAlignment = Alignment.Center,
            ) {
                SlouchcatIconImage(icon, contentDescription = null, size = 22.dp, tint = c.tx2)
            }
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 3, overflow = TextOverflow.Ellipsis)
            if (subtitle != null) {
                Text(subtitle, style = SlouchcatType.caption, color = c.tx3, maxLines = 3, overflow = TextOverflow.Ellipsis)
            }
        }
        if (end != null) {
            CompositionLocalProvider(LocalContentColor provides c.tx3) { end() }
        }
    }
}

/** `sw(...)`: a row with a switch; the whole row toggles and reads as one switch. */
@Composable
fun SwitchRow(
    title: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    enabled: Boolean = true,
) {
    val c = SlouchcatTheme.colors
    Row(
        modifier = modifier
            .fillMaxWidth()
            .defaultMinSize(minHeight = SlouchcatDimens.rowHeight)
            .toggleable(value = checked, enabled = enabled, role = Role.Switch, onValueChange = onCheckedChange)
            .padding(horizontal = gutter(), vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, style = SlouchcatType.rowTitle, color = c.tx)
            if (subtitle != null) Text(subtitle, style = SlouchcatType.caption, color = c.tx3)
        }
        SwitchTrack(checked)
    }
}

/** The switch of the mockup (`.sw`): 52 x 32, outlined when off, accent when on. */
@Composable
private fun SwitchTrack(checked: Boolean) {
    val c = SlouchcatTheme.colors
    Box(
        modifier = Modifier
            .width(52.dp)
            .height(32.dp)
            .clip(CircleShape)
            .background(if (checked) c.accent else Color.Transparent)
            .border(BorderStroke(2.dp, if (checked) c.accent else c.ctl), CircleShape)
            .padding(horizontal = if (checked) 4.dp else 6.dp),
        contentAlignment = if (checked) Alignment.CenterEnd else Alignment.CenterStart,
    ) {
        Box(
            Modifier
                .size(if (checked) 22.dp else 16.dp)
                .clip(CircleShape)
                .background(if (checked) c.onAccent else c.tx3),
        )
    }
}
