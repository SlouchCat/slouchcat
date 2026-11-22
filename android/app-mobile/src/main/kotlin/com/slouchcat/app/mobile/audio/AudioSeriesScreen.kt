package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.slouchcat.app.mobile.item.ProgressBar
import com.slouchcat.app.mobile.media.DetailState
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.app.mobile.media.pluralCount
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.AudioSeriesVolume
import com.slouchcat.core.design.BarNavigation
import com.slouchcat.core.design.SectionTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.TopBar
import com.slouchcat.core.design.gutter
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.minutesLeft
import com.slouchcat.core.media.share
import com.slouchcat.core.media.started
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R
import java.math.BigDecimal

/** Where the series page leads; `null` while the page behind it is not there. */
data class SeriesLinks(
    val back: () -> Unit,
    val retry: () -> Unit,
    val item: ((String) -> Unit)?,
    val play: ((String) -> Unit)?,
)

/**
 * The series of an audiobook (web `SeriesPage`): stacked cover, «Reihe», name, author, the reader
 * when every volume has the same, the number of volumes and the length; «Weiterhören: Band n» for
 * the server's next volume; the volumes in their order with their progress, each opening its page
 * and playing where it stands (volumes always resume, O-9).
 */
@Composable
fun AudioSeriesScreen(state: Loadable<AudioSeries>, images: ServerImages, links: SeriesLinks) {
    val c = SlouchcatTheme.colors
    Scaffold(
        containerColor = c.bg,
        topBar = { TopBar(stringResource(R.string.audioSeries_kicker), navigation = BarNavigation.Back, navigationLabel = stringResource(R.string.common_back), onNavigation = links.back) },
    ) { padding ->
        Box(Modifier.fillMaxSize().padding(padding)) {
            if (state !is Loadable.Ready) {
                DetailState(state, stringResource(R.string.audioSeries_notFound_title), stringResource(R.string.audioSeries_notFound_body), SlouchcatIcon.Book, links.retry)
                return@Box
            }
            val series = state.value
            LazyColumn(
                contentPadding = PaddingValues(horizontal = gutter(), vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.fillMaxSize(),
            ) {
                item { Head(series, images, links) }
                item { SectionTitle(stringResource(R.string.audioSeries_list), inset = false, modifier = Modifier.padding(top = 18.dp, bottom = 4.dp)) }
                items(series.volumes, key = { it.item.id }) { volume -> VolumeRow(volume, images, links) }
            }
        }
    }
}

@Composable
private fun Head(series: AudioSeries, images: ServerImages, links: SeriesLinks) {
    val c = SlouchcatTheme.colors
    Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        val shape = RoundedCornerShape(16.dp)
        Box(Modifier.widthIn(max = 220.dp).fillMaxWidth(0.55f).aspectRatio(1f).clip(shape).border(BorderStroke(1.dp, c.line), shape)) {
            MediaImage(images.posters(series.cover?.images), series.name, Modifier.fillMaxSize())
        }
        Text(stringResource(R.string.audioSeries_kicker), style = SlouchcatType.label, color = c.tx2)
        Text(series.name, style = SlouchcatType.pageTitle.copy(fontSize = 26.sp), color = c.tx, textAlign = TextAlign.Center, modifier = Modifier.semantics { heading() })
        series.author?.let { Text(it, style = SlouchcatType.rowTitle, color = c.tx2) }
        series.narrator?.let { Text(stringResource(R.string.audioSeries_narratedBy, it), style = SlouchcatType.caption, color = c.tx3) }
        Text(
            listOf(pluralCount(R.plurals.audioSeries_volumes, series.volumes.size), runtimeText((series.durationMs / 60_000).toInt())).joinToString(stringResource(R.string.format_separator)),
            style = SlouchcatType.caption,
            color = c.tx3,
        )
        val next = series.next
        val play = links.play
        if (play != null && next != null) {
            val volume = series.volumes.firstOrNull { it.item.id == next.id }?.seriesIndex
            SlouchcatButton(
                volume?.let { stringResource(R.string.audioSeries_continue, number(it)) } ?: stringResource(R.string.audioSeries_continueTitle, next.title),
                onClick = { play(next.id) },
                block = true,
                leadingIcon = SlouchcatIcon.Play,
                modifier = Modifier.padding(top = 8.dp).heightIn(min = 54.dp),
            )
        }
    }
}

@Composable
private fun VolumeRow(volume: AudioSeriesVolume, images: ServerImages, links: SeriesLinks) {
    val c = SlouchcatTheme.colors
    val item = volume.item
    val title = volume.seriesIndex?.let { stringResource(R.string.audioSeries_volumeLabel, number(it), item.title) } ?: item.title
    val started = item.progress.started()
    val heard = item.watchedMark == WatchedMark.Full
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .then(links.item?.let { open -> Modifier.clickable(role = Role.Button) { open(item.id) } } ?: Modifier)
            .padding(8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(Modifier.size(64.dp).clip(RoundedCornerShape(10.dp))) { MediaImage(images.posters(item.images), item.title, Modifier.fillMaxSize()) }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
            val meta = listOfNotNull(
                item.durationMs?.let { runtimeText(maxOf(1, (it / 60_000).toInt())) },
                started?.let { stringResource(R.string.audioSeries_left, runtimeText(minutesLeft(it))) },
                if (heard) stringResource(R.string.library_status_heard) else null,
            ).joinToString(stringResource(R.string.format_separator))
            if (meta.isNotEmpty()) Text(meta, style = SlouchcatType.caption, color = c.tx3)
            started?.let { ProgressBar(share(it)) }
        }
        if (heard) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 18.dp, tint = c.ok)
        links.play?.let { play -> SlouchcatIconButton(SlouchcatIcon.Play, stringResource(R.string.audioSeries_play, item.title), onClick = { play(item.id) }, tint = c.tx2) }
    }
}

/** A volume number as the server sends it («2», «2,5»). */
private fun number(value: BigDecimal): String = value.stripTrailingZeros().toPlainString()
