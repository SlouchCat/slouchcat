package com.slouchcat.app.tv.browse

import androidx.annotation.PluralsRes
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.layout.layout
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.zIndex
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.ui.CardInfo
import com.slouchcat.app.tv.ui.CardShape
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvCard
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R
import java.text.NumberFormat
import com.slouchcat.core.media.CardShape as MediaShape

/*
 * The building blocks the browsing pages of stage 2b share (mockup `card`, `.br-head`, `.br-cb`,
 * `.br-ib`, `.br-empty`, `.sp-meta`): a card of a title, the texts of a title, the head of a grid
 * page, a filter chip, a round icon button with its label in focus, the empty state.
 */

/** The TV card form of a title (2:3, square for audio, 16:9 when the page asks for it). */
fun MediaItem.tvShape(): CardShape = when (defaultShape) {
    MediaShape.Poster -> CardShape.Poster
    MediaShape.Square -> CardShape.Square
    MediaShape.Wide -> CardShape.Wide
}

/** The title a card shows: the series of an episode («Weiterschauen» shows the series). */
val MediaItem.cardTitle: String get() = seriesTitle?.takeIf { kind == ItemKind.EPISODE } ?: title

/**
 * A card of a title (mockup `card(i, opts)`): the poster (or the backdrop for [CardShape.Wide]) in
 * the size the TV draws, the title as what a screen reader says. [onClick] `null` (the page is not
 * registered): the card stays focusable for browsing and does nothing.
 */
@Composable
fun MediaCard(
    item: MediaItem,
    onClick: (() -> Unit)?,
    modifier: Modifier = Modifier,
    shape: CardShape = item.tvShape(),
    caption: String? = null,
    subcaption: String? = null,
    tag: String? = null,
    progress: Float? = null,
    /** `null`: as wide as its cell (a grid). */
    width: Dp? = shape.width,
) {
    val image = if (shape == CardShape.Wide) item.backdrop.or(item.poster).forWidth(shape.width) else item.poster.forWidth(width ?: shape.width)
    val said = listOfNotNull(item.cardTitle, caption?.takeIf { it != item.cardTitle }, subcaption, tag).joinToString(stringResource(R.string.format_listSeparator))
    TvCard(
        title = said,
        image = image,
        onClick = onClick ?: {},
        modifier = modifier,
        shape = shape,
        caption = caption,
        subcaption = subcaption,
        tag = tag,
        progress = progress,
        width = width,
    )
}

/** The name of a kind of title («Film», «Serie»); `null` for a kind the app does not know. */
@Composable
fun kindName(kind: ItemKind): String? = when (kind) {
    ItemKind.MOVIE -> stringResource(R.string.kind_movie)
    ItemKind.SERIES -> stringResource(R.string.kind_series)
    ItemKind.SEASON -> stringResource(R.string.kind_season)
    ItemKind.EPISODE -> stringResource(R.string.kind_episode)
    ItemKind.EXTRA -> stringResource(R.string.kind_extra)
    ItemKind.AUDIOBOOK -> stringResource(R.string.kind_audiobook)
    ItemKind.PODCAST -> stringResource(R.string.kind_podcast)
    ItemKind.PODCAST_EPISODE -> stringResource(R.string.kind_podcast_episode)
    ItemKind.UNKNOWN_DEFAULT_OPEN_API -> null
}

/** «2 h 25 min», «48 min». */
@Composable
fun runtimeText(minutes: Int): String =
    if (minutes >= MINUTES_PER_HOUR) {
        stringResource(R.string.format_hoursMinutes, (minutes / MINUTES_PER_HOUR).toString(), (minutes % MINUTES_PER_HOUR).toString())
    } else {
        stringResource(R.string.format_minutes, minutes.toString())
    }

/** «S1 · F3» of an episode, `null` without numbers. */
@Composable
fun episodeCode(item: MediaItem): String? {
    val season = item.seasonNumber ?: return null
    val episode = item.episodeNumber ?: return null
    val end = item.episodeEnd
    return if (end != null && end > episode) {
        stringResource(R.string.home_continue_episodeRange, season.toString(), episode.toString(), end.toString())
    } else {
        stringResource(R.string.home_continue_episode, season.toString(), episode.toString())
    }
}

/**
 * A plural with the number in the separators of the language («1'846 Titel»): `{{count}}` is
 * `%1$d` in the generated resources, formatted here as text.
 */
@Composable
fun pluralCount(@PluralsRes id: Int, count: Int, vararg more: Any): String {
    val resources = LocalResources.current
    val locale = resources.configuration.locales[0]
    val template = resources.getQuantityText(id, count).toString().replace("%1\$d", "%1\$s")
    return String.format(locale, template, NumberFormat.getIntegerInstance(locale).format(count), *more)
}

/** The quiet part of the line under a row (`.rinfo`): year, kind or length, «noch …». */
@Composable
fun MediaItem.meta(): String? {
    val parts = buildList {
        if (kind == ItemKind.EPISODE) episodeCode(this@meta)?.let(::add)
        year?.let { add(it.toString()) }
        if (kind == ItemKind.SERIES) kindName(kind)?.let(::add) else runtimeMinutes?.takeIf { it > 0 }?.let { add(runtimeText(it)) }
        remainingMinutes?.let { add(stringResource(R.string.home_remaining, runtimeText(it))) }
    }
    return parts.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
}

/** What the line under a row says about this card. */
@Composable
fun MediaItem.info(): CardInfo = CardInfo(if (kind == ItemKind.EPISODE) "$cardTitle · $title" else title, meta())

/** The tag «Gesehen» of a finished title on a card. */
@Composable
fun watchedTag(item: MediaItem): String? =
    if (item.watched) stringResource(if (item.isAudio) R.string.library_status_heard else R.string.library_status_watched) else null

/** `fsk(n)`: the minimum age in a thin frame («12»), said as «ab 12». */
@Composable
fun RatingMark(age: Int, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val said = stringResource(R.string.rating_age, age.toString())
    Text(
        age.toString(),
        style = TvType.bodyStrong,
        color = c.tx,
        modifier = modifier
            .border(BorderStroke(TvDimens.borderThick, c.tx2), RoundedCornerShape(TvDimens.radiusBadge))
            .padding(horizontal = TvDimens.s)
            .semantics { contentDescription = said },
    )
}

/** `.sp-meta`: parts of a line with a small dot between them; [rating] goes second, as a frame. */
@Composable
fun MetaLine(parts: List<String>, modifier: Modifier = Modifier, rating: Int? = null) {
    val c = SlouchcatTheme.colors
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(TvDimens.Browse.MetaGap), verticalAlignment = Alignment.CenterVertically) {
        val all: List<Any> = buildList {
            parts.firstOrNull()?.let(::add)
            rating?.let(::add)
            addAll(parts.drop(1))
        }
        all.forEachIndexed { index, part ->
            if (index > 0) Box(Modifier.size(TvDimens.xs).clip(CircleShape).background(c.tx3))
            if (part is Int) RatingMark(part) else Text(part as String, style = TvType.body, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** `.sp-kind`: the small spaced capitals over a title («FILM»). */
@Composable
fun KindLine(text: String, modifier: Modifier = Modifier) =
    Text(text.uppercase(), style = TvType.section.copy(fontSize = TvType.small.fontSize), color = SlouchcatTheme.colors.tx2, modifier = modifier)

/** `.br-head`: the head of a grid page, the icon in a box, the title, a count. */
@Composable
fun PageHead(icon: ImageVector, title: String, count: String?, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Row(modifier.heightIn(min = TvDimens.Browse.HeadHeight), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(TvDimens.Browse.HeadIcon).clip(RoundedCornerShape(TvDimens.radiusIconBox)).background(c.s2), contentAlignment = Alignment.Center) {
            TvIcon(icon, TvDimens.Browse.Icon, tint = c.tx2)
        }
        Text(title, style = TvType.h1, modifier = Modifier.semantics { heading() }, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (count != null) Text(count, style = TvType.large, color = c.tx3, maxLines = 1)
    }
}

/**
 * `.br-cb`: a chip of the controls of a grid page (filter, sort, «Bearbeiten»). [on]: a filter in
 * use (a frame of the accent); [menu]: opens a side menu (a chevron down).
 */
@Composable
fun ControlChip(
    text: String,
    icon: ImageVector,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    on: Boolean = false,
    menu: Boolean = false,
    ghost: Boolean = false,
) {
    val c = SlouchcatTheme.colors
    val frame = when {
        on -> c.accent
        ghost -> c.frame
        else -> Color.Transparent
    }
    Surface(
        onClick = onClick,
        modifier = modifier.height(TvDimens.Browse.ControlHeight).semantics { if (on) selected = true },
        shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = if (ghost) Color.Transparent else com.slouchcat.core.design.SlouchcatTokens.Tv.Color.Glass,
            contentColor = if (on) c.tx else c.tx2,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(
            border = Border(BorderStroke(TvDimens.borderThick, frame), shape = CircleShape),
            focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = CircleShape),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
    ) {
        Row(
            Modifier.align(Alignment.Center).padding(horizontal = TvDimens.Browse.ControlPad),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TvIcon(icon, TvDimens.Browse.Icon)
            Text(text, style = TvType.bodyStrong, maxLines = 1)
            if (menu) TvIcon(SlouchcatIcon.Down, TvDimens.Button.Icon)
        }
    }
}

/**
 * `.br-ib`: a round button with only an icon; in focus its [label] shows below it (`.br-ibl`).
 * [pressed]: a state that is on («Gemerkt»), a frame and icon of the accent. [label] is also what a
 * screen reader says; [state] the second part («an», «aus»).
 */
@Composable
fun IconAction(
    icon: ImageVector,
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    pressed: Boolean = false,
    enabled: Boolean = true,
) {
    val c = SlouchcatTheme.colors
    var focused by remember { mutableStateOf(false) }
    val size = TvDimens.Browse.IconButton
    Box(modifier.zIndex(if (focused) 1f else 0f)) {
        Surface(
            onClick = onClick,
            enabled = enabled,
            modifier = Modifier
                .size(size)
                .onFocusChanged { focused = it.isFocused }
                .semantics {
                    contentDescription = label
                    if (pressed) stateDescription = label
                },
            shape = ClickableSurfaceDefaults.shape(shape = CircleShape),
            colors = ClickableSurfaceDefaults.colors(
                containerColor = com.slouchcat.core.design.SlouchcatTokens.Tv.Color.Glass,
                contentColor = if (pressed) c.accent else c.tx,
                focusedContainerColor = c.tx,
                focusedContentColor = c.bg,
                pressedContainerColor = c.tx,
                pressedContentColor = c.bg,
            ),
            border = ClickableSurfaceDefaults.border(
                border = Border(BorderStroke(TvDimens.borderThick, if (pressed) c.accent else Color.Transparent), shape = CircleShape),
                focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = CircleShape),
            ),
            scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
        ) {
            Box(Modifier.align(Alignment.Center)) { TvIcon(icon, TvDimens.Browse.Icon) }
        }
        Text(
            label,
            style = TvType.small.copy(fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold),
            color = c.tx,
            maxLines = 1,
            softWrap = false,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .belowWithoutSize(size + TvDimens.s)
                .clearAndSetSemantics { }
                .alpha(if (focused) 1f else 0f),
        )
    }
}

/**
 * Places the label [gap] below the top of its box, centred, at its own width, and takes no room:
 * a label wider than its round button must not widen the row of buttons.
 */
private fun Modifier.belowWithoutSize(gap: Dp): Modifier = layout { measurable, constraints ->
    val placeable = measurable.measure(constraints.copy(minWidth = 0, maxWidth = Constraints.Infinity))
    layout(0, 0) { placeable.place(-placeable.width / 2, gap.roundToPx()) }
}

/**
 * `.br-empty`: a page or grid with nothing to show: a large icon in a circle, a title, a text and
 * [action] (a primary button, focusable: it gives the page a focus).
 */
@Composable
fun EmptyState(
    icon: ImageVector,
    title: String,
    text: String?,
    modifier: Modifier = Modifier,
    action: (@Composable () -> Unit)? = null,
) {
    val c = SlouchcatTheme.colors
    Column(modifier.padding(vertical = TvDimens.xl), verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
        Box(Modifier.size(TvDimens.Browse.HeadIcon * 2).clip(CircleShape).background(c.s2), contentAlignment = Alignment.Center) {
            TvIcon(icon, TvDimens.Browse.HeadIcon, tint = c.tx3)
        }
        Text(title, style = TvType.h1, modifier = Modifier.semantics { heading() })
        if (text != null) Text(text, style = TvType.large, color = c.tx2, modifier = Modifier.widthIn(max = TvDimens.Browse.Overview))
        if (action != null) {
            Spacer(Modifier.height(TvDimens.s))
            action()
        }
    }
}

/** The primary button of an [EmptyState]. */
@Composable
fun EmptyAction(text: String, icon: ImageVector, onClick: () -> Unit, modifier: Modifier = Modifier) =
    TvButton(text, onClick = onClick, icon = icon, kind = TvButtonKind.Primary, modifier = modifier)

/** A progress bar (`.br-pbar`): [share] 0..1 in the accent on glass. */
@Composable
fun ProgressBar(share: Float, width: Dp, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(
        modifier
            .width(width)
            .height(TvDimens.Card.Progress)
            .clip(CircleShape)
            .background(com.slouchcat.core.design.SlouchcatTokens.Tv.Color.GlassStrong),
    ) {
        Box(Modifier.fillMaxWidth(share.coerceIn(0f, 1f)).height(TvDimens.Card.Progress).background(c.accent))
    }
}

private const val MINUTES_PER_HOUR = 60
