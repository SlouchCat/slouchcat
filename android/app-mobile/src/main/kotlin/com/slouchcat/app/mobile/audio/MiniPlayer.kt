package com.slouchcat.app.mobile.audio

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.paneTitle
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconButton
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.RadioError
import com.slouchcat.core.strings.R

/** What the small player draws: title, second line, cover, whether it plays. */
data class MiniUi(val title: String, val line: String?, val cover: ImageSet, val playing: Boolean, val retry: Boolean = false)

/** The two lines of [now] for the small player and the notification of a station. */
@Composable
fun miniUi(now: AudioNow, cover: ImageSet): MiniUi = when (now) {
    is AudioNow.Title -> {
        val item = now.state.item
        MiniUi(
            title = item?.title.orEmpty(),
            line = item?.author ?: item?.seriesTitle,
            cover = cover,
            playing = now.state.playing || (now.state.buffering && !now.state.ended && now.state.error == null),
            retry = now.state.error != null,
        )
    }
    is AudioNow.Radio -> {
        val radio = now.state
        val entry = radio.now
        val error = radio.error
        val line = when {
            error != null -> radioErrorText(error)
            radio.starting -> stringResource(R.string.radio_player_starting)
            entry?.artist != null && entry.title != null -> stringResource(R.string.radio_player_artistTitle, entry.artist.orEmpty(), entry.title.orEmpty())
            entry != null -> entry.text
            !radio.wantsToPlay -> stringResource(R.string.radio_player_paused)
            else -> stringResource(R.string.radio_player_live)
        }
        MiniUi(radio.station?.name.orEmpty(), line, cover, playing = radio.wantsToPlay && radio.error == null, retry = radio.error != null)
    }
    is AudioNow.Restored -> MiniUi(now.last.title, now.last.byline ?: stringResource(R.string.app_audio_restored), cover, playing = false)
}

/**
 * The small player (mockup `drawMini`, photo handy 66): above the bottom bar on the phone, at the
 * foot of the side column from 700 dp. Cover, title and second line, play or pause, and the cross
 * that ends the playback (web «Radio beenden»). A tap on the rest opens the audio player or the
 * radio ([onOpen]).
 */
@Composable
fun MiniPlayer(ui: MiniUi, onOpen: () -> Unit, onToggle: () -> Unit, onStop: () -> Unit, modifier: Modifier = Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(14.dp)
    val region = stringResource(R.string.radio_player_region)
    val open = stringResource(R.string.app_audio_open, ui.title)
    Row(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 6.dp)
            .clip(shape)
            .background(c.s2)
            .border(BorderStroke(1.dp, c.line), shape)
            .semantics { paneTitle = region }
            .heightIn(min = 64.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            Modifier
                .weight(1f)
                .clickable(role = Role.Button, onClickLabel = open, onClick = onOpen)
                .padding(8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Box(Modifier.size(44.dp).clip(RoundedCornerShape(8.dp))) { MediaImage(ui.cover, ui.title, Modifier.fillMaxSize()) }
            Column(Modifier.weight(1f)) {
                Text(ui.title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 1, overflow = TextOverflow.Ellipsis)
                ui.line?.let { Text(it, style = SlouchcatType.caption, color = c.tx2, maxLines = 1, overflow = TextOverflow.Ellipsis) }
            }
        }
        val toggle = when {
            ui.retry -> stringResource(R.string.radio_player_retry)
            ui.playing -> stringResource(R.string.player_pause)
            else -> stringResource(R.string.player_play)
        }
        SlouchcatIconButton(if (ui.playing) SlouchcatIcon.Pause else if (ui.retry) SlouchcatIcon.Refresh else SlouchcatIcon.Play, toggle, onToggle)
        SlouchcatIconButton(SlouchcatIcon.Close, stringResource(R.string.app_audio_stop), onStop)
    }
}

@Composable
fun radioErrorText(error: RadioError): String = stringResource(
    when (error) {
        RadioError.WatchLimit -> R.string.app_radio_error_watchLimit
        RadioError.Capacity -> R.string.app_radio_error_capacity
        RadioError.Unreachable -> R.string.app_radio_error_unreachable
        RadioError.NotAudio -> R.string.app_radio_error_notAudio
        RadioError.Ended -> R.string.app_radio_error_ended
        RadioError.Network -> R.string.app_radio_error_network
        RadioError.Failed -> R.string.app_radio_error_failed
    },
)
