package com.slouchcat.app.tv.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens

/** The forms of a title card (`.tc`, `.tc.sq`, `.tc.wide`). */
enum class CardShape(val ratio: Float) {
    /** Films, series, books: 2:3. */
    Poster(POSTER_RATIO),

    /** Podcasts, radio, audiobooks. */
    Square(1f),

    /** Episodes, chapters, «Weiterschauen»: 16:9. */
    Wide(WIDE_RATIO),
    ;

    val width: Dp
        get() = when (this) {
            Poster -> TvDimens.Card.Width
            Square -> TvDimens.Card.Square
            Wide -> TvDimens.Card.Wide
        }
}

/** What the info line under a row says about the focused card (`.rinfo`: title, then a quiet second part). */
data class CardInfo(val title: String, val meta: String? = null)

/** Whether the row or grid of a card has the focus: cards of other rows rest a little darker. */
private val LocalRowFocused = compositionLocalOf { true }

/**
 * `.tc`: an image card. In focus it grows by `--focus-scale` and gets a light ring with a dark gap
 * (the focus is visible on any image); without an image it shows its ground and [title]. [caption]
 * (title over a shade at the bottom), [tag] (top left) and [progress] (0..1, a bar at the bottom)
 * are optional. [title] is also what a screen reader says.
 */
@Composable
fun TvCard(
    title: String,
    image: Any?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    shape: CardShape = CardShape.Poster,
    caption: String? = null,
    subcaption: String? = null,
    tag: String? = null,
    progress: Float? = null,
    /** `null`: as wide as its cell (a grid of seven columns). */
    width: Dp? = shape.width,
) {
    val c = SlouchcatTheme.colors
    val corner = RoundedCornerShape(TvDimens.radiusCard)
    val ring = RoundedCornerShape(TvDimens.radiusCard + TvDimens.Card.RingGap)
    val rowFocused = LocalRowFocused.current
    Surface(
        onClick = onClick,
        modifier = modifier
            .then(if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .aspectRatio(shape.ratio)
            .semantics(mergeDescendants = true) { contentDescription = title },
        shape = ClickableSurfaceDefaults.shape(shape = corner),
        colors = ClickableSurfaceDefaults.colors(containerColor = c.s2, focusedContainerColor = c.s2, pressedContainerColor = c.s2),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScale),
        border = ClickableSurfaceDefaults.border(
            focusedBorder = Border(BorderStroke(TvDimens.Card.Ring, c.tx), inset = -TvDimens.Card.RingGap, shape = ring),
        ),
    ) {
        Box(Modifier.fillMaxSize().clip(corner)) {
            if (image != null) {
                TvImage(image, Modifier.fillMaxSize())
            } else if (caption == null) {
                Text(
                    title,
                    style = TvType.bodyStrong,
                    color = c.tx2,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.align(Alignment.Center).padding(horizontal = TvDimens.Card.CaptionX),
                )
            }
            if (tag != null) {
                Text(
                    tag,
                    style = TvType.badge,
                    color = c.tx,
                    modifier = Modifier
                        .padding(TvDimens.s)
                        .background(SlouchcatTokens.Color.Scrim.Solid, RoundedCornerShape(TvDimens.radiusBadge))
                        .padding(horizontal = TvDimens.Badge.Pad, vertical = TvDimens.xs / 2),
                )
            }
            if (caption != null) {
                Column(
                    Modifier
                        .align(Alignment.BottomStart)
                        .fillMaxWidth()
                        .background(Brush.verticalGradient(listOf(SlouchcatTokens.Color.Scrim.Clear, SlouchcatTokens.Color.Scrim.Solid)))
                        .padding(horizontal = TvDimens.Card.CaptionX, vertical = TvDimens.Card.CaptionY),
                ) {
                    Text(caption, style = TvType.bodyStrong, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (subcaption != null) Text(subcaption, style = TvType.small, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            if (progress != null) {
                Box(
                    Modifier
                        .align(Alignment.BottomCenter)
                        .padding(horizontal = TvDimens.Card.CaptionX, vertical = TvDimens.s)
                        .fillMaxWidth()
                        .height(TvDimens.Card.Progress)
                        .clip(RoundedCornerShape(TvDimens.Card.Progress))
                        .background(SlouchcatTokens.Tv.Color.GlassStrong),
                ) {
                    Box(Modifier.fillMaxWidth(progress.coerceIn(0f, 1f)).height(TvDimens.Card.Progress).background(c.accent))
                }
            }
            // `.rsec:not(:focus-within) .tc .im { filter: brightness(.82) }`
            if (!rowFocused) Box(Modifier.fillMaxSize().background(c.bg.copy(alpha = 1f - TvMotion.rowRest)))
        }
    }
}

/**
 * `.rinfo`: the line under a row with the title of the focused card and a quiet second part (year,
 * length, episode). Shown only while the row has the focus; read out politely.
 */
@Composable
fun TvRowInfo(info: CardInfo?, visible: Boolean, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Row(
        modifier
            .fillMaxWidth()
            .heightIn(min = TvDimens.Card.Info)
            .padding(start = LocalContentStart.current, end = TvDimens.safeX)
            .alpha(if (visible && info != null) 1f else 0f)
            .semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite },
        horizontalArrangement = Arrangement.spacedBy(TvDimens.Card.InfoGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (info != null && visible) {
            Text(info.title, style = TvType.bodyStrong, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
            if (info.meta != null) Text(info.meta, style = TvType.body, color = c.tx3, maxLines = 1)
        }
    }
}

/**
 * `.rsec`: a titled row of cards (BAUKASTEN «Fokus-Engine»). Coming back into the row lands on the
 * card that had the focus last (`focusRestorer` with the first card as fallback); every card is
 * known to the page's [focus] as `"$rowKey-<key>"`, so Back to the page finds it again, and [state]
 * (save it with the page: `rememberLazyListState()` in the page) keeps the scroll. [info] fills the
 * line under the row for the focused card; [card] draws one card and must pass the modifier on.
 */
@Composable
fun <T> TvCardRow(
    title: String?,
    items: List<T>,
    key: (T) -> Any,
    focus: FocusMemory,
    rowKey: String,
    modifier: Modifier = Modifier,
    state: LazyListState = rememberLazyListState(),
    info: ((T) -> CardInfo?)? = null,
    card: @Composable (item: T, modifier: Modifier) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val first = remember { FocusRequester() }
    var hasFocus by remember { mutableStateOf(false) }
    var focusedKey by rememberSaveable { mutableStateOf<String?>(null) }
    Column(modifier.fillMaxWidth().onFocusChanged { hasFocus = it.hasFocus }) {
        if (title != null) {
            Text(
                title,
                style = TvType.largeStrong,
                color = if (hasFocus) c.tx else c.tx2,
                modifier = Modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX).semantics { heading() },
            )
        }
        CompositionLocalProvider(LocalRowFocused provides hasFocus) {
            LazyRow(
                state = state,
                contentPadding = PaddingValues(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.Card.RowTop, bottom = TvDimens.Card.RowBottom),
                horizontalArrangement = Arrangement.spacedBy(TvDimens.gap),
                // Into the row: the card it had (also after Back, when the row is new: a request of
                // the page's focus memory enters the row and lands here), else the first one.
                modifier = Modifier.fillMaxWidth().focusRestorer(focusedKey?.let(focus::requester) ?: first),
            ) {
                itemsIndexed(items, key = { _, item -> key(item) }) { index, item ->
                    val name = "$rowKey-${key(item)}"
                    card(
                        item,
                        Modifier
                            .focusKey(focus, name)
                            .then(if (index == 0) Modifier.focusRequester(first) else Modifier)
                            .onFocusChanged { if (it.isFocused) focusedKey = name },
                    )
                }
            }
        }
        if (info != null) {
            val focused = items.firstOrNull { "$rowKey-${key(it)}" == focusedKey }
            TvRowInfo(focused?.let(info), visible = hasFocus)
        }
    }
}

/**
 * A grid of cards (library, search, lists). Each tile is known to the page's [focus] as
 * `"$gridKey-<key>"`; coming back into the grid lands on the tile that had the focus last (first
 * tile as fallback). [columns] per row; [state] keeps the scroll with the page.
 */
@Composable
fun <T> TvCardGrid(
    items: List<T>,
    key: (T) -> Any,
    focus: FocusMemory,
    gridKey: String,
    columns: Int,
    modifier: Modifier = Modifier,
    state: LazyGridState = rememberLazyGridState(),
    /** A line below the tiles over the whole width (the retry of a failed next page). */
    footer: (@Composable () -> Unit)? = null,
    card: @Composable (item: T, modifier: Modifier) -> Unit,
) {
    val first = remember { FocusRequester() }
    var focusedKey by rememberSaveable { mutableStateOf<String?>(null) }
    LazyVerticalGrid(
        columns = GridCells.Fixed(columns),
        state = state,
        contentPadding = PaddingValues(start = LocalContentStart.current, end = TvDimens.safeX, top = TvDimens.Card.RowTop, bottom = TvDimens.safeY),
        horizontalArrangement = Arrangement.spacedBy(TvDimens.gap),
        verticalArrangement = Arrangement.spacedBy(TvDimens.gap),
        modifier = modifier.focusRestorer(focusedKey?.let(focus::requester) ?: first),
    ) {
        itemsIndexed(items, key = { _, item -> key(item) }) { index, item ->
            val name = "$gridKey-${key(item)}"
            card(
                item,
                Modifier
                    .focusKey(focus, name)
                    .then(if (index == 0) Modifier.focusRequester(first) else Modifier)
                    .onFocusChanged { if (it.isFocused) focusedKey = name },
            )
        }
        if (footer != null) item(span = { GridItemSpan(maxLineSpan) }) { footer() }
    }
}

private const val POSTER_RATIO = 2f / 3f
private const val WIDE_RATIO = 16f / 9f
