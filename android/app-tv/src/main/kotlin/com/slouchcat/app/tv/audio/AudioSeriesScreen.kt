package com.slouchcat.app.tv.audio

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.KindLine
import com.slouchcat.app.tv.browse.pluralCount
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.common.LoadableContent
import com.slouchcat.app.tv.shell.LocalContentStart
import com.slouchcat.app.tv.ui.StartFocus
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.app.tv.ui.focusKey
import com.slouchcat.app.tv.ui.rememberFocusMemory
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.minutesLeft
import com.slouchcat.core.media.share
import com.slouchcat.core.media.started
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.strings.R

/** Where the series page leads; `null`: that page is not registered. */
data class SeriesLinks(
    val retry: () -> Unit,
    /** A volume's detail page. */
    val item: ((String) -> Unit)?,
    /** The audio player for a volume («Weiterhören: Band 2»). */
    val play: ((String) -> Unit)?,
)

/** Test tags and focus keys of the series page. */
object SeriesKeys {
    const val NEXT = "series-next"
    const val RETRY = "series-retry"
    fun volume(id: String) = "volume-$id"
}

/**
 * The series of an audiobook (web `SeriesPage`; the mockup has no page of its own, so it is built
 * like the book page): on the left the cover and «Weiterhören: Band 2», on the right «Reihe», the
 * name, author and narrator, «4 Bände · 38 Std.» and the volumes in their order (number or check,
 * title, progress, length); OK opens a volume's page.
 */
@Composable
fun AudioSeriesScreen(state: Loadable<AudioSeries>, images: ServerImages, links: SeriesLinks) {
    val focus = rememberFocusMemory()
    val c = SlouchcatTheme.colors
    val series = (state as? Loadable.Ready)?.value
    if (series == null) {
        LoadableContent(
            state,
            onRetry = links.retry,
            modifier = Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop),
            retryModifier = Modifier.focusKey(focus, SeriesKeys.RETRY).testTag(SeriesKeys.RETRY),
            empty = {
                Column(Modifier.padding(start = LocalContentStart.current, top = TvDimens.Browse.DetailTop), verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
                    Text(stringResource(R.string.audioSeries_notFound_title), style = TvType.h1)
                    Text(stringResource(R.string.audioSeries_notFound_body), style = TvType.large, color = c.tx2)
                }
            },
        ) {}
        focus.StartFocus(SeriesKeys.RETRY, state::class)
        return
    }
    val next = series.next
    Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.spacedBy(A.DetailGap)) {
        Column(Modifier.padding(start = LocalContentStart.current, top = TvDimens.safeY).width(A.DetailCover), verticalArrangement = Arrangement.spacedBy(TvDimens.l)) {
            Cover(images.posters((series.cover ?: series.volumes.firstOrNull()?.item)?.images), A.DetailCover)
            val play = links.play
            if (next != null && play != null) {
                val volume = series.volumes.firstOrNull { it.item.id == next.id }?.seriesIndex
                TvButton(
                    if (volume != null) stringResource(R.string.audioSeries_continue, volume.stripTrailingZeros().toPlainString()) else stringResource(R.string.audioSeries_continueTitle, next.title),
                    onClick = { play(next.id) },
                    icon = SlouchcatIcon.Play.vector,
                    kind = TvButtonKind.Primary,
                    modifier = Modifier.focusKey(focus, SeriesKeys.NEXT).testTag(SeriesKeys.NEXT),
                )
            }
        }
        LazyColumn(
            state = rememberLazyListState(),
            modifier = Modifier.weight(1f).fillMaxHeight(),
            contentPadding = PaddingValues(top = TvDimens.safeY, end = TvDimens.safeX, bottom = TvDimens.safeY),
        ) {
            item(key = "head") {
                Column {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(TvDimens.s)) {
                        TvIcon(SlouchcatIcon.Book, TvDimens.Browse.Icon, tint = c.accent)
                        KindLine(stringResource(R.string.audioSeries_kicker))
                    }
                    Text(series.name, style = TvType.xl, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = TvDimens.s).semantics { heading() })
                    series.author?.takeIf { it.isNotBlank() }?.let { Text(it, style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.xs)) }
                    val facts = listOfNotNull(
                        pluralCount(R.plurals.audioSeries_volumes, series.volumes.size),
                        (series.durationMs / MINUTE_MS).toInt().takeIf { it > 0 }?.let { runtimeText(it) },
                        series.narrator?.takeIf { it.isNotBlank() }?.let { stringResource(R.string.audioSeries_narratedBy, it) },
                    )
                    Text(facts.joinToString(stringResource(R.string.format_separator)), style = TvType.large, color = c.tx2, modifier = Modifier.padding(top = TvDimens.s))
                    Text(stringResource(R.string.audioSeries_list), style = TvType.h2, modifier = Modifier.fillMaxWidth().padding(top = A.ListTitleTop, bottom = TvDimens.m).semantics { heading() })
                }
            }
            itemsIndexed(series.volumes, key = { _, volume -> volume.item.id }) { index, volume ->
                val item = volume.item
                val heard = item.watchedMark == WatchedMark.Full
                val begun = item.progress.started()
                val number = volume.seriesIndex?.stripTrailingZeros()?.toPlainString()
                val state = when {
                    heard -> stringResource(R.string.tv_audio_heard)
                    begun != null -> stringResource(R.string.app_item_left, runtimeText(minutesLeft(begun)))
                    else -> null
                }
                AudioRow(
                    item.title,
                    onClick = { links.item?.invoke(item.id) },
                    subtitle = state,
                    label = number?.let { stringResource(R.string.audioSeries_volumeLabel, it, item.title) } ?: item.title,
                    end = (item.durationMs?.let { (it / MINUTE_MS).toInt() } ?: item.runtimeMinutes)?.takeIf { it > 0 }?.let { runtimeText(it) },
                    strong = !heard,
                    progress = begun?.let(::share),
                    modifier = Modifier.focusKey(focus, SeriesKeys.volume(item.id)).testTag(SeriesKeys.volume(item.id)),
                ) {
                    if (heard) HeardMark() else NumberMark(number?.toIntOrNull() ?: (index + 1))
                }
            }
        }
    }
    focus.StartFocus(if (next != null && links.play != null) SeriesKeys.NEXT else series.volumes.firstOrNull()?.let { SeriesKeys.volume(it.item.id) } ?: SeriesKeys.RETRY, series.id)
}

private const val MINUTE_MS = 60_000L
