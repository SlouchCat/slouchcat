package com.slouchcat.core.player

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audio
import com.slouchcat.core.player.testing.group
import com.slouchcat.core.player.testing.info
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * New starts of the same title (concept 26 stage 3, review M-1 to M-4, m-1): a start that comes in
 * the background, a second new start while the first still loads, the wish to play across new
 * starts, a track choice outside the reported profile, a change of the output during the first
 * start. The attrappe behaves like Media3: after `stop()` the position is 0.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class PlaybackRestartTest {
    private val stereoTv = DeviceFacts(
        decoders = listOf(DecoderInfo("video/avc"), DecoderInfo("audio/mp4a-latm")),
        passthrough = emptySet(),
        outputChannels = 2,
        display = DisplayInfo(1920, 1080),
        type = DeviceType.Tv,
        name = "Test TV",
    )

    private class Setup(val controller: PlaybackController, val engine: FakeEngine, val server: FakeServer) {
        var facts: DeviceFacts? = null
    }

    private fun TestScope.setup(server: FakeServer = FakeServer(), settings: DeviceSettings = DeviceSettings()): Setup {
        val clock = { testScheduler.currentTime }
        val engine = FakeEngine(clock)
        val store = MemoryDeviceSettingsStore(settings)
        lateinit var s: Setup
        val controller = PlaybackController(
            scope = backgroundScope,
            finalScope = backgroundScope,
            server = server,
            engine = engine,
            capabilities = { narrowing -> CapabilityRules.build(s.facts ?: stereoTv, store.settings.first().audio, narrowing) },
            settings = store.settings,
            clock = clock,
        )
        s = Setup(controller, engine, server)
        runCurrent()
        return s
    }

    @Test
    fun aStartThatComesInTheBackgroundWaitsAndComesPaused() = runTest {
        val s = setup()
        s.controller.moveToBackground()
        // The page resolved its title only now (M-1): no session in the background.
        s.controller.start("film", startAtMs = 3_000)
        runCurrent()
        assertTrue(s.server.starts.isEmpty())
        assertTrue(s.engine.loads.isEmpty())
        s.controller.resumeFromBackground()
        runCurrent()
        assertEquals(3_000L, s.server.starts.single().second.positionMs)
        val (_, at, play) = s.engine.loads.single()
        assertEquals(3_000L, at)
        assertFalse("comes paused", play)
        assertFalse(s.controller.state.value.loading)
    }

    @Test
    fun aSecondNewStartWhileTheFirstLoadsKeepsThePosition() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(5_000)
        val gate = CompletableDeferred<Unit>()
        s.server.startGate = gate
        s.controller.capabilitiesChanged()
        runCurrent()
        // The player is empty now (Media3: position 0); a second change comes during the start (M-2).
        assertEquals(0L, s.engine.positionMs)
        s.controller.capabilitiesChanged()
        runCurrent()
        gate.complete(Unit)
        runCurrent()
        val position = s.server.starts.last().second.positionMs!!
        assertTrue("where it stood, not 0: $position", position in 4_900L..5_100L)
        assertTrue(s.engine.loads.last().second in 4_900L..5_100L)
    }

    @Test
    fun aPausedTitleStaysPausedAcrossNewStarts() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.controller.pause()
        runCurrent()
        // The receiver goes to standby and comes back (M-3): two new starts, both paused.
        s.controller.capabilitiesChanged()
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertEquals(listOf(true, false, false), s.engine.loads.map { it.third })
        // Back from the background paused, then the HDMI handshake: still paused.
        s.controller.moveToBackground()
        s.controller.resumeFromBackground()
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertFalse(s.engine.loads.last().third)
        // Play, then a new start: it plays on.
        s.engine.ready()
        runCurrent()
        s.controller.play()
        runCurrent()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertTrue(s.engine.loads.last().third)
    }

    @Test
    fun aTrackOutsideTheReportedProfileGoesToTheServer() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng", codec = "ac3"))
        val server = FakeServer().apply { answer = { id, request -> ApiResult.Ok(info(id, audio = tracks, resumeMs = request.positionMs ?: 0)) } }
        // A sound system that takes Dolby Digital, but «Immer Stereo» (M-4, rule 12).
        val s = setup(server, DeviceSettings(audio = AudioSetting.Stereo))
        s.facts = stereoTv.copy(passthrough = setOf("ac3"))
        s.controller.start("film")
        runCurrent()
        assertFalse("ac3" in s.server.starts.single().second.capabilities!!.audio)
        s.engine.ready(audio = listOf(group("de", selected = true), group("en", mime = "audio/ac3")))
        runCurrent()
        s.controller.selectAudio(tracks[1])
        runCurrent()
        // Media3 could pass it through, the profile says no: the server decides on a new start.
        // Only the server's choice at the start (group 0), never the Dolby group.
        assertEquals(listOf(0), s.engine.audioSelections)
        assertEquals(2, s.server.starts.size)
        assertEquals(2, s.server.starts.last().second.audioTrack)
    }

    @Test
    fun aReportedTrackIsChosenInThePlayer() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng", codec = "ac3"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, audio = tracks)) } }
        val s = setup(server)
        s.facts = stereoTv.copy(passthrough = setOf("ac3"))
        s.controller.start("film")
        runCurrent()
        s.engine.ready(audio = listOf(group("de", selected = true), group("en", mime = "audio/ac3")))
        runCurrent()
        s.controller.selectAudio(tracks[1])
        runCurrent()
        // The server's choice at the start (group 0), then the reported Dolby track in the player.
        assertEquals(listOf(0, 1), s.engine.audioSelections)
        assertEquals(1, s.server.starts.size)
    }

    @Test
    fun aNewOutputDuringTheFirstStartIsReported() = runTest {
        val s = setup()
        val gate = CompletableDeferred<Unit>()
        s.server.startGate = gate
        s.controller.start("film")
        runCurrent()
        // The receiver wakes up while the server still decides (m-1).
        s.facts = stereoTv.copy(passthrough = setOf("ac3", "eac3"))
        s.controller.capabilitiesChanged()
        runCurrent()
        gate.complete(Unit)
        runCurrent()
        assertEquals(PlaybackCapabilities.MaxAudioChannels._6, s.server.starts.last().second.capabilities!!.maxAudioChannels)
        assertEquals(1, s.engine.loads.size)
    }

    @Test
    fun nothingStartsAgainAtTheEndInTheBackgroundOrAfterAnError() = runTest {
        val s = setup()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertTrue("nothing plays yet", s.server.starts.isEmpty())
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.engine.end()
        runCurrent()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertEquals("not at the end (L-6)", 1, s.server.starts.size)
        s.controller.moveToBackground()
        s.controller.capabilitiesChanged()
        runCurrent()
        assertEquals(1, s.server.starts.size)
    }
}
