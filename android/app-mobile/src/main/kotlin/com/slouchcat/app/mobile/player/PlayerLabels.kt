package com.slouchcat.app.mobile.player

import android.text.format.Formatter
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.slouchcat.app.mobile.media.kindName
import com.slouchcat.app.mobile.media.runtimeText
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.QualityOption
import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.Transcoding
import com.slouchcat.core.player.PlayerState
import com.slouchcat.core.player.TrackMatching
import com.slouchcat.core.strings.R
import java.text.NumberFormat
import java.util.Locale

/*
 * The words of the player (web `labels.ts` as the model): titles, track names, qualities, the
 * info sheet. Language and codec names are data, not texts of the app (rule 9): the language
 * comes from the system's table, the codec is the short name viewers know.
 */

/** Short names of the ffprobe codecs viewers know (web `codecNames`); anything else in capitals. */
private val CODEC_NAMES = mapOf(
    "subrip" to "SRT", "ass" to "ASS", "ssa" to "SSA", "webvtt" to "WebVTT", "mov_text" to "MOV-Text",
    "hdmv_pgs_subtitle" to "PGS", "dvd_subtitle" to "VobSub", "eac3" to "E-AC3", "truehd" to "TrueHD",
    "h264" to "H.264", "hevc" to "HEVC", "av1" to "AV1", "vp9" to "VP9", "mpeg2video" to "MPEG-2",
)

fun codecName(codec: String): String = CODEC_NAMES[codec] ?: codec.uppercase(Locale.ROOT)

/** «5.1» for six channels, «2.0» for two. */
private fun channelLayout(channels: Int): String = when (channels) {
    6 -> "5.1"
    8 -> "7.1"
    else -> "$channels.0"
}

/** The name of a language in the app's language («Deutsch» for `ger`, `deu`, `de`). */
@Composable
fun languageName(code: String?): String? {
    val normalized = TrackMatching.normalize(code) ?: return null
    val locale = LocalConfiguration.current.locales[0]
    val name = Locale.forLanguageTag(normalized).getDisplayLanguage(locale)
    return name.takeIf { it.isNotBlank() && it != normalized }?.replaceFirstChar { it.titlecase(locale) }
}

/**
 * The line of a track in a sheet (web `trackLabel`): its title or language, the codec unless the
 * title names it, the channel layout of audio, «erzwungen» and «extern».
 */
@Composable
fun trackLabel(track: Track, position: Int): String {
    val name = track.title ?: languageName(track.language) ?: stringResource(R.string.player_tracks_unknown, (position + 1).toString())
    val codec = codecName(track.codec)
    val parts = mutableListOf(name)
    if (!name.lowercase().contains(codec.lowercase())) parts += codec
    if (track.kind == Track.Kind.AUDIO && track.channels != null && track.channels!! > 0) parts += channelLayout(track.channels!!)
    if (track.forced) parts += stringResource(R.string.player_tracks_forced)
    if (track.source == Track.Source.SIDECAR) parts += stringResource(R.string.player_tracks_external)
    return parts.joinToString(stringResource(R.string.format_separator))
}

/** «S1 · F1 · Titel» for an episode, the title for anything else (mockup `line1`). */
@Composable
fun titleLine(item: ItemRef): String {
    val season = item.seasonNumber
    val episode = item.episodeNumber
    if (season == null || episode == null) return item.title
    val code = stringResource(R.string.home_continue_episode, season.toString(), episode.toString())
    return code + stringResource(R.string.format_separator) + item.title
}

/** The series of an episode; «2026 · Film · 2 h 25 min» for a film (mockup `line2`). */
@Composable
fun subtitleLine(item: ItemRef, durationMs: Long): String? {
    if (item.episodeNumber != null) return item.seriesTitle
    val minutes = item.runtimeMinutes ?: (durationMs / 60_000).toInt().takeIf { it > 0 }
    val parts = listOfNotNull(item.year?.toString(), kindName(item.kind), minutes?.let { runtimeText(it) })
    return parts.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
}

/** «1:05:59» or «4:07» (mockup `tc`). */
fun timecode(ms: Long): String {
    val total = (ms.coerceAtLeast(0) / 1000)
    val h = total / 3600
    val m = (total % 3600) / 60
    val s = total % 60
    return if (h > 0) "%d:%02d:%02d".format(Locale.ROOT, h, m, s) else "%d:%02d".format(Locale.ROOT, m, s)
}

/** «0,5×», «Normal» (mockup `speedText`). */
@Composable
fun speedLabel(speed: Float): String {
    if (speed == 1f) return stringResource(R.string.app_player_normalSpeed)
    val number = NumberFormat.getNumberInstance(LocalConfiguration.current.locales[0]).apply { maximumFractionDigits = 2 }.format(speed.toDouble())
    return stringResource(R.string.player_settings_rateValue, number)
}

/** «8,3 Mbit/s». */
@Composable
fun bitrateLabel(bitsPerSecond: Long): String {
    val number = NumberFormat.getNumberInstance(LocalConfiguration.current.locales[0]).apply { maximumFractionDigits = 1 }
        .format(bitsPerSecond / 1_000_000.0)
    return stringResource(R.string.app_player_info_mbps, number)
}

/** The name of a quality option (web `qualityLabel`). */
@Composable
fun qualityLabel(option: QualityOption, info: PlaybackInfo?): String = when (option.quality) {
    Quality.AUTO -> when {
        option.direct == true -> stringResource(R.string.player_quality_autoOriginal)
        info?.transcoding?.height != null -> stringResource(R.string.player_quality_autoWith, stringResource(R.string.format_height, info.transcoding!!.height.toString()))
        else -> stringResource(R.string.player_quality_auto)
    }
    Quality.ORIGINAL -> stringResource(R.string.player_quality_original)
    else -> option.quality.value
}

/** «8,3 Mbit/s · ca. 3,7 GB pro Stunde» of a quality option with a bit rate. */
@Composable
fun qualityDetail(option: QualityOption): String? {
    val bitrate = option.bitrate?.toLong()?.takeIf { it > 0 } ?: return null
    val perHour = Formatter.formatShortFileSize(LocalContext.current, bitrate / 8 * 3600)
    return stringResource(R.string.player_quality_detail, bitrateLabel(bitrate), perHour)
}

/** The reason the server gave for a remux or a transcode (`Transcoding.reason`). */
@Composable
fun reasonLabel(reason: String): String = stringResource(
    when (reason.substringBefore(',')) {
        "container" -> R.string.player_streamInfo_reasons_container
        "video_codec" -> R.string.player_streamInfo_reasons_video_codec
        "audio_codec" -> R.string.player_streamInfo_reasons_audio_codec
        "hdr" -> R.string.player_streamInfo_reasons_hdr
        "pixel_format" -> R.string.player_streamInfo_reasons_pixel_format
        "keyframes_pending" -> R.string.player_streamInfo_reasons_keyframes_pending
        "keyframes_unavailable" -> R.string.player_streamInfo_reasons_keyframes_unavailable
        "dolby_vision" -> R.string.player_streamInfo_reasons_dolby_vision
        "bandwidth" -> R.string.player_streamInfo_reasons_bandwidth
        "max_height" -> R.string.player_streamInfo_reasons_max_height
        "quality" -> R.string.player_streamInfo_reasons_quality
        "audio_track" -> R.string.player_streamInfo_reasons_audio_track
        "subtitle_burn" -> R.string.player_streamInfo_reasons_subtitle_burn
        "budget_lowered" -> R.string.player_streamInfo_reasons_budget_lowered
        else -> R.string.player_streamInfo_none
    },
)

/** «4K», «1080p», «720p», «SD» by the width of the picture (a scope film of 1920 × 802 is 1080p). */
@Composable
private fun resolutionClass(width: Int): String = stringResource(
    when {
        width >= 3000 -> R.string.format_uhd
        width >= 1800 -> R.string.format_fhd
        width >= 1200 -> R.string.format_hd
        else -> R.string.format_sd
    },
)

/** The rows of the sheet «Wiedergabe-Info» (mockup `info`): label and value. */
@Composable
fun infoRows(state: PlayerState): List<Pair<String, String>> {
    val info = state.info ?: return emptyList()
    val file = info.mediaFile
    val transcoding = info.transcoding
    val none = stringResource(R.string.player_streamInfo_none)
    val mode = when {
        info.mode == PlaybackInfo.Mode.DIRECT || transcoding == null -> stringResource(R.string.app_player_info_direct)
        transcoding.mode == Transcoding.Mode.REMUX -> stringResource(R.string.app_player_info_remux, reasonLabel(transcoding.reason))
        else -> stringResource(
            R.string.app_player_info_transcode,
            transcoding.height?.let { stringResource(R.string.format_height, it.toString()) } ?: transcoding.quality?.value ?: none,
            reasonLabel(transcoding.reason),
        )
    }
    val codec = if (transcoding?.mode == Transcoding.Mode.TRANSCODE) "h264" else file.videoCodec
    val width = state.videoWidth ?: transcoding?.width ?: file.width
    val height = state.videoHeight ?: transcoding?.height ?: file.height
    val separator = stringResource(R.string.format_separator)
    val video = listOfNotNull(
        codec?.let(::codecName),
        width?.let { resolutionClass(it) },
        if (width != null && height != null) stringResource(R.string.format_resolution, width.toString(), height.toString()) else null,
    ).joinToString(separator).ifEmpty { none }
    val audio = state.selectedAudio?.let { trackLabel(it, state.audio.indexOfFirst { c -> c.track.id == it.id }) } ?: none
    val subtitle = state.selectedSubtitle?.let { trackLabel(it, state.subtitles.indexOfFirst { c -> c.track.id == it.id }) }
        ?: stringResource(R.string.player_tracks_off)
    val bitrate = (transcoding?.bitrate?.toLong() ?: file.bitrate)?.takeIf { it > 0 }?.let { bitrateLabel(it) } ?: none
    val container = if (info.mode == PlaybackInfo.Mode.HLS) "HLS" else file.container.uppercase(Locale.ROOT)
    return listOfNotNull(
        stringResource(R.string.app_player_info_mode) to mode,
        stringResource(R.string.player_streamInfo_video) to video,
        stringResource(R.string.player_tracks_audio) to audio,
        stringResource(R.string.player_tracks_subtitles) to subtitle,
        stringResource(R.string.player_streamInfo_bitrate) to bitrate,
        stringResource(R.string.player_streamInfo_container) to container,
        state.capabilities?.name?.let { stringResource(R.string.app_player_info_device) to it },
    )
}
