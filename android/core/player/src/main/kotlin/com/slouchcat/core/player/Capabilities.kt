package com.slouchcat.core.player

import android.media.MediaCodecInfo.CodecProfileLevel
import com.slouchcat.core.api.generated.models.PlaybackCapabilities

/*
 * What the device reports to the server (concept 27 8.2, concept 26 5.2/5.3, ADR 0041). The server
 * decides between direct play, remux and transcode (rule 12); the app only says what the device
 * can. The rules are pure (tables in CapabilitiesTest); DeviceFactsReader reads the facts from
 * MediaCodecList, the audio outputs and the display.
 */

/** One decoder of the device for one MIME type (a `MediaCodecInfo` and one of its types). */
data class DecoderInfo(
    val mime: String,
    /** `CodecProfileLevel.profile` values the decoder names (HEVC Main 10: [CodecProfileLevel.HEVCProfileMain10]). */
    val profiles: Set<Int> = emptySet(),
)

/** HDR formats the display shows. */
enum class HdrType { Hdr10, Hlg, DolbyVision, Hdr10Plus }

/** The largest mode of the display in landscape and the HDR formats it shows. */
data class DisplayInfo(val maxWidth: Int, val maxHeight: Int, val hdrTypes: Set<HdrType> = emptySet())

/** `PlaybackCapabilities.type` of the app. */
enum class DeviceType(val value: PlaybackCapabilities.Type) {
    Phone(PlaybackCapabilities.Type.PHONE),
    Tablet(PlaybackCapabilities.Type.TABLET),
    Tv(PlaybackCapabilities.Type.TV),
}

/**
 * The facts of a device the rules work from. [passthrough]: ffprobe names of the encodings the
 * audio output takes as they are (HDMI, ARC); [outputChannels]: the channels of the widest PCM
 * output (2 for the speaker and every headphone).
 */
data class DeviceFacts(
    val decoders: List<DecoderInfo>,
    val passthrough: Set<String>,
    val outputChannels: Int,
    val display: DisplayInfo,
    val type: DeviceType,
    val name: String,
)

/**
 * The sound setting of both apps (concept 26 5.3): automatic, always stereo, or by hand. The phone
 * draws no page for it in the first edition (the mockup has none); the TV app does.
 */
sealed interface AudioSetting {
    /** What the device and the output can (the rules below). */
    data object Auto : AudioSetting

    /** Only what the device decodes, without Dolby and DTS, two channels. */
    data object Stereo : AudioSetting

    /** [formats]: ffprobe names the user ticked; [channels]: 2, 6 or 8. */
    data class Manual(val formats: Set<String>, val channels: Int) : AudioSetting
}

/**
 * What one playback rules out after a failure: an audio codec Media3 played silently, a video
 * codec whose decoder failed (and with it HDR). It narrows the next start of the same playback.
 */
data class Narrowing(
    val audio: Set<String> = emptySet(),
    val video: Set<String> = emptySet(),
    val noHdr: Boolean = false,
    /** The groups of the file did not match the server's tracks: the server picks the tracks (no `selectsTracks`). */
    val noTrackChoice: Boolean = false,
) {
    operator fun plus(other: Narrowing) =
        Narrowing(audio + other.audio, video + other.video, noHdr || other.noHdr, noTrackChoice || other.noTrackChoice)
}

object CapabilityRules {
    /** Containers ExoPlayer plays directly (names of `library/probe.go`). */
    val CONTAINERS = listOf("mp4", "mkv", "webm", "ts", "m2ts", "mov")

    /** Subtitle formats Media3 draws itself from the file (concept 26 stage 0 point 3). */
    val SUBTITLES = listOf("subrip", "ass", "webvtt", "hdmv_pgs_subtitle", "dvd_subtitle")

    /** ffprobe name to the MIME type of its decoder. */
    val VIDEO_MIMES = linkedMapOf(
        "h264" to "video/avc", "hevc" to "video/hevc", "av1" to "video/av01", "vp9" to "video/x-vnd.on2.vp9",
        "mpeg2video" to "video/mpeg2",
    )
    val AUDIO_MIMES = linkedMapOf(
        "aac" to "audio/mp4a-latm", "mp3" to "audio/mpeg", "opus" to "audio/opus", "flac" to "audio/flac",
        "vorbis" to "audio/vorbis", "ac3" to "audio/ac3", "eac3" to "audio/eac3", "dts" to "audio/vnd.dts",
        "truehd" to "audio/true-hd",
    )

    /** Encodings an output may take as they are. */
    val PASSTHROUGH = listOf("ac3", "eac3", "dts", "truehd")

    /** The formats «von Hand» offers (concept 26 5.3). */
    val MANUAL_FORMATS = listOf("aac", "ac3", "eac3", "dts", "truehd", "flac", "opus")

    /** ffprobe name of an audio MIME type (Media3 `Format.sampleMimeType`), `null` when unknown. */
    fun audioCodecOf(mime: String?): String? = AUDIO_MIMES.entries.firstOrNull { it.value.equals(mime, ignoreCase = true) }?.key
        ?: if (mime.equals("audio/eac3-joc", ignoreCase = true)) "eac3" else null

    /** Video codecs the device decodes; HEVC only with Main 10 (most HEVC files are 10 bit, K27 stage 0 point 4). */
    fun video(facts: DeviceFacts): List<String> = VIDEO_MIMES.filter { (codec, mime) ->
        val decoders = facts.decoders.filter { it.mime.equals(mime, ignoreCase = true) }
        if (codec == "hevc") decoders.any { CodecProfileLevel.HEVCProfileMain10 in it.profiles } else decoders.isNotEmpty()
    }.keys.toList()

    /** Audio codecs a decoder of the device plays. */
    fun decodedAudio(facts: DeviceFacts): List<String> =
        AUDIO_MIMES.filter { (_, mime) -> facts.decoders.any { it.mime.equals(mime, ignoreCase = true) } }.keys.toList()

    /** 2, 6 or 8: the widest output, 6 for any passthrough, 8 for TrueHD passthrough. */
    fun channels(facts: DeviceFacts): Int {
        val widest = maxOf(
            facts.outputChannels,
            if (facts.passthrough.isNotEmpty()) 6 else 0,
            if ("truehd" in facts.passthrough) 8 else 0,
        )
        return when {
            widest >= 8 -> 8
            widest >= 6 -> 6
            else -> 2
        }
    }

    fun build(facts: DeviceFacts, setting: AudioSetting = AudioSetting.Auto, narrowing: Narrowing = Narrowing()): PlaybackCapabilities {
        val video = video(facts) - narrowing.video
        val decoded = decodedAudio(facts)
        val passthrough = PASSTHROUGH.filter { it in facts.passthrough }
        var audio = (decoded + passthrough).distinct()
        var channels = channels(facts)
        when (setting) {
            AudioSetting.Auto -> Unit
            AudioSetting.Stereo -> {
                audio = decoded.filter { it !in PASSTHROUGH }
                channels = 2
            }
            is AudioSetting.Manual -> {
                audio = MANUAL_FORMATS.filter { it in setting.formats } + setting.formats.filter { it !in MANUAL_FORMATS }.sorted()
                channels = setting.channels.takeIf { it == 6 || it == 8 } ?: 2
            }
        }
        audio = audio - narrowing.audio
        val hdrDisplay = HdrType.Hdr10 in facts.display.hdrTypes || HdrType.Hlg in facts.display.hdrTypes
        return PlaybackCapabilities(
            video = video,
            audio = audio,
            hls = PlaybackCapabilities.Hls.MSE,
            containers = CONTAINERS,
            maxAudioChannels = when (channels) {
                8 -> PlaybackCapabilities.MaxAudioChannels._8
                6 -> PlaybackCapabilities.MaxAudioChannels._6
                else -> PlaybackCapabilities.MaxAudioChannels._2
            },
            selectsTracks = !narrowing.noTrackChoice,
            subtitles = SUBTITLES.takeUnless { narrowing.noTrackChoice },
            maxWidth = facts.display.maxWidth,
            maxHeight = facts.display.maxHeight,
            // HDR only when the display shows HDR10 or HLG and a Main 10 HEVC decoder exists.
            hdr = hdrDisplay && "hevc" in video && !narrowing.noHdr,
            name = facts.name,
            type = facts.type.value,
        )
    }
}
