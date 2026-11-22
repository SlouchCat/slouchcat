package com.slouchcat.app.tv.player

import android.text.format.Formatter
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.slouchcat.app.tv.browse.kindName
import com.slouchcat.app.tv.browse.runtimeText
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
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
 * The words of the TV player (the phone app's `PlayerLabels.kt` and the web `labels.ts` as the
 * model; a copy until a shared module for app texts exists): track names, qualities, the info
 * panel. Language and codec names are data, not texts of the app (rule 9).
 */

/** Short names of the ffprobe codecs viewers know; anything else in capitals. */
private val CODEC_NAMES = mapOf(
    "subrip" to "SRT", "ass" to "ASS", "ssa" to "SSA", "webvtt" to "WebVTT", "mov_text" to "MOV-Text",
    "hdmv_pgs_subtitle" to "PGS", "dvd_subtitle" to "VobSub", "eac3" to "E-AC3", "truehd" to "TrueHD",
    "h264" to "H.264", "hevc" to "HEVC", "av1" to "AV1", "vp9" to "VP9", "mpeg2video" to "MPEG-2",
)

/** The sound formats as a sound system names them (mockup `MD_FMT`): product names, data. */
private val SOUND_NAMES = mapOf(
    "aac" to "AAC", "ac3" to "Dolby Digital", "eac3" to "Dolby Digital Plus", "dts" to "DTS", "truehd" to "Dolby TrueHD",
    "flac" to "FLAC", "opus" to "Opus", "mp3" to "MP3", "vorbis" to "Vorbis", PCM to "PCM",
)

/** The name of decoded multichannel sound in [soundName]. */
const val PCM = "pcm"

/** The HDR formats of a file (`MediaFileInfo.hdr`) by their names: data. */
private val HDR_NAMES = mapOf(
    MediaFileInfo.Hdr.HDR10 to "HDR10", MediaFileInfo.Hdr.HDR10PLUS to "HDR10+", MediaFileInfo.Hdr.DOLBY_VISION to "Dolby Vision", MediaFileInfo.Hdr.HLG to "HLG",
)

fun codecName(codec: String): String = CODEC_NAMES[codec] ?: codec.uppercase(Locale.ROOT)

fun soundName(codec: String): String = SOUND_NAMES[codec] ?: codecName(codec)

fun hdrName(hdr: MediaFileInfo.Hdr): String = HDR_NAMES[hdr] ?: hdr.value

/** «5.1» for six channels, «2.0» for two (data: the way channel layouts are written). */
fun channelLayout(channels: Int): String = when (channels) {
    6 -> "5.1"
    8 -> "7.1"
    else -> "$channels.0"
}

/** Whether a subtitle codec is a picture (PGS, DVD), not text. */
fun isPictureSubtitle(codec: String): Boolean = codec == "hdmv_pgs_subtitle" || codec == "dvd_subtitle"

/** The name of a language in the app's language («Deutsch» for `ger`, `deu`, `de`). */
@Composable
fun languageName(code: String?): String? {
    val normalized = TrackMatching.normalize(code) ?: return null
    val locale = LocalConfiguration.current.locales[0]
    val name = Locale.forLanguageTag(normalized).getDisplayLanguage(locale)
    return name.takeIf { it.isNotBlank() && it != normalized }?.replaceFirstChar { it.titlecase(locale) }
}

/** The line of a track in a menu: its title or language, the codec unless the title names it, the channel layout of audio, «erzwungen», «extern». */
@Composable
fun trackLabel(track: Track, position: Int): String {
    val name = track.title ?: languageName(track.language) ?: stringResource(R.string.player_tracks_unknown, (position + 1).toString())
    val codec = if (track.kind == Track.Kind.AUDIO) soundName(track.codec) else codecName(track.codec)
    val parts = mutableListOf(name)
    if (!name.lowercase().contains(codec.lowercase())) parts += codec
    val channels = track.channels
    if (track.kind == Track.Kind.AUDIO && channels != null && channels > 0) parts += channelLayout(channels)
    if (track.forced) parts += stringResource(R.string.player_tracks_forced)
    if (track.source == Track.Source.SIDECAR) parts += stringResource(R.string.player_tracks_external)
    return parts.joinToString(stringResource(R.string.format_separator))
}

/** «S1 · F1 · Titel» for an episode, the title for anything else (mockup `brPlHtml`). */
@Composable
fun titleLine(item: ItemRef): String {
    val season = item.seasonNumber
    val episode = item.episodeNumber
    if (season == null || episode == null) return item.title
    val code = stringResource(R.string.home_continue_episode, season.toString(), episode.toString())
    return code + stringResource(R.string.format_separator) + item.title
}

/** The series of an episode; «2026 · Film» for a film (mockup `over`). */
@Composable
fun overLine(item: ItemRef, durationMs: Long): String? {
    if (item.episodeNumber != null) return item.seriesTitle
    val minutes = item.runtimeMinutes ?: (durationMs / MS_PER_MINUTE).toInt().takeIf { it > 0 }
    val parts = listOfNotNull(item.year?.toString(), kindName(item.kind), minutes?.let { runtimeText(it) })
    return parts.takeIf { it.isNotEmpty() }?.joinToString(stringResource(R.string.format_separator))
}

/** «1:05:59» or «4:07» (mockup `brTc`). */
fun timecode(ms: Long): String {
    val total = ms.coerceAtLeast(0) / MS_PER_SECOND
    val h = total / SECONDS_PER_HOUR
    val m = (total % SECONDS_PER_HOUR) / SECONDS_PER_MINUTE
    val s = total % SECONDS_PER_MINUTE
    return if (h > 0) "%d:%02d:%02d".format(Locale.ROOT, h, m, s) else "%d:%02d".format(Locale.ROOT, m, s)
}

/** «0,5×», «Normal». */
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
        .format(bitsPerSecond / BITS_PER_MEGABIT)
    return stringResource(R.string.app_player_info_mbps, number)
}

/** The name of a quality option (web `qualityLabel`). */
@Composable
fun qualityLabel(option: QualityOption, info: PlaybackInfo?): String = when (option.quality) {
    Quality.AUTO -> {
        val height = info?.transcoding?.height
        when {
            option.direct == true -> stringResource(R.string.player_quality_autoOriginal)
            height != null -> stringResource(R.string.player_quality_autoWith, stringResource(R.string.format_height, height.toString()))
            else -> stringResource(R.string.player_quality_auto)
        }
    }
    Quality.ORIGINAL -> stringResource(R.string.player_quality_original)
    else -> option.quality.value
}

/** «8,3 Mbit/s · ca. 3,7 GB pro Stunde» of a quality option with a bit rate. */
@Composable
fun qualityDetail(option: QualityOption): String? {
    val bitrate = option.bitrate?.toLong()?.takeIf { it > 0 } ?: return null
    val perHour = Formatter.formatShortFileSize(LocalContext.current, bitrate / BITS_PER_BYTE * SECONDS_PER_HOUR)
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
        width >= UHD_WIDTH -> R.string.format_uhd
        width >= FHD_WIDTH -> R.string.format_fhd
        width >= HD_WIDTH -> R.string.format_hd
        else -> R.string.format_sd
    },
)

/**
 * The rows of the panel «Wiedergabe» (mockup `brPlInfoHtml`): mode, video, HDR, sound, container,
 * bit rate, subtitles, device. Over HLS the sound is the format Media3 plays ([sound]), the
 * track of the file otherwise.
 */
@Composable
fun infoRows(state: PlayerState, sound: SoundState? = null): List<Pair<String, String>> {
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
    val transcoded = transcoding?.mode == Transcoding.Mode.TRANSCODE
    val codec = if (transcoded) "h264" else file.videoCodec
    val width = state.videoWidth ?: transcoding?.width ?: file.width
    val height = state.videoHeight ?: transcoding?.height ?: file.height
    val separator = stringResource(R.string.format_separator)
    val video = listOfNotNull(
        codec?.let(::codecName),
        if (width != null && height != null) stringResource(R.string.format_resolution, width.toString(), height.toString()) else null,
        width?.let { resolutionClass(it) },
    ).joinToString(separator).ifEmpty { none }
    val hdr = file.hdr
    val hdrText = when {
        hdr == null -> stringResource(R.string.tv_player_hdrNone)
        transcoding?.tonemap == true -> stringResource(R.string.tv_player_hdrTonemapped, hdrName(hdr))
        else -> hdrName(hdr)
    }
    val outgoing = sound?.playing?.takeIf { info.mode == PlaybackInfo.Mode.HLS && it.codec != null }
    // Over HLS the server may convert the sound: the format Media3 plays, not the file's track (review m-5).
    val audio = outgoing?.let { o -> listOfNotNull(soundName(o.codec!!), o.channels?.let(::channelLayout)).joinToString(separator) }
        ?: state.selectedAudio?.let { trackLabel(it, state.audio.indexOfFirst { c -> c.track.id == it.id }) } ?: none
    val subtitle = state.selectedSubtitle?.let { trackLabel(it, state.subtitles.indexOfFirst { c -> c.track.id == it.id }) }
        ?: stringResource(R.string.player_tracks_off)
    val bitrate = (transcoding?.bitrate?.toLong() ?: file.bitrate)?.takeIf { it > 0 }?.let { bitrateLabel(it) } ?: none
    val container = if (info.mode == PlaybackInfo.Mode.HLS) "HLS" else file.container.uppercase(Locale.ROOT)
    return listOfNotNull(
        stringResource(R.string.app_player_info_mode) to mode,
        stringResource(R.string.player_streamInfo_video) to video,
        stringResource(R.string.tv_player_hdr) to hdrText,
        stringResource(R.string.player_tracks_audio) to audio,
        stringResource(R.string.player_streamInfo_container) to container,
        stringResource(R.string.player_streamInfo_bitrate) to bitrate,
        stringResource(R.string.player_tracks_subtitles) to subtitle,
        state.capabilities?.name?.let { stringResource(R.string.app_player_info_device) to it },
    )
}

private const val MS_PER_SECOND = 1000L
private const val MS_PER_MINUTE = 60_000L
private const val SECONDS_PER_MINUTE = 60L
private const val SECONDS_PER_HOUR = 3600L
private const val BITS_PER_BYTE = 8L
private const val BITS_PER_MEGABIT = 1_000_000.0
private const val UHD_WIDTH = 3000
private const val FHD_WIDTH = 1800
private const val HD_WIDTH = 1200
