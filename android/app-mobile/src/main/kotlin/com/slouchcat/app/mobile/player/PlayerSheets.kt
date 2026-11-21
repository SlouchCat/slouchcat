package com.slouchcat.app.mobile.player

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.slouchcat.app.mobile.media.MediaImage
import com.slouchcat.core.media.ServerImages
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.Note
import com.slouchcat.core.design.SheetActions
import com.slouchcat.core.design.SheetTitle
import com.slouchcat.core.design.SlouchcatButton
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatIconImage
import com.slouchcat.core.design.SlouchcatSheet
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.design.SlouchcatType
import com.slouchcat.core.design.gutter
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.strings.R

/** One sheet of the player (mockup `plMenu`): its title, its choices, «Schliessen». */
@Composable
fun PlayerSheetHost(sheet: PlayerSheet, state: PlayerState, events: PlayerEvents, images: ServerImages?, onClose: () -> Unit) {
    val title = when (sheet) {
        PlayerSheet.Audio -> stringResource(R.string.player_tracks_audio)
        PlayerSheet.Subtitles -> stringResource(R.string.player_tracks_subtitles)
        PlayerSheet.Quality -> stringResource(R.string.player_settings_quality)
        PlayerSheet.Speed -> stringResource(R.string.player_settings_speed)
        PlayerSheet.Episodes -> state.item?.seasonNumber?.let { stringResource(R.string.app_player_season, it.toString()) } ?: stringResource(R.string.app_player_episodes)
        PlayerSheet.Info -> stringResource(R.string.app_player_infoButton)
    }
    SlouchcatSheet(title, onDismiss = onClose) {
        SheetTitle(title)
        when (sheet) {
            PlayerSheet.Audio -> Choices {
                state.audio.forEachIndexed { n, choice ->
                    Choice(trackLabel(choice.track, n), if (choice.track.default) stringResource(R.string.player_option_default) else null, choice.selected) {
                        events.onAudio(choice.track)
                        onClose()
                    }
                }
            }
            PlayerSheet.Subtitles -> Choices {
                Choice(stringResource(R.string.player_tracks_off), null, state.subtitlesOff) {
                    events.onSubtitle(null)
                    onClose()
                }
                state.subtitles.forEachIndexed { n, choice ->
                    Choice(trackLabel(choice.track, n), null, !state.subtitlesOff && choice.selected) {
                        events.onSubtitle(choice.track)
                        onClose()
                    }
                }
            }
            PlayerSheet.Quality -> {
                Choices {
                    val options = state.qualities.filter { it.available || it.quality == Quality.AUTO }
                    options.forEach { option ->
                        Choice(qualityLabel(option, state.info), qualityDetail(option), option.current) {
                            events.onQuality(option.quality)
                            onClose()
                        }
                    }
                }
                Note(stringResource(R.string.app_player_qualityNote), Modifier.padding(horizontal = gutter(), vertical = 8.dp), icon = SlouchcatIcon.Wifi)
            }
            PlayerSheet.Speed -> Choices {
                SPEEDS.forEach { speed ->
                    Choice(speedLabel(speed), null, state.speed == speed) {
                        events.onSpeed(speed)
                        onClose()
                    }
                }
            }
            PlayerSheet.Episodes -> Episodes(state, images, events, onClose)
            PlayerSheet.Info -> Info(state)
        }
        SheetActions {
            SlouchcatButton(stringResource(R.string.common_close), onClick = onClose, kind = ButtonKind.Ghost, block = true, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun Choices(content: @Composable () -> Unit) {
    Column(Modifier.fillMaxWidth().selectableGroup()) { content() }
}

/** `.li.bx-opt`: a radio row, the chosen one in the accent colour with a check mark. */
@Composable
private fun Choice(label: String, detail: String?, selected: Boolean, onClick: () -> Unit) {
    val c = SlouchcatTheme.colors
    Row(
        Modifier
            .fillMaxWidth()
            .defaultMinSize(minHeight = 56.dp)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = gutter(), vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Column(Modifier.weight(1f)) {
            Text(label, style = SlouchcatType.rowTitle, color = if (selected) c.accent else c.tx)
            if (detail != null) Text(detail, style = SlouchcatType.caption, color = c.tx3)
        }
        if (selected) SlouchcatIconImage(SlouchcatIcon.Check, contentDescription = null, size = 22.dp, tint = c.accent)
    }
}

/** `.vp-eps`: the episodes of the season with still, number, title and length; the playing one marked. */
@Composable
private fun Episodes(state: PlayerState, images: ServerImages?, events: PlayerEvents, onClose: () -> Unit) {
    val c = SlouchcatTheme.colors
    val list = state.episodes?.episodes
    if (list == null) {
        Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(Modifier.size(32.dp), color = c.tx, strokeWidth = 3.dp)
        }
        return
    }
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp).selectableGroup()) {
        list.forEach { episode ->
            val current = episode.id == state.item?.id
            val title = episode.episodeNumber?.let { n ->
                if (episode.title.isNotBlank()) stringResource(R.string.app_player_episodeLine, n.toString(), episode.title) else stringResource(R.string.app_player_episodeNumber, n.toString())
            } ?: episode.title
            Row(
                Modifier
                    .fillMaxWidth()
                    .defaultMinSize(minHeight = 64.dp)
                    .clip(RoundedCornerShape(14.dp))
                    .background(if (current) c.s3 else c.elev)
                    .selectable(selected = current, role = Role.RadioButton) {
                        if (!current) events.onEpisode(episode.id)
                        onClose()
                    }
                    .padding(horizontal = 8.dp, vertical = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Box(Modifier.width(112.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(8.dp))) {
                    if (images != null) MediaImage(images.backdrops(episode.images).or(images.posters(episode.images)), episode.title, Modifier.fillMaxSize())
                }
                Column(Modifier.weight(1f)) {
                    Text(title, style = SlouchcatType.rowTitle, color = c.tx, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    val sub = if (current) stringResource(R.string.app_player_nowPlaying) else episode.runtimeMinutes?.let { runtimeText(it) }
                    if (sub != null) {
                        Text(sub, style = SlouchcatType.caption.copy(fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal), color = if (current) c.accent else c.tx3)
                    }
                }
            }
        }
    }
}

/** `.dt-tech.bx-info`: label left, value right, in a framed box. */
@Composable
private fun Info(state: PlayerState) {
    val c = SlouchcatTheme.colors
    val rows = infoRows(state)
    Column(
        Modifier
            .padding(horizontal = gutter())
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .border(BorderStroke(1.dp, c.line), RoundedCornerShape(14.dp)),
    ) {
        rows.forEachIndexed { n, (label, value) ->
            if (n > 0) HorizontalDivider(color = c.line, modifier = Modifier.padding(horizontal = 16.dp))
            Row(
                Modifier
                    .fillMaxWidth()
                    .defaultMinSize(minHeight = 44.dp)
                    .padding(horizontal = 16.dp, vertical = 10.dp)
                    .semantics(mergeDescendants = true) { contentDescription = "$label: $value" },
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Text(label, style = SlouchcatType.bodySmall, color = c.tx2)
                Text(value, style = SlouchcatType.bodySmall.copy(fontWeight = FontWeight.Bold), color = c.tx, textAlign = TextAlign.End, modifier = Modifier.weight(1f))
            }
        }
    }
}
