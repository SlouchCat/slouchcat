package com.slouchcat.app.mobile.media

import androidx.annotation.PluralsRes
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.ImageLoader
import coil3.compose.AsyncImage
import coil3.compose.AsyncImagePainter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.CardShape
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.strings.R
import java.text.NumberFormat

/**
 * The image loader of the media building blocks; `null` takes the app's (`graph.imageLoader`, the
 * singleton of `SlouchcatApplication`). Tests provide one that answers without a network.
 */
val LocalMediaImageLoader = staticCompositionLocalOf<ImageLoader?> { null }

/** Test tag of the field that stands in for a missing or failed image (`.ph` of the mockup). */
const val IMAGE_PLACEHOLDER_TAG = "media-image-placeholder"

/** Test tag of an image that loaded. */
const val IMAGE_LOADED_TAG = "media-image-loaded"

/**
 * An image of the server in the size its box needs ([ImageSet.pick] by the measured width), on
 * the surface colour while it loads. Without an image, or when it fails, a field with [name]
 * stands in its place (mockup `pic()`: «zeigt bei einem fehlenden Bild ein Feld mit dem Namen»).
 * Decorative: the card around it carries the name for the screen reader. [surface] `false`: no
 * surface behind it (a logo with transparency); [placeholder] `false`: nothing in place of a missing one.
 */
@Composable
fun MediaImage(
    images: ImageSet,
    name: String,
    modifier: Modifier = Modifier,
    contentScale: ContentScale = ContentScale.Crop,
    placeholder: Boolean = true,
    surface: Boolean = true,
    alignment: Alignment = Alignment.Center,
) {
    val c = SlouchcatTheme.colors
    BoxWithConstraints(if (surface) modifier.background(c.s2) else modifier) {
        val px = with(LocalDensity.current) { maxWidth.roundToPx() }
        val url = remember(images, px) { images.pick(px) }
        var state by remember(url) { mutableStateOf<AsyncImagePainter.State>(AsyncImagePainter.State.Empty) }
        val loaded = state is AsyncImagePainter.State.Success
        // The name stands until the image is there: also while it loads and when it never comes.
        if (!loaded && placeholder) {
            Box(
                Modifier
                    .fillMaxSize()
                    .background(Brush.linearGradient(listOf(c.s3, c.s1)))
                    .padding(8.dp)
                    .testTag(IMAGE_PLACEHOLDER_TAG)
                    .clearAndSetSemantics {},
                contentAlignment = Alignment.Center,
            ) {
                Text(name, style = SlouchcatType.label.copy(fontSize = 12.sp), color = c.tx3, textAlign = TextAlign.Center, maxLines = 4, overflow = TextOverflow.Ellipsis)
            }
        }
        if (url != null) {
            val loader = LocalMediaImageLoader.current
            val image = Modifier.fillMaxSize().then(if (loaded) Modifier.testTag(IMAGE_LOADED_TAG) else Modifier)
            if (loader != null) {
                AsyncImage(model = url, contentDescription = null, imageLoader = loader, contentScale = contentScale, alignment = alignment, modifier = image, onState = { state = it })
            } else {
                AsyncImage(model = url, contentDescription = null, contentScale = contentScale, alignment = alignment, modifier = image, onState = { state = it })
            }
        }
    }
}

/** `.fsk`: the minimum age as a small framed number; the screen reader says «ab 12». */
@Composable
fun FskBadge(rating: Int, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val label = stringResource(R.string.rating_age, rating.toString())
    Box(
        modifier
            .heightIn(min = 20.dp)
            .widthIn(min = 26.dp)
            .border(BorderStroke(1.dp, c.frame), RoundedCornerShape(5.dp))
            .padding(horizontal = 5.dp)
            .clearAndSetSemantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        Text(rating.toString(), style = SlouchcatType.badge.copy(fontSize = 11.5.sp), color = c.tx)
    }
}

/** The mark at the top left of a cover (`.pc .tag`): «♥ Tipp», «✓ Gesehen». */
data class CardTag(val text: String, val icon: SlouchcatIcon? = null)

@Composable
private fun TagMark(tag: CardTag, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Row(
        modifier
            .clip(RoundedCornerShape(6.dp))
            .background(c.scrim)
            .padding(horizontal = 6.dp, vertical = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(3.dp),
    ) {
        if (tag.icon != null) SlouchcatIconImage(tag.icon, contentDescription = null, size = 12.dp, tint = c.tx)
        Text(tag.text, style = SlouchcatType.badge.copy(fontSize = 11.sp), color = c.tx, maxLines = 1)
    }
}

/** The width of a card in a row (`.pc`): 124 dp, 248 dp wide; from 700 dp 150 dp and 300 dp. */
@Composable
fun cardWidth(shape: CardShape): Dp {
    val wide = isWideLayout()
    return when (shape) {
        CardShape.Wide -> if (wide) 300.dp else 248.dp
        else -> if (wide) 150.dp else 124.dp
    }
}

/**
 * The second line of a card when the page names none (mockup `card()`): the author of an
 * audiobook or podcast, else «2004 · Film».
 */
@Composable
fun defaultCaption(item: MediaItem): String? {
    if (item.isAudio && !item.author.isNullOrBlank()) return item.author
    val parts = listOfNotNull(item.year?.toString(), kindName(item.kind))
    return parts.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
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
    if (minutes >= 60) {
        stringResource(R.string.format_hoursMinutes, (minutes / 60).toString(), (minutes % 60).toString())
    } else {
        stringResource(R.string.format_minutes, minutes.toString())
    }

/** «Noch 18 min» of a started title. */
@Composable
fun remainingText(item: MediaItem): String? = item.remainingMinutes?.let { stringResource(R.string.home_remaining, runtimeText(it)) }

/** The tag «✓ Gesehen» (or «Gehört») of a finished title, `null` otherwise. */
@Composable
fun watchedTag(item: MediaItem): CardTag? =
    if (item.watched) CardTag(stringResource(if (item.isAudio) R.string.library_status_heard else R.string.library_status_watched), SlouchcatIcon.Check) else null

/**
 * A card of a title (mockup `card(i, opts)`, `.pc`): cover in [shape] (2:3, square for audio, 16:9
 * with the backdrop, the play mark and the progress bar), title in at most two lines, [caption]
 * (default [defaultCaption]) in one, [tag] on the cover. [width]: `null` fills the cell of a grid.
 * [onClick] `null` (its page is not there yet): the card is no control. The screen reader says
 * title, caption and tag as one control.
 */
@Composable
fun MediaCard(
    item: MediaItem,
    onClick: (() -> Unit)?,
    modifier: Modifier = Modifier,
    shape: CardShape = item.defaultShape,
    caption: String? = defaultCaption(item),
    tag: CardTag? = null,
    width: Dp? = cardWidth(shape),
) {
    val c = SlouchcatTheme.colors
    val title = item.seriesTitle?.takeIf { item.kind == ItemKind.EPISODE } ?: item.title
    val said = listOfNotNull(title, caption, tag?.text).joinToString(stringResource(R.string.format_listSeparator))
    Column(
        modifier
            .then(if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clip(RoundedCornerShape(10.dp))
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .clearAndSetSemantics { contentDescription = said }
            .heightIn(min = 48.dp),
    ) {
        val ratio = when (shape) {
            CardShape.Poster -> 2f / 3f
            CardShape.Square -> 1f
            CardShape.Wide -> 16f / 9f
        }
        Box(
            Modifier
                .fillMaxWidth()
                .aspectRatio(ratio)
                .clip(RoundedCornerShape(10.dp))
                .border(BorderStroke(1.dp, c.line), RoundedCornerShape(10.dp)),
        ) {
            val images = if (shape == CardShape.Wide) item.backdrop.or(item.poster) else item.poster
            MediaImage(images, title, Modifier.fillMaxSize())
            if (tag != null) TagMark(tag, Modifier.align(Alignment.TopStart).padding(6.dp))
            if (shape == CardShape.Wide) {
                Box(
                    Modifier
                        .align(Alignment.BottomEnd)
                        .padding(end = 8.dp, bottom = 12.dp)
                        .size(36.dp)
                        .clip(CircleShape)
                        .background(c.scrim)
                        .border(BorderStroke(1.dp, c.frame), CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    SlouchcatIconImage(SlouchcatIcon.Play, contentDescription = null, size = 18.dp, tint = c.tx)
                }
            }
            val progress = item.progress
            if (progress != null) {
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(4.dp).background(c.ctl)) {
                    Box(Modifier.fillMaxWidth(progress.coerceAtLeast(MIN_PROGRESS)).height(4.dp).background(c.accent))
                }
            }
        }
        Text(
            title,
            style = SlouchcatType.label.copy(fontSize = 14.sp, lineHeight = 17.5.sp),
            color = c.tx,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 8.dp),
        )
        if (caption != null) {
            Text(caption, style = SlouchcatType.caption.copy(fontSize = 12.5.sp), color = c.tx3, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 2.dp))
        }
    }
}

/** The bar shows at least this share, so a title just started still shows that it is (mockup `max(4, prog)`). */
private const val MIN_PROGRESS = 0.04f

/** «12 Titel», the count line of a list (`home.list.count`). */
@Composable
fun titleCount(count: Int): String = pluralCount(R.plurals.home_list_count, count)

/**
 * A plural text whose `{{count}}` (`%1$d`) shows the number in the format of the language
 * («1’846 Titel» in Swiss German); the plural form still follows the number. [more] fill the
 * placeholders after the count (`%2$s` …).
 */
@Composable
fun pluralCount(@PluralsRes id: Int, count: Int, vararg more: Any): String {
    val resources = LocalResources.current
    val locale = resources.configuration.locales[0]
    val template = resources.getQuantityText(id, count).toString().replace("%1\$d", "%1\$s")
    return String.format(locale, template, NumberFormat.getIntegerInstance(locale).format(count), *more)
}
