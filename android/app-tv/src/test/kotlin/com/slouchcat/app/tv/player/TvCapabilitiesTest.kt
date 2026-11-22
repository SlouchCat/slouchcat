package com.slouchcat.app.tv.player

import android.media.AudioDeviceInfo
import android.media.MediaCodecInfo.CodecProfileLevel
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.HdrType
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * What a TV reports to the server (concept 26 5.2, ADR 0041), for made-up devices against fixed
 * expected reports: stereo only, 5.1 as PCM, AC-3 passed through, HDR10 yes and no, a headphone
 * on a box with a sound system. The server decides from it (rule 12).
 */
class TvCapabilitiesTest {
    private val main10 = setOf(CodecProfileLevel.HEVCProfileMain, CodecProfileLevel.HEVCProfileMain10, CodecProfileLevel.HEVCProfileMain10HDR10)
    private val decoders = listOf(
        DecoderInfo("video/avc"), DecoderInfo("video/hevc", main10), DecoderInfo("video/x-vnd.on2.vp9"),
        DecoderInfo("audio/mp4a-latm"), DecoderInfo("audio/flac"), DecoderInfo("audio/opus"),
    )
    private val name = "Shield · Android TV 11"

    private fun device(passthrough: Set<String> = emptySet(), channels: Int = 2, hdr: Set<HdrType> = emptySet(), width: Int = 3840, height: Int = 2160) =
        DeviceFacts(decoders, passthrough, channels, DisplayInfo(width, height, hdr), DeviceType.Tv, "SHIELD Android TV · Android 11")

    private fun report(facts: DeviceFacts, outputs: Set<Int> = setOf(AudioDeviceInfo.TYPE_HDMI), setting: AudioSetting = AudioSetting.Auto) =
        CapabilityRules.build(tvFacts(facts, outputs, name), setting)

    private fun expected(audio: List<String>, channels: PlaybackCapabilities.MaxAudioChannels, hdr: Boolean = false, width: Int = 3840, height: Int = 2160) = PlaybackCapabilities(
        video = listOf("h264", "hevc", "vp9"),
        audio = audio,
        hls = PlaybackCapabilities.Hls.MSE,
        containers = listOf("mp4", "mkv", "webm", "ts", "m2ts", "mov"),
        maxAudioChannels = channels,
        selectsTracks = true,
        subtitles = listOf("subrip", "ass", "webvtt", "hdmv_pgs_subtitle", "dvd_subtitle"),
        maxWidth = width,
        maxHeight = height,
        hdr = hdr,
        name = name,
        type = PlaybackCapabilities.Type.TV,
    )

    @Test
    fun aTvWithItsOwnSpeakersReportsStereo() {
        assertEquals(expected(listOf("aac", "opus", "flac"), PlaybackCapabilities.MaxAudioChannels._2), report(device()))
    }

    @Test
    fun aReceiverWithSixChannelsOfPcmReportsSixWithoutDolby() {
        assertEquals(expected(listOf("aac", "opus", "flac"), PlaybackCapabilities.MaxAudioChannels._6), report(device(channels = 6)))
    }

    @Test
    fun aSoundbarThatTakesDolbyDigitalGetsItPassedThrough() {
        val soundbar = device(passthrough = setOf("ac3", "eac3"), channels = 2)
        assertEquals(expected(listOf("aac", "opus", "flac", "ac3", "eac3"), PlaybackCapabilities.MaxAudioChannels._6), report(soundbar))
        // TrueHD over eARC: eight channels.
        assertEquals(PlaybackCapabilities.MaxAudioChannels._8, report(device(passthrough = setOf("ac3", "eac3", "truehd"))).maxAudioChannels)
    }

    @Test
    fun hdrOnlyWithAnHdrDisplayAndAMain10Decoder() {
        assertEquals(expected(listOf("aac", "opus", "flac"), PlaybackCapabilities.MaxAudioChannels._2, hdr = true), report(device(hdr = setOf(HdrType.Hdr10))))
        assertEquals(false, report(device(hdr = emptySet())).hdr)
        // Dolby Vision alone does not count (K26 stage 0 point 5).
        assertEquals(false, report(device(hdr = setOf(HdrType.DolbyVision))).hdr)
        // An HDR display on a 1080p box: no HDR10 profile of the decoder, no HDR.
        val plain = DeviceFacts(decoders.map { if (it.mime == "video/hevc") DecoderInfo("video/hevc", setOf(CodecProfileLevel.HEVCProfileMain10)) else it }, emptySet(), 2, DisplayInfo(1920, 1080, setOf(HdrType.Hdr10)), DeviceType.Tv, "x")
        assertEquals(false, CapabilityRules.build(tvFacts(plain, emptySet(), name)).hdr)
    }

    @Test
    fun aHeadphoneTakesTheSoundFromTheSoundSystem() {
        val box = device(passthrough = setOf("ac3", "eac3"), channels = 8)
        val headphone = report(box, outputs = setOf(AudioDeviceInfo.TYPE_HDMI, AudioDeviceInfo.TYPE_BLUETOOTH_A2DP))
        assertEquals(expected(listOf("aac", "opus", "flac"), PlaybackCapabilities.MaxAudioChannels._2), headphone)
    }

    @Test
    fun theSettingOfTheSoundWins() {
        val soundbar = device(passthrough = setOf("ac3", "eac3"), channels = 6)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, report(soundbar, setting = AudioSetting.Stereo).maxAudioChannels)
        assertEquals(listOf("aac", "opus", "flac"), report(soundbar, setting = AudioSetting.Stereo).audio)
        // By hand: what the page puts in (the decoders and the ticked formats), DTS although nothing found it.
        val manual = report(device(), setting = AudioSetting.Manual(setOf("aac", "flac", "opus", "ac3", "dts"), 6))
        assertEquals(listOf("aac", "ac3", "dts", "flac", "opus"), manual.audio)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._6, manual.maxAudioChannels)
    }

    @Test
    fun theOutputTheMenuNamesFollowsTheSetting() {
        val found = SoundOutput.of(tvFacts(device(passthrough = setOf("ac3", "eac3", "dts"), channels = 6), setOf(AudioDeviceInfo.TYPE_HDMI_ARC), name))
        assertEquals(SoundOutput(listOf("ac3", "eac3", "dts"), 6), found)
        assertEquals(SoundOutput(emptyList(), 2), reported(AudioSetting.Stereo, found))
        assertEquals(SoundOutput(listOf("ac3"), 8), reported(AudioSetting.Manual(setOf("aac", "ac3"), 8), found))
        assertEquals(found, reported(AudioSetting.Auto, found))
    }
}
