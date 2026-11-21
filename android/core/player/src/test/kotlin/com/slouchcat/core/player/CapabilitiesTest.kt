package com.slouchcat.core.player

import android.media.MediaCodecInfo.CodecProfileLevel
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The capability rules over recorded decoder lists (concept 26 stage 0 point 2, K27 stage 0 point 4). */
class CapabilitiesTest {
    private val phoneDisplay = DisplayInfo(2424, 1080)

    private fun facts(
        decoders: List<DecoderInfo>,
        passthrough: Set<String> = emptySet(),
        channels: Int = 2,
        display: DisplayInfo = phoneDisplay,
        type: DeviceType = DeviceType.Phone,
    ) = DeviceFacts(decoders, passthrough, channels, display, type, "Pixel 9 · Android 16")

    /** The decoders the emulator sc_phone reports: HEVC only Main (profiles 1 and 4), no Dolby. */
    private val emulator = listOf(
        DecoderInfo("video/avc", setOf(1, 2, 8)),
        DecoderInfo("video/hevc", setOf(CodecProfileLevel.HEVCProfileMain, CodecProfileLevel.HEVCProfileMainStill)),
        DecoderInfo("video/av01"),
        DecoderInfo("video/x-vnd.on2.vp9"),
        DecoderInfo("video/mpeg2"),
        DecoderInfo("audio/mp4a-latm"),
        DecoderInfo("audio/mpeg"),
        DecoderInfo("audio/opus"),
        DecoderInfo("audio/flac"),
        DecoderInfo("audio/vorbis"),
        DecoderInfo("audio/raw"),
    )

    /** A phone of today: HEVC with Main 10, the same audio decoders. */
    private val phoneWithMain10 = emulator.map {
        if (it.mime == "video/hevc") it.copy(profiles = it.profiles + CodecProfileLevel.HEVCProfileMain10) else it
    }

    /** A TV box: Main 10 and AC-3, E-AC-3 and TrueHD decoders. */
    private val boxWithDolby = phoneWithMain10 + listOf(DecoderInfo("audio/ac3"), DecoderInfo("audio/eac3"), DecoderInfo("audio/true-hd"))

    @Test
    fun aDeviceWithoutMain10ReportsNoHevc() {
        val caps = CapabilityRules.build(facts(emulator))
        assertEquals(listOf("h264", "av1", "vp9", "mpeg2video"), caps.video)
        assertEquals(listOf("aac", "mp3", "opus", "flac", "vorbis"), caps.audio)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, caps.maxAudioChannels)
        assertEquals(PlaybackCapabilities.Hls.MSE, caps.hls)
        assertEquals(true, caps.selectsTracks)
        assertEquals(CapabilityRules.SUBTITLES, caps.subtitles)
        assertEquals(2424, caps.maxWidth)
        assertEquals(1080, caps.maxHeight)
        assertEquals(false, caps.hdr)
        assertEquals(PlaybackCapabilities.Type.PHONE, caps.type)
    }

    @Test
    fun hevcCountsWithMain10() {
        assertEquals(listOf("h264", "hevc", "av1", "vp9", "mpeg2video"), CapabilityRules.build(facts(phoneWithMain10)).video)
    }

    @Test
    fun hdrNeedsTheDisplayAndMain10() {
        val hdrDisplay = phoneDisplay.copy(hdrTypes = setOf(HdrType.Hdr10))
        assertEquals(true, CapabilityRules.build(facts(phoneWithMain10, display = hdrDisplay)).hdr)
        assertEquals(false, CapabilityRules.build(facts(emulator, display = hdrDisplay)).hdr)
        // Dolby Vision alone counts not (concept 26 stage 0 point 5).
        assertEquals(false, CapabilityRules.build(facts(phoneWithMain10, display = phoneDisplay.copy(hdrTypes = setOf(HdrType.DolbyVision)))).hdr)
    }

    @Test
    fun dolbyDecodersAndPassthroughCount() {
        val decoded = CapabilityRules.build(facts(boxWithDolby))
        assertEquals(listOf("aac", "mp3", "opus", "flac", "vorbis", "ac3", "eac3", "truehd"), decoded.audio)
        // Decoded 5.1 goes to a stereo output: still two channels.
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, decoded.maxAudioChannels)

        val hdmi = CapabilityRules.build(facts(phoneWithMain10, passthrough = setOf("ac3", "eac3"), type = DeviceType.Tv))
        assertEquals(listOf("aac", "mp3", "opus", "flac", "vorbis", "ac3", "eac3"), hdmi.audio)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._6, hdmi.maxAudioChannels)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._8, CapabilityRules.build(facts(phoneWithMain10, passthrough = setOf("truehd"))).maxAudioChannels)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._8, CapabilityRules.build(facts(phoneWithMain10, channels = 8)).maxAudioChannels)
    }

    @Test
    fun stereoDropsDolbyAndChannels() {
        val caps = CapabilityRules.build(facts(boxWithDolby, passthrough = setOf("ac3"), channels = 6), AudioSetting.Stereo)
        assertEquals(listOf("aac", "mp3", "opus", "flac", "vorbis"), caps.audio)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, caps.maxAudioChannels)
    }

    @Test
    fun manualTakesTheTickedFormats() {
        val caps = CapabilityRules.build(facts(emulator), AudioSetting.Manual(setOf("ac3", "aac"), 6))
        assertEquals(listOf("aac", "ac3"), caps.audio)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._6, caps.maxAudioChannels)
    }

    @Test
    fun aNarrowingRemovesCodecsAndTrackChoice() {
        val caps = CapabilityRules.build(facts(phoneWithMain10, display = phoneDisplay.copy(hdrTypes = setOf(HdrType.Hdr10))), narrowing = Narrowing(audio = setOf("flac"), video = setOf("hevc"), noHdr = true, noTrackChoice = true))
        assertFalse("flac" in caps.audio)
        assertFalse("hevc" in caps.video)
        assertEquals(false, caps.hdr)
        assertEquals(false, caps.selectsTracks)
        assertEquals(null, caps.subtitles)
    }

    @Test
    fun audioCodecOfMime() {
        assertEquals("ac3", CapabilityRules.audioCodecOf("audio/ac3"))
        assertEquals("eac3", CapabilityRules.audioCodecOf("audio/eac3-joc"))
        assertEquals(null, CapabilityRules.audioCodecOf("audio/x-unknown"))
    }

    /** The profile of the contract test is the one these rules make for the emulator. */
    @Test
    fun theContractFixtureIsTheEmulatorsProfile() {
        val dir = requireNotNull(System.getProperty("slouchcat.fixtures")) { "the Gradle test task sets slouchcat.fixtures" }
        val fixture = SlouchcatApi.json.decodeFromString(PlaybackCapabilities.serializer(), File(dir, "phone-capabilities.json").readText())
        assertEquals(fixture, CapabilityRules.build(facts(emulator)))
        assertTrue(fixture.containers!!.contains("mkv"))
    }
}
