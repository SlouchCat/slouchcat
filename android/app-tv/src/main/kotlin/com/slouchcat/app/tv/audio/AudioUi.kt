package com.slouchcat.app.tv.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
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
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.player.timecode
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatTokens
import com.slouchcat.core.design.slouchcatIcon
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.ImageSource
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.ServerPaths
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl
import java.text.NumberFormat
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** The sizes of stage 4 (`tv.size.audio`). */
internal val A = SlouchcatTokens.Tv.Size.Audio

/** The icons of stage 4 the shared sets lack (`TvIcons` says: a page's own file). */
object AudioIcons {
    /** The bars of «Läuft gerade» (mockup `mdEq`). */
    val Bars: ImageVector by lazy {
        slouchcatIcon("bars", """<path d="M6 19V9M12 19V5M18 19v-7"/>""")
    }
}

/** «Kapitel 3» for a chapter without a title (web `player.chapters.untitled`). */
@Composable
fun chapterTitle(chapter: Chapter, index: Int): String =
    chapter.title?.takeIf { it.isNotBlank() } ?: stringResource(R.string.player_chapters_untitled, (index + 1).toString())

/** «1,25×» in the format of the language (web `player.audio.rate`). */
@Composable
fun rateText(speed: Float): String {
    val format = NumberFormat.getNumberInstance(LocalConfiguration.current.locales[0]).apply { maximumFractionDigits = 2 }
    return stringResource(R.string.player_audio_rate, format.format(speed.toDouble()))
}

/** «16 Kapitel». */
@Composable
fun chaptersText(count: Int): String = pluralStringResource(R.plurals.tv_audio_chapters, count, count)

/** «17. November 2026» in the language of the app. */
@Composable
fun dateText(date: OffsetDateTime): String =
    DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG).withLocale(LocalConfiguration.current.locales[0]).format(date)

/** The length of an episode: «32 Min.». */
@Composable
fun episodeLength(episode: ItemRef): String? =
    (episode.durationMs?.let { (it / MINUTE_MS).toInt() } ?: episode.runtimeMinutes)?.let { com.slouchcat.app.tv.browse.runtimeText(maxOf(1, it)) }

/** A logo or the current cover of a station as an image of the server (`null`: none). */
fun stationImage(server: HttpUrl?, path: String?): Any? {
    path ?: return null
    server ?: return null
    return ServerPaths.absolute(server, path)
}

/** The logo of [station] (`null`: the tile shows its name). */
fun stationLogo(server: HttpUrl?, station: RadioStation): Any? = stationImage(server, station.logo?.url)

/**
 * A row of a list of chapters or episodes (`.md-row`): a mark (number, check, dot), a title with a
 * second line and an optional bar, the length at the end. In focus it turns light.
 */
@Composable
fun AudioRow(
    title: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    end: String? = null,
    strong: Boolean = false,
    progress: Float? = null,
    label: String = title,
    mark: @Composable (focused: Boolean) -> Unit,
) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusMenuItem)
    Surface(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().heightIn(min = A.Row).semantics(mergeDescendants = true) { contentDescription = label },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleRail),
    ) {
        Row(Modifier.fillMaxWidth().padding(horizontal = A.RowPad, vertical = TvDimens.s), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(A.RowPad)) {
            Box(Modifier.width(A.RowMark), contentAlignment = Alignment.Center) { mark(false) }
            Column(Modifier.weight(1f)) {
                Text(title, style = if (strong) TvType.largeStrong else TvType.large, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (subtitle != null) Text(subtitle, style = TvType.body, color = androidx.tv.material3.LocalContentColor.current.copy(alpha = TvMotion.rowRest), maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (progress != null) {
                    Box(Modifier.padding(top = TvDimens.xs).width(A.RowBar).height(TvDimens.Card.Progress).clip(CircleShape).background(SlouchcatTokens.Tv.Color.GlassStrong)) {
                        Box(Modifier.fillMaxWidth(progress.coerceIn(0f, 1f)).height(TvDimens.Card.Progress).background(c.accent))
                    }
                }
            }
            if (end != null) Text(end, style = TvType.large, maxLines = 1)
        }
    }
}

/** The mark of a heard row (a check in the colour of «ok»). */
@Composable
fun HeardMark() = TvIcon(SlouchcatIcon.Check, TvDimens.Browse.Icon, tint = SlouchcatTheme.colors.ok)

/** The mark of an episode not heard yet (a dot of the accent). */
@Composable
fun NewMark() = Box(Modifier.size(A.RowDot).clip(CircleShape).background(SlouchcatTheme.colors.accent))

/** The number of a chapter. */
@Composable
fun NumberMark(number: Int) = Text(number.toString(), style = TvType.largeStrong, color = androidx.tv.material3.LocalContentColor.current)

/**
 * `#md-now`: what plays, at the top right left of the clock, on the pages with the rail (not on
 * the radio page while the radio plays: it shows the station itself). No control: the rail has
 * «Läuft gerade».
 */
@Composable
fun NowPill(now: AudioNow?, server: HttpUrl?) {
    val c = SlouchcatTheme.colors
    val (image, line, title) = when (now) {
        is AudioNow.Title -> {
            val item = now.state.item ?: return
            val paused = !now.state.playing && !now.state.buffering && !now.state.loading
            Triple(
                server?.let { ServerImages(it).posters(item.images).forWidth(A.NowImage) },
                stringResource(if (paused) R.string.radio_page_paused else R.string.tv_audio_nowPlaying),
                titleOf(item),
            )
        }
        is AudioNow.Radio -> {
            val station = now.state.station ?: return
            val song = now.state.now?.let { entry -> nowLine(entry.artist, entry.title, entry.text) }
            Triple(
                stationImage(server, now.state.now?.cover?.url ?: station.logo?.url),
                listOf(stringResource(R.string.radio_player_live), station.name).joinToString(stringResource(R.string.format_separator)),
                song ?: station.name,
            )
        }
        is AudioNow.Restored, null -> return
    }
    Box(Modifier.fillMaxSize()) {
        Row(
            Modifier
                .align(Alignment.TopEnd)
                .padding(top = TvDimens.safeY - TvDimens.s, end = TvDimens.safeX + A.NowEnd)
                .widthIn(max = A.NowWidth)
                .height(A.NowHeight)
                .clip(CircleShape)
                .background(SlouchcatTokens.Tv.Color.Rail)
                .border(BorderStroke(TvDimens.borderThin, c.line), CircleShape)
                .padding(start = TvDimens.xs, end = TvDimens.l)
                .semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite },
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TvDimens.m),
        ) {
            Box(Modifier.size(A.NowImage).clip(CircleShape).background(c.s2), contentAlignment = Alignment.Center) {
                if (image != null) TvImage(image, Modifier.fillMaxSize()) else TvIcon(SlouchcatIcon.Radio, TvDimens.Browse.Icon, tint = c.tx3)
            }
            Column(Modifier.weight(1f, fill = false)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.xs)) {
                    TvIcon(AudioIcons.Bars, TvDimens.Button.Icon, tint = c.accent)
                    Text(line, style = TvType.small, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                Text(title, style = TvType.small.copy(fontWeight = androidx.compose.ui.text.font.FontWeight.Bold), color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** «Artist – Title» of a station's current entry, else its text (`radio.player.artistTitle`). */
@Composable
fun nowLine(artist: String?, title: String?, text: String?): String? = when {
    !artist.isNullOrBlank() && !title.isNullOrBlank() -> stringResource(R.string.radio_player_artistTitle, artist, title)
    else -> text?.takeIf { it.isNotBlank() }
}

/** A row of [content] at the start of the page's content (right of the rail). */
@Composable
fun PageRow(modifier: Modifier = Modifier, content: @Composable RowScope.() -> Unit) =
    Row(modifier.padding(start = LocalContentStart.current, end = TvDimens.safeX), horizontalArrangement = Arrangement.spacedBy(TvDimens.l), verticalAlignment = Alignment.CenterVertically, content = content)

/** «14:31» and «−27:25» under a timeline. */
@Composable
fun remainingText(ms: Long): String = stringResource(R.string.tv_player_behind, timecode(ms))

private const val MINUTE_MS = 60_000L
private const val LOGO_WIDTH = 512
