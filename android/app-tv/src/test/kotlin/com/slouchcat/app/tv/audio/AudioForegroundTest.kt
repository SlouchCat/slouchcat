package com.slouchcat.app.tv.audio

import com.slouchcat.app.tv.testing.EMULATOR_TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.player.testing.station
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The sound of the TV against Home, standby and the screen saver (concept 26 5.6, mockup: the
 * screen saver may come) and a video (mockup `hashchange`): Home ends the title's session with its
 * last progress and it comes back paused, standby as well and at once, the screen saver keeps it
 * playing, a station ends; a video pauses the title and ends the station.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class AudioForegroundTest {
    private val engines = mutableListOf<FakeEngine>()
    private val titles = FakeServer().apply { answer = { id, request -> ApiResult.Ok(audioInfo(id, resumeMs = request.positionMs ?: 0)) } }
    private val radio = FakeRadioServer()

    private fun TestScope.audio(): TvAudio {
        val playback = AudioPlayback(
            scope = backgroundScope,
            engines = { FakeEngine { testScheduler.currentTime }.also { it.duration = 600_000; engines += it } },
            radioServer = { radio },
            capabilities = { CapabilityRules.build(EMULATOR_TV, narrowing = it) },
            settings = MemoryDeviceSettingsStore().settings,
            clock = { testScheduler.currentTime },
        )
        return TvAudio(playback, Job()).also { it.follow(backgroundScope) }
    }

    private fun TestScope.playTitle(audio: TvAudio) {
        audio.playback.playTitle("book", AudioSource("online") { titles })
        runCurrent()
        engines.last().ready()
        runCurrent()
        engines.last().seekTo(90_000)
        runCurrent()
    }

    @Test
    fun homeEndsTheSessionAndTheTitleComesBackPaused() = runTest {

        val audio = audio()
        playTitle(audio)
        val foreground = AudioForeground(audio, backgroundScope)
        foreground.onStop()
        // Not at once: the screen saver may still tell itself.
        runCurrent()
        assertTrue(titles.stops.isEmpty())
        advanceTimeBy(AudioForeground.GRACE_MS + 1)
        runCurrent()
        assertEquals(listOf("session-book"), titles.stops)
        assertTrue("the last progress went out first: ${titles.progress}", titles.progress.isNotEmpty())
        // Back in front: the title starts again where it stood, paused.
        foreground.onStart()
        runCurrent()
        assertEquals(2, titles.starts.size)
        // It played on during the moment the stop waited for the screen saver.
        assertEquals(90_000L + AudioForeground.GRACE_MS,titles.starts.last().second.positionMs)
        assertFalse(engines.last().wantsPlay)
        assertTrue((audio.now.value as Playing.Title).paused)
    }

    @Test
    fun theScreenSaverKeepsItPlayingAndStandbyEndsItAtOnce() = runTest {

        val audio = audio()
        playTitle(audio)
        val foreground = AudioForeground(audio, backgroundScope)
        foreground.onDreamingStarted()
        foreground.onStop()
        advanceTimeBy(AudioForeground.GRACE_MS * 2)
        runCurrent()
        assertTrue(titles.stops.isEmpty())
        assertTrue(engines.last().wantsPlay)
        // The TV goes to standby under the screen saver.
        foreground.onScreenOff()
        runCurrent()
        assertEquals(listOf("session-book"), titles.stops)
    }

    @Test
    fun homeEndsAStation() = runTest {

        val audio = audio()
        audio.playback.playRadio(station(7))
        runCurrent()
        assertTrue(audio.now.value is Playing.Radio)
        val foreground = AudioForeground(audio, backgroundScope)
        foreground.onStop()
        advanceTimeBy(AudioForeground.GRACE_MS + 1)
        runCurrent()
        assertTrue("endRadioListen: ${radio.calls}", radio.calls.any { it.startsWith("end") })
        assertNull(audio.playback.now.value)
        foreground.onStart()
        runCurrent()
        assertNull(audio.now.value)
    }

    @Test
    fun aVideoPausesTheTitleAndEndsTheStation() = runTest {

        val audio = audio()
        playTitle(audio)
        audio.videoStarts()
        runCurrent()
        assertFalse(engines.last().wantsPlay)
        assertTrue(audio.playback.now.value is AudioNow.Title)
        audio.playback.playRadio(station(7))
        runCurrent()
        audio.videoStarts()
        runCurrent()
        assertNull(audio.playback.now.value)
    }

    @Test
    fun aProfileChangeEndsEverythingAndWaitsForTheLastProgress() = runTest {

        val audio = audio()
        playTitle(audio)
        audio.end()
        runCurrent()
        assertEquals(listOf("session-book"), titles.stops)
        assertNull(audio.now.value)
    }
}
