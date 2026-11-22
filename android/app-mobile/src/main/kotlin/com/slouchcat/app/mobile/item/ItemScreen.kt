package com.slouchcat.app.mobile.item

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.triStateToggleable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import coil3.compose.AsyncImagePainter
import com.slouchcat.app.mobile.media.DetailState
import com.slouchcat.app.mobile.media.FskBadge
import com.slouchcat.app.mobile.media.LocalMediaImageLoader
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.RoundIcon
import com.slouchcat.app.mobile.media.kindName
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.item.ItemAsk
import com.slouchcat.core.browse.item.ItemEvents
import com.slouchcat.core.browse.item.ItemUiState
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetHeadIcon
import com.slouchcat.core.design.SheetText
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatSnackbarHost
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.PlayTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.hasCheck
import com.slouchcat.core.media.isAudio
import com.slouchcat.core.media.listable
import com.slouchcat.core.media.minutesLeft
import com.slouchcat.core.media.playTarget
import com.slouchcat.core.media.share
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R
import java.util.Locale

/**
 * Where the detail page leads. Each link is `null` while no feature registered its page
 * (`Navigator.opener`): then its card or button is no control. [play]: the player of stage 5.
 */
data class ItemLinks(
    val back: () -> Unit,
    val item: (String) -> (() -> Unit)? = { null },
    val person: (Long) -> (() -> Unit)? = { null },
    val collection: (String) -> (() -> Unit)? = { null },
    val search: (() -> Unit)? = null,
    val play: ((String) -> Unit)? = null,
    /** «Von vorn» under the bar of a started title (mockup `from0`): the player from the beginning. */
    val playFromStart: ((String) -> Unit)? = null,
    /** Opens YouTube outside the app (concept 27 3.3, F-30). */
    val trailer: (String) -> Unit = {},
)

/**
 * The detail page of a film, series, season, episode or any other title (mockup `/item/:i`,
 * reference photos handy 16 to 19): the backdrop with its shade, logo or title, the facts, the
 * big button, the round actions, the summary, cast, episodes, file and technique, collections and
 * extras. From 700 dp two columns: poster, button and actions on the left, the rest on the right.
 * [download]: the slot of stage 6 (nothing until then).
 */
@Composable
fun ItemScreen(
    state: ItemUiState,
    images: ServerImages,
    links: ItemLinks,
    events: ItemEvents,
    snackbar: SnackbarHostState,
    modifier: Modifier = Modifier,
    download: (@Composable (ItemDetail) -> Unit)? = null,
) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxSize().background(c.bg)) {
        when (val item = state.item) {
            is Loadable.Ready -> Page(item.value, state, images, links, events, download)
            else -> DetailState(item, stringResource(R.string.item_notFound_title), stringResource(R.string.item_notFound_body), SlouchcatIcon.Film, events::retry)
        }
        TopActions(links)
        SlouchcatSnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter).padding(16.dp))
    }
    when (val ask = state.ask) {
        is ItemAsk.Watched -> WatchedSheet(ask, events)
        ItemAsk.Recommend -> state.detail?.let { RecommendSheet(it, events) }
        null -> Unit
    }
}

/** The back arrow and the search over the picture (mockup `topBack`). */
@Composable
private fun TopActions(links: ItemLinks) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .background(Brush.verticalGradient(listOf(c.bg.copy(alpha = 0.7f), Color.Transparent)))
            .padding(horizontal = 6.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RoundIcon(SlouchcatIcon.Back, stringResource(R.string.common_back), links.back)
        Spacer(Modifier.weight(1f))
        links.search?.let { RoundIcon(SlouchcatIcon.Search, stringResource(R.string.app_item_search), it) }
    }
}

@Composable
private fun Page(
    item: ItemDetail,
    state: ItemUiState,
    images: ServerImages,
    links: ItemLinks,
    events: ItemEvents,
    download: (@Composable (ItemDetail) -> Unit)?,
) {
    val media = remember(item, images) { MediaItem.from(item, images) }
    val wide = isWideLayout()
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
        if (wide) WidePage(item, media, state, images, links, events, download) else PhonePage(item, media, state, images, links, events, download)
    }
}

/** The kinds drawn with their cover in front of a blurred one (audio; an episode shows its still). */
private fun coverShape(item: ItemDetail): Float = when {
    isAudio(item.kind) -> 1f
    item.kind == ItemKind.EPISODE -> 16f / 9f
    else -> 2f / 3f
}

@Composable
private fun PhonePage(
    item: ItemDetail,
    media: MediaItem,
    state: ItemUiState,
    images: ServerImages,
    links: ItemLinks,
    events: ItemEvents,
    download: (@Composable (ItemDetail) -> Unit)?,
) {
    val audio = isAudio(item.kind)
    val heroHeight = if (audio) 260.dp else 300.dp
    Box(Modifier.fillMaxWidth()) {
        Hero(item, media, Modifier.fillMaxWidth().height(heroHeight), wide = false)
        Column(Modifier.fillMaxWidth().padding(top = if (audio) 60.dp else heroHeight - 92.dp)) {
            if (audio) {
                Cover(media, 1f, Modifier.align(Alignment.CenterHorizontally).width(200.dp).padding(bottom = 18.dp))
            }
            Head(item, media, centered = true, modifier = Modifier.padding(horizontal = gutter()))
            Column(Modifier.padding(start = gutter(), end = gutter(), top = 18.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                PlayArea(item, links)
            }
            Actions(item, state, links, events, download, wide = false, modifier = Modifier.padding(start = 8.dp, end = 8.dp, top = 12.dp, bottom = 4.dp))
            Related(item, links, Modifier.padding(horizontal = gutter()))
        }
    }
    Main(item, state, images, links, events)
    Rows(item, images, links)
}

@Composable
private fun WidePage(
    item: ItemDetail,
    media: MediaItem,
    state: ItemUiState,
    images: ServerImages,
    links: ItemLinks,
    events: ItemEvents,
    download: (@Composable (ItemDetail) -> Unit)?,
) {
    Box(Modifier.fillMaxWidth()) {
        Hero(item, media, Modifier.fillMaxWidth().height(460.dp), wide = true)
        Row(
            Modifier.fillMaxWidth().padding(start = 32.dp, end = 40.dp, top = 224.dp),
            horizontalArrangement = Arrangement.spacedBy(44.dp),
        ) {
            Column(Modifier.width(272.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Cover(media, coverShape(item), Modifier.fillMaxWidth())
                PlayArea(item, links)
                Actions(item, state, links, events, download, wide = true)
            }
            Column(Modifier.weight(1f).padding(top = 72.dp)) {
                Head(item, media, centered = false, modifier = Modifier.widthIn(max = 720.dp))
                Related(item, links, Modifier.padding(top = 12.dp))
                Main(item, state, images, links, events)
            }
        }
    }
    Rows(item, images, links)
}

/** The picture behind the head: the backdrop, an episode's still, else the blurred cover (`.dt-bg`). */
@Composable
private fun Hero(item: ItemDetail, media: MediaItem, modifier: Modifier, wide: Boolean) {
    val c = SlouchcatTheme.colors
    val backdrop = !media.backdrop.isEmpty && !isAudio(item.kind)
    Box(modifier.background(c.s1)) {
        if (backdrop) {
            MediaImage(media.backdrop, item.title, Modifier.fillMaxSize(), placeholder = false)
        } else if (!media.poster.isEmpty) {
            MediaImage(media.poster, item.title, Modifier.fillMaxSize().blur(30.dp), placeholder = false)
        }
        val bg = c.bg
        Box(
            Modifier.fillMaxSize().background(
                Brush.verticalGradient(
                    0f to bg.copy(alpha = if (wide) 0.4f else 0.45f),
                    (if (wide) 0.22f else 0.28f) to Color.Transparent,
                    (if (wide) 0.5f else 0.62f) to bg.copy(alpha = if (wide) 0f else 0.4f),
                    1f to bg,
                ),
            ),
        )
        if (wide) {
            Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to bg.copy(alpha = 0.88f), 0.42f to bg.copy(alpha = 0.45f), 0.72f to Color.Transparent)))
        }
    }
}

/** The cover in front (`.dt-poster`): 2:3, square for audio, 16:9 for an episode's still. */
@Composable
private fun Cover(media: MediaItem, ratio: Float, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(16.dp)
    val images = if (ratio > 1f) media.backdrop.or(media.poster) else media.poster
    Box(modifier.aspectRatio(ratio).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
        MediaImage(images, media.title, Modifier.fillMaxSize())
    }
}

/** «S01E02 · Serie» over an episode's title, the series over a season's. */
@Composable
private fun kicker(item: ItemDetail): String? {
    val series = item.seriesRef?.title ?: item.seriesTitle ?: item.series?.title
    return when (item.kind) {
        ItemKind.EPISODE -> {
            val code = episodeCode(item.seasonNumber, item.episodeNumber, item.episodeEnd)
            when {
                code != null && series != null -> stringResource(R.string.item_episodeOf, code, series)
                else -> code ?: series
            }
        }
        ItemKind.SEASON -> series
        ItemKind.PODCAST_EPISODE -> item.podcast?.title
        else -> null
    }
}

/** «S01E02» (web `episodeCode`); `null` without numbers. */
@Composable
fun episodeCode(season: Int?, episode: Int?, end: Int?): String? {
    if (season == null || episode == null) return null
    // Latin digits as in the web app, also in Arabic or Persian (code review L-7).
    val s = String.format(Locale.ROOT, "%02d", season)
    val e = String.format(Locale.ROOT, "%02d", episode)
    return if (end != null && end > episode) stringResource(R.string.item_episodeCodeRange, s, e, String.format(Locale.ROOT, "%02d", end)) else stringResource(R.string.item_episodeCode, s, e)
}

/** Logo or title, the facts and the tagline (`.dt-head`). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Head(item: ItemDetail, media: MediaItem, centered: Boolean, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val wide = isWideLayout()
    val align = if (centered) Alignment.CenterHorizontally else Alignment.Start
    Column(modifier.fillMaxWidth(), horizontalAlignment = align, verticalArrangement = Arrangement.spacedBy(10.dp)) {
        kicker(item)?.let { Text(it, style = SlouchcatType.label.copy(fontSize = 14.sp), color = c.tx2, textAlign = if (centered) TextAlign.Center else TextAlign.Start) }
        TitleOrLogo(item.title, media.logo, centered, wide)
        if (isAudio(item.kind) && !item.author.isNullOrBlank()) {
            Text(stringResource(R.string.item_audio_by, item.author!!), style = SlouchcatType.bodySmall, color = c.tx2)
        }
        val separator = stringResource(R.string.format_separator).trim()
        val facts = facts(item)
        if (facts.isNotEmpty() || item.rating != null) {
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp, if (centered) Alignment.CenterHorizontally else Alignment.Start),
                verticalArrangement = Arrangement.spacedBy(4.dp),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                val style = SlouchcatType.bodySmall.copy(fontSize = if (wide) 15.sp else 14.sp)
                val parts = buildList<@Composable () -> Unit> {
                    item.year?.let { y -> add { Text(y.toString(), style = style, color = c.tx2) } }
                    item.rating?.let { r -> add { FskBadge(r) } }
                    facts.forEach { f -> add { Text(f, style = style, color = c.tx2, textAlign = if (centered) TextAlign.Center else TextAlign.Start) } }
                }
                parts.forEachIndexed { i, part ->
                    if (i > 0) Text(separator, style = style, color = c.tx3)
                    part()
                }
            }
        }
        item.tagline?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = SlouchcatType.body.copy(fontSize = if (wide) 17.sp else 15.sp, fontStyle = FontStyle.Italic), color = c.tx2, textAlign = if (centered) TextAlign.Center else TextAlign.Start)
        }
    }
}

/** Duration, seasons or kind, then the genres (`.dt-meta` after year and age). */
@Composable
private fun facts(item: ItemDetail): List<String> = buildList {
    when (item.kind) {
        ItemKind.SERIES -> item.seasons?.size?.takeIf { it > 0 }?.let { add(pluralStringResource(R.plurals.app_item_seasons, it, it)) } ?: kindName(item.kind)?.let(::add)
        ItemKind.SEASON -> (item.childCount ?: item.episodes?.size)?.let { add(pluralStringResource(R.plurals.app_item_episodes, it, it)) }
        else -> {
            val minutes = item.runtimeMinutes ?: item.durationMs?.let { (it / 60_000).toInt() }?.takeIf { it > 0 }
            if (minutes != null) add(runtimeText(minutes)) else kindName(item.kind)?.let(::add)
        }
    }
    if (item.genres.isNotEmpty()) add(item.genres.joinToString(stringResource(R.string.format_listSeparator)))
}

/** The title logo, else the title (`.dt-title`); the logo is said as the title, both are the heading. */
@Composable
private fun TitleOrLogo(title: String, logo: ImageSet, centered: Boolean, wide: Boolean) {
    val c = SlouchcatTheme.colors
    var failed by remember(logo) { mutableStateOf(false) }
    val url = logo.pick(if (wide) 880 else 640)
    if (url != null && !failed) {
        val loader = LocalMediaImageLoader.current
        val modifier = Modifier
            .heightIn(max = if (wide) 120.dp else 92.dp)
            .widthIn(max = if (wide) 440.dp else 320.dp)
            .semantics { contentDescription = title; heading() }
        val onState: (AsyncImagePainter.State) -> Unit = { if (it is AsyncImagePainter.State.Error) failed = true }
        val alignment = if (centered) Alignment.Center else Alignment.CenterStart
        if (loader != null) {
            AsyncImage(url, contentDescription = null, imageLoader = loader, contentScale = ContentScale.Fit, alignment = alignment, modifier = modifier, onState = onState)
        } else {
            AsyncImage(url, contentDescription = null, contentScale = ContentScale.Fit, alignment = alignment, modifier = modifier, onState = onState)
        }
    } else {
        Text(
            title,
            style = SlouchcatType.pageTitle.copy(fontSize = if (wide) 40.sp else 28.sp, lineHeight = if (wide) 46.sp else 32.sp, fontWeight = FontWeight.ExtraBold),
            color = c.tx,
            textAlign = if (centered) TextAlign.Center else TextAlign.Start,
            modifier = Modifier.semantics { heading() },
        )
    }
}

/** The big button with the bar and «noch …» (`.dt-cta`); only once the player is there (stage 5). */
@Composable
private fun PlayArea(item: ItemDetail, links: ItemLinks) {
    val c = SlouchcatTheme.colors
    val play = links.play ?: return
    val target = playTarget(item) ?: return
    val code = target.episode?.let { episodeShort(it.seasonNumber, it.episodeNumber) }
    val label = when (target.kind) {
        PlayTarget.Kind.Play -> stringResource(R.string.app_item_play)
        PlayTarget.Kind.Resume -> stringResource(R.string.app_item_resume)
        PlayTarget.Kind.Again -> stringResource(R.string.app_item_again)
        PlayTarget.Kind.Episode -> code?.let { stringResource(R.string.app_item_playEpisode, it) } ?: stringResource(R.string.app_item_play)
        PlayTarget.Kind.EpisodeResume -> code?.let { stringResource(R.string.app_item_resumeEpisode, it) } ?: stringResource(R.string.app_item_resume)
    }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        SlouchcatButton(label, onClick = { play(target.id) }, block = true, leadingIcon = SlouchcatIcon.Play, modifier = Modifier.heightIn(min = 54.dp))
        target.resume?.let { p ->
            ProgressBar(share(p))
            val left = runtimeText(minutesLeft(p))
            val episodeTitle = target.episode?.title
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(
                    if (episodeTitle != null) stringResource(R.string.app_item_leftEpisode, left, episodeTitle) else stringResource(R.string.app_item_left, left),
                    style = SlouchcatType.caption.copy(fontSize = 13.5.sp),
                    color = c.tx3,
                    modifier = Modifier.weight(1f),
                )
                links.playFromStart?.let { fromStart ->
                    Text(
                        stringResource(R.string.app_item_fromStart),
                        style = SlouchcatType.label.copy(fontSize = 13.5.sp),
                        color = c.tx,
                        modifier = Modifier
                            .heightIn(min = 48.dp)
                            .clickable(role = Role.Button) { fromStart(target.id) }
                            .wrapContentHeight(Alignment.CenterVertically)
                            .padding(horizontal = 4.dp),
                    )
                }
            }
        }
    }
}

/** «S1 · F3» of the big button (mockup `epLabel`). */
@Composable
fun episodeShort(season: Int?, episode: Int?): String? =
    if (season != null && episode != null) stringResource(R.string.home_continue_episode, season.toString(), episode.toString()) else null

@Composable
fun ProgressBar(share: Float, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    Box(modifier.fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp)).background(c.s3)) {
        Box(Modifier.fillMaxWidth(share.coerceAtLeast(0.03f)).height(4.dp).background(c.accent))
    }
}

/**
 * The round actions (`.dt-acts`): Merken, Gesehen, the download of stage 6, Empfehlen, Trailer;
 * a row of five on the phone, a list from 700 dp. Each says its state (on, off, half).
 */
@Composable
private fun Actions(
    item: ItemDetail,
    state: ItemUiState,
    links: ItemLinks,
    events: ItemEvents,
    download: (@Composable (ItemDetail) -> Unit)?,
    wide: Boolean,
    modifier: Modifier = Modifier,
) {
    val audio = isAudio(item.kind)
    val entries = buildList<@Composable (Modifier) -> Unit> {
        if (listable(item.kind)) {
            add { m ->
                ActionButton(
                    if (item.inWatchlist) SlouchcatIcon.BookmarkOn else SlouchcatIcon.Bookmark,
                    stringResource(if (item.inWatchlist) R.string.item_watchlist_added else R.string.item_watchlist_add),
                    if (item.inWatchlist) ToggleableState.On else ToggleableState.Off,
                    events::onWatchlist,
                    wide,
                    m,
                )
            }
        }
        if (hasCheck(item.kind)) {
            add { m ->
                ActionButton(
                    SlouchcatIcon.Check,
                    stringResource(if (audio) R.string.watched_heard_button else R.string.watched_button),
                    when (item.watchedMark) {
                        WatchedMark.None -> ToggleableState.Off
                        WatchedMark.Half -> ToggleableState.Indeterminate
                        WatchedMark.Full -> ToggleableState.On
                    },
                    events::onWatched,
                    wide,
                    m,
                )
            }
        }
        if (download != null) add { m -> Box(m) { download(item) } }
        if (item.canRecommend) {
            add { m ->
                ActionButton(
                    SlouchcatIcon.Heart,
                    stringResource(if (item.recommendedByMe) R.string.item_recommend_done else R.string.item_recommend_add),
                    if (item.recommendedByMe) ToggleableState.On else ToggleableState.Off,
                    events::onRecommendAsk,
                    wide,
                    m,
                )
            }
        }
        val trailer = state.trailer
        if (trailer != null && !audio) {
            add { m -> ActionButton(SlouchcatIcon.Film, stringResource(R.string.item_trailer), null, { links.trailer(trailer) }, wide, m, stringResource(R.string.app_item_trailerLabel)) }
        }
    }
    if (entries.isEmpty()) return
    if (wide) {
        Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(2.dp)) { entries.forEach { it(Modifier.fillMaxWidth()) } }
    } else {
        Row(modifier.fillMaxWidth()) { entries.forEach { it(Modifier.weight(1f)) } }
    }
}

/** One round action (`.dt-act`): the icon in its pill, filled with the accent when on, the label. */
@Composable
internal fun ActionButton(
    icon: SlouchcatIcon,
    label: String,
    toggle: ToggleableState?,
    onClick: () -> Unit,
    wide: Boolean,
    modifier: Modifier,
    description: String? = null,
) {
    val c = SlouchcatTheme.colors
    val on = toggle == ToggleableState.On || toggle == ToggleableState.Indeterminate
    val control = if (toggle != null) {
        Modifier.triStateToggleable(state = toggle, role = Role.Checkbox, onClick = onClick)
    } else {
        Modifier.clickable(role = Role.Button, onClick = onClick)
    }
    val said = if (description != null) Modifier.semantics(mergeDescendants = true) { contentDescription = description } else Modifier
    val pill: @Composable (Dp, Dp) -> Unit = { w, h ->
        Box(Modifier.size(w, h).clip(RoundedCornerShape(h / 2)).background(if (on) c.accent else Color.Transparent), contentAlignment = Alignment.Center) {
            SlouchcatIconImage(icon, contentDescription = null, size = 22.dp, tint = if (on) c.onAccent else c.tx2)
        }
    }
    if (wide) {
        Row(
            modifier.clip(RoundedCornerShape(24.dp)).then(control).then(said).heightIn(min = 48.dp).padding(horizontal = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            pill(40.dp, 32.dp)
            Text(label, style = SlouchcatType.button, color = if (on) c.tx else c.tx2)
        }
    } else {
        Column(
            modifier.clip(RoundedCornerShape(16.dp)).then(control).then(said).heightIn(min = 68.dp).padding(vertical = 6.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            pill(48.dp, 36.dp)
            Text(label, style = SlouchcatType.label.copy(fontSize = 12.sp), color = if (on) c.tx else c.tx2, textAlign = TextAlign.Center)
        }
    }
}

/** «Nächste Folge», «Zur Serie», «Zum Podcast» (web `RelatedLinks`). */
@Composable
private fun Related(item: ItemDetail, links: ItemLinks, modifier: Modifier = Modifier) {
    val next = item.nextEpisode?.takeIf { item.kind == ItemKind.EPISODE }?.let { e -> links.item(e.id)?.let { it to e } }
    val seriesId = when (item.kind) {
        ItemKind.EPISODE -> item.series?.id ?: item.seriesRef?.id
        ItemKind.SEASON -> item.parentId ?: item.series?.id
        else -> null
    }
    val series = seriesId?.let(links.item)
    val podcast = item.podcast?.takeIf { item.kind == ItemKind.PODCAST_EPISODE }?.let { links.item(it.id) }
    if (next == null && series == null && podcast == null) return
    Column(modifier.fillMaxWidth().padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        next?.let { (open, episode) ->
            val code = episodeCode(episode.seasonNumber, episode.episodeNumber, episode.episodeEnd)
            val label = listOfNotNull(stringResource(R.string.item_nextEpisode), code, episode.title).joinToString(stringResource(R.string.format_separator))
            SlouchcatButton(label, onClick = open, kind = ButtonKind.Secondary, block = true, trailingIcon = SlouchcatIcon.Right)
        }
        series?.let { SlouchcatButton(stringResource(R.string.item_seriesLink), onClick = it, kind = ButtonKind.Ghost, block = true) }
        podcast?.let { SlouchcatButton(stringResource(R.string.item_podcast_link), onClick = it, kind = ButtonKind.Ghost, block = true) }
    }
}

/** «… als ungesehen markieren?» for a series, season or podcast, or a half check (web `WatchedChoice`). */
@Composable
private fun WatchedSheet(ask: ItemAsk.Watched, events: ItemEvents) {
    val title = stringResource(
        when {
            ask.half && ask.audio -> R.string.watched_heard_ask_halfTitle
            ask.half -> R.string.watched_ask_halfTitle
            ask.audio -> R.string.watched_heard_ask_removeTitle
            else -> R.string.watched_ask_removeTitle
        },
        ask.title,
    )
    val body = when {
        ask.half -> pluralStringResource(R.plurals.watched_ask_halfBody, ask.count, ask.count)
        ask.count > 0 -> pluralStringResource(R.plurals.watched_ask_removeBody, ask.count, ask.count)
        else -> stringResource(R.string.watched_ask_removeBodyAll)
    }
    SlouchcatSheet(title = title, onDismiss = events::onDismissAsk) {
        SheetHeadIcon(SlouchcatIcon.Check)
        SheetTitle(title, centered = true)
        SheetText(body, centered = true)
        if (ask.half) {
            Column(Modifier.padding(start = gutter(), end = gutter(), top = 14.dp)) {
                SlouchcatButton(
                    stringResource(if (ask.audio) R.string.watched_heard_ask_markAll else R.string.watched_ask_markAll),
                    onClick = { events.onAskChoice(true) },
                    block = true,
                )
            }
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismissAsk, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(stringResource(R.string.watched_ask_removeAll), onClick = { events.onAskChoice(false) }, kind = ButtonKind.Danger, modifier = Modifier.weight(1f))
        }
    }
}

/**
 * «Empfehlen» (web `DetailActions`, concept 18 part B): the title goes to «User-Empfehlungen» of
 * every profile that may see it, without the own name. Who sees it the server decides (rule 12),
 * so the sheet names no profiles. A recommended title offers taking it back.
 */
@Composable
private fun RecommendSheet(item: ItemDetail, events: ItemEvents) {
    val title = stringResource(R.string.app_item_recommendTitle)
    SlouchcatSheet(title = title, onDismiss = events::onDismissAsk) {
        SheetHeadIcon(SlouchcatIcon.Heart)
        SheetTitle(item.title, centered = true)
        SheetText(stringResource(R.string.item_recommend_hint), centered = true)
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_cancel), onClick = events::onDismissAsk, kind = ButtonKind.Ghost, modifier = Modifier.weight(1f))
            SlouchcatButton(
                stringResource(if (item.recommendedByMe) R.string.app_item_unrecommend else R.string.app_item_recommendConfirm),
                onClick = events::onRecommendConfirm,
                kind = if (item.recommendedByMe) ButtonKind.Danger else ButtonKind.Accent,
                modifier = Modifier.weight(1f),
            )
        }
    }
}

/** A section of the page with its heading and an optional note at the end (`.dt-sec`, `.dt-h`). */
@Composable
fun Section(title: String?, modifier: Modifier = Modifier, end: String? = null, content: @Composable ColumnScope.() -> Unit) {
    val c = SlouchcatTheme.colors
    Column(modifier.fillMaxWidth().padding(top = 18.dp)) {
        if (title != null) {
            Row(Modifier.fillMaxWidth().padding(bottom = 10.dp), verticalAlignment = Alignment.Bottom) {
                Text(title, style = SlouchcatType.strong.copy(fontSize = 19.sp, lineHeight = 24.sp), color = c.tx, modifier = Modifier.weight(1f).semantics { heading() })
                end?.let { Text(it, style = SlouchcatType.bodySmall.copy(fontSize = 14.sp), color = c.tx3) }
            }
        }
        content()
    }
}

