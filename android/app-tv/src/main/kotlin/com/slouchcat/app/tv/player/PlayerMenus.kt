package com.slouchcat.app.tv.player

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.LocalContentColor
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.slouchcat.app.tv.browse.ControlChip
import com.slouchcat.app.tv.browse.forWidth
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.app.tv.ui.TvButton
import com.slouchcat.app.tv.ui.TvButtonKind
import com.slouchcat.app.tv.ui.TvDimens
import com.slouchcat.app.tv.ui.TvIcon
import com.slouchcat.app.tv.ui.TvImage
import com.slouchcat.app.tv.ui.TvMenuItem
import com.slouchcat.app.tv.ui.TvMotion
import com.slouchcat.app.tv.ui.TvNote
import com.slouchcat.app.tv.ui.TvSideMenu
import com.slouchcat.app.tv.ui.TvType
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.strings.R

/** The speeds of the menu «Tempo» (the web player's list). */
val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)

/**
 * The side menus of the player (mockup `brPlMenu`): a choice closes the menu; the focus starts on
 * the current choice. Play/Pause of the remote still works under a menu (the page takes it).
 */
@Composable
fun PlayerMenuPanel(
    menu: PlayerMenu,
    state: PlayerState,
    sound: SoundState,
    seasons: SeasonsMenu,
    images: ServerImages?,
    events: PlayerEvents,
    onClose: () -> Unit,
) {
    val first = remember(menu) { FocusRequester() }
    fun choose(action: () -> Unit) {
        onClose()
        action()
    }
    when (menu) {
        PlayerMenu.Audio -> TvSideMenu(stringResource(R.string.player_tracks_audio), onDismiss = onClose, initialFocus = first) {
            Column(verticalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
                val chosen = state.audio.indexOfFirst { it.selected }.coerceAtLeast(0)
                state.audio.forEachIndexed { n, choice ->
                    TvMenuItem(
                        trackLabel(choice.track, n),
                        onClick = { choose { events.onAudio(choice.track) } },
                        subtitle = if (choice.track.default) stringResource(R.string.player_option_default) else null,
                        selected = choice.selected,
                        modifier = if (n == chosen) Modifier.focusRequester(first) else Modifier,
                    )
                }
            }
            TvNote(soundNote(sound), Modifier.padding(top = TvDimens.xl), icon = SlouchcatIcon.Audio.vector)
            TvButton(
                stringResource(R.string.tv_player_soundSettings),
                onClick = { choose(events::onSoundSettings) },
                icon = SlouchcatIcon.Settings.vector,
                kind = TvButtonKind.Ghost,
                small = true,
                modifier = Modifier.padding(top = TvDimens.Menu.ActionsTop).then(if (state.audio.isEmpty()) Modifier.focusRequester(first) else Modifier),
            )
        }
        PlayerMenu.Subtitles -> TvSideMenu(stringResource(R.string.player_tracks_subtitles), onDismiss = onClose, initialFocus = first) {
            val direct = state.info?.mode == PlaybackInfo.Mode.DIRECT && state.capabilities?.selectsTracks == true
            Column(verticalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
                TvMenuItem(
                    stringResource(R.string.player_tracks_off),
                    onClick = { choose { events.onSubtitle(null) } },
                    selected = state.subtitlesOff,
                    modifier = if (state.subtitlesOff) Modifier.focusRequester(first) else Modifier,
                )
                state.subtitles.forEachIndexed { n, choice ->
                    val on = choice.selected && !state.subtitlesOff
                    val picture = isPictureSubtitle(choice.track.codec)
                    TvMenuItem(
                        trackLabel(choice.track, n),
                        onClick = { choose { events.onSubtitle(choice.track) } },
                        subtitle = when {
                            picture && direct -> stringResource(R.string.tv_player_subtitleImage)
                            picture -> stringResource(R.string.player_option_burnIn)
                            else -> null
                        },
                        selected = on,
                        modifier = if (on) Modifier.focusRequester(first) else Modifier,
                    )
                }
            }
        }
        PlayerMenu.Quality -> TvSideMenu(stringResource(R.string.player_settings_quality), onDismiss = onClose, initialFocus = first) {
            val current = state.qualities.indexOfFirst { it.current }.coerceAtLeast(0)
            Column(verticalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
                state.qualities.forEachIndexed { n, option ->
                    val converted = option.quality != Quality.AUTO && option.quality != Quality.ORIGINAL
                    TvMenuItem(
                        qualityLabel(option, state.info),
                        onClick = { choose { events.onQuality(option.quality) } },
                        subtitle = qualityDetail(option) ?: if (converted) stringResource(R.string.tv_player_qualityConverted) else null,
                        selected = option.current,
                        enabled = option.available,
                        modifier = if (n == current) Modifier.focusRequester(first) else Modifier,
                    )
                }
            }
        }
        PlayerMenu.Speed -> TvSideMenu(stringResource(R.string.player_settings_speed), onDismiss = onClose, initialFocus = first) {
            Column(verticalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
                SPEEDS.forEach { speed ->
                    val on = speed == state.speed
                    TvMenuItem(
                        speedLabel(speed),
                        onClick = { choose { events.onSpeed(speed) } },
                        selected = on,
                        modifier = if (on) Modifier.focusRequester(first) else Modifier,
                    )
                }
            }
        }
        PlayerMenu.Episodes -> {
            val item = state.item
            val series = item?.seriesTitle
            TvSideMenu(
                if (series != null) stringResource(R.string.tv_player_episodesOf, series) else stringResource(R.string.app_player_episodes),
                onDismiss = onClose,
                initialFocus = first,
            ) {
                Episodes(state, seasons, images, events, first, onClose)
            }
        }
    }
}

/** The line about the sound under the tracks (mockup `.tnote` of the menu «Ton»). */
@Composable
private fun soundNote(sound: SoundState): String {
    val output = sound.output
    val separator = stringResource(R.string.format_listSeparator)
    val formats = { list: List<String> -> list.joinToString(separator) { soundName(it) } }
    return when (val setting = sound.setting) {
        AudioSetting.Stereo -> stringResource(R.string.tv_player_soundForcedStereo)
        is AudioSetting.Manual -> {
            val chosen = reported(setting, output ?: SoundOutput(emptyList(), 2))
            stringResource(R.string.tv_player_soundManual, formats(chosen.passthrough).ifEmpty { channelLayout(chosen.channels) }, channelLayout(chosen.channels))
        }
        AudioSetting.Auto -> when {
            output == null -> stringResource(R.string.tv_player_soundStereoOnly)
            output.passthrough.isNotEmpty() -> stringResource(R.string.tv_player_soundDetected, formats(output.passthrough), channelLayout(output.channels))
            output.channels > 2 -> stringResource(R.string.tv_player_soundPcm, channelLayout(output.channels))
            else -> stringResource(R.string.tv_player_soundStereoOnly)
        }
    }
}

/** The menu «Folgen» (mockup `brEpsMenu`): the seasons as chips, the episodes with a picture, the playing one marked. */
@Composable
private fun Episodes(state: PlayerState, seasons: SeasonsMenu, images: ServerImages?, events: PlayerEvents, first: FocusRequester, onClose: () -> Unit) {
    val playing = state.item
    val shownSeason = seasons.shown
    val list: List<ItemRef>? = if (shownSeason == null) state.episodes?.episodes else seasons.episodes[shownSeason]
    if (seasons.seasons.size > 1) {
        Row(Modifier.padding(bottom = TvDimens.l), horizontalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
            seasons.seasons.forEach { season ->
                val on = season.id == (shownSeason ?: playing?.parentId)
                ControlChip(
                    season.seasonNumber?.let { stringResource(R.string.app_player_season, it.toString()) } ?: season.title,
                    icon = SlouchcatIcon.List.vector,
                    onClick = { events.onSeason(season.id) },
                    on = on,
                )
            }
        }
    }
    if (list == null) {
        com.slouchcat.app.tv.ui.TvBusy(stringResource(R.string.common_loading), Modifier.focusRequester(first))
        return
    }
    Column(verticalArrangement = Arrangement.spacedBy(P.ChipsGap)) {
        val start = list.indexOfFirst { it.id == playing?.id }.takeIf { it >= 0 } ?: 0
        list.forEachIndexed { n, episode ->
            val current = episode.id == playing?.id
            EpisodeRow(
                episode = episode,
                current = current,
                images = images,
                onClick = {
                    onClose()
                    if (!current) events.onEpisode(episode.id)
                },
                modifier = if (n == start) Modifier.focusRequester(first) else Modifier,
            )
        }
    }
}

/** `.br-mep`: a picture of the episode, «2. Titel», the runtime and «läuft gerade» or «gesehen». */
@Composable
private fun EpisodeRow(episode: ItemRef, current: Boolean, images: ServerImages?, onClick: () -> Unit, modifier: Modifier) {
    val c = SlouchcatTheme.colors
    val shape = RoundedCornerShape(TvDimens.radiusMenuItem)
    Surface(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().heightIn(min = P.Episode).semantics { if (current) selected = true },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = if (current) c.s2 else Color.Transparent,
            contentColor = c.tx,
            focusedContainerColor = c.tx,
            focusedContentColor = c.bg,
            pressedContainerColor = c.tx,
            pressedContentColor = c.bg,
        ),
        border = ClickableSurfaceDefaults.border(focusedBorder = Border(BorderStroke(TvDimens.borderThick, c.tx), shape = shape)),
        scale = ClickableSurfaceDefaults.scale(focusedScale = TvMotion.focusScaleSmall),
    ) {
        val focused = LocalContentColor.current == c.bg
        Row(
            Modifier.fillMaxWidth().padding(horizontal = TvDimens.Menu.ItemPad, vertical = TvDimens.s),
            horizontalArrangement = Arrangement.spacedBy(TvDimens.l),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val image = images?.backdrops(episode.images)?.forWidth(P.EpisodeImage)
            Box(Modifier.width(P.EpisodeImage).aspectRatio(EPISODE_RATIO).clip(RoundedCornerShape(P.EpisodeRadius)).background(c.s3)) {
                if (image != null) TvImage(image, Modifier.fillMaxSize())
            }
            Column(Modifier.weight(1f)) {
                val number = episode.episodeNumber
                Text(
                    if (number != null) stringResource(R.string.app_player_episodeLine, number.toString(), episode.title) else episode.title,
                    style = TvType.largeStrong,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                val line = listOfNotNull(
                    episode.runtimeMinutes?.let { runtimeText(it) },
                    when {
                        current -> stringResource(R.string.app_player_nowPlaying)
                        episode.watched == true -> stringResource(R.string.tv_player_seen)
                        else -> null
                    },
                ).joinToString(stringResource(R.string.format_separator))
                if (line.isNotEmpty()) Text(line, style = TvType.small, color = if (focused) c.bg else c.tx2)
            }
            if (current) TvIcon(SlouchcatIcon.Play, TvDimens.Menu.Icon)
        }
    }
}

private const val EPISODE_RATIO = 16f / 9f
