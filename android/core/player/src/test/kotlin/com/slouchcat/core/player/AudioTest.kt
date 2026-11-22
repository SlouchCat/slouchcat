package com.slouchcat.core.player

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.MemoryAudio
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.player.testing.ref
import com.slouchcat.core.player.testing.station
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioNowEntry
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.math.BigDecimal

@OptIn(ExperimentalCoroutinesApi::class)
class AudioTest {
    private val phone = DeviceFacts(
        decoders = listOf(DecoderInfo("audio/mp4a-latm")),
        passthrough = emptySet(),
        outputChannels = 2,
        display = DisplayInfo(2424, 1080),
        type = DeviceType.Phone,
        name = "Test phone",
    )

    private fun TestScope.controller(server: FakeServer, engine: FakeEngine) = PlaybackController(
        backgroundScope,
        backgroundScope,
        server,
        engine,
        { CapabilityRules.build(phone, narrowing = it) },
        MemoryDeviceSettingsStore().settings,
        clock = { testScheduler.currentTime },
    )

    @Test
    fun anAudiobookPlaysAtItsOwnSpeedOverItsWholeLength() = runTest {
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id, rate = 1.5, durationMs = 7_200_000)) } }
        val engine = FakeEngine { testScheduler.currentTime }.apply { duration = 0 }
        val c = controller(server, engine)
        runCurrent()
        c.start("book")
        runCurrent()
        assertTrue(c.state.value.isAudio)
        assertEquals(1.5f, c.state.value.speed)
        assertEquals(listOf(1.5f), engine.speeds)
        // The first file is short; the title is two hours: the timeline is the title's.
        assertEquals(7_200_000L, c.state.value.durationMs)
        val media = engine.loads.single().first
        assertEquals("Ann Author", media.subtitle)
        assertTrue(media.hls)
    }

    @Test
    fun audioReportsItsSpeedAlsoBackAtOne() = runTest {
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id, rate = 2.0)) } }
        val engine = FakeEngine { testScheduler.currentTime }
        val c = controller(server, engine)
        runCurrent()
        c.start("book")
        runCurrent()
        engine.ready()
        runCurrent()
        c.setSpeed(1f)
        advanceTimeBy(10_001)
        assertEquals(BigDecimal("1.0"), server.progress.last().second.playbackRate)
    }

    @Test
    fun anEpisodeWithANextOneShowsNoCountdown() = runTest {
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id).copy(nextEpisode = ref("next", kind = ItemKind.PODCAST_EPISODE))) } }
        val engine = FakeEngine { testScheduler.currentTime }
        val c = controller(server, engine)
        runCurrent()
        c.start("episode")
        runCurrent()
        engine.ready()
        runCurrent()
        engine.end()
        advanceTimeBy(20_000)
        assertNull(c.state.value.countdown)
        assertTrue(c.state.value.ended)
        assertEquals(1, server.starts.size)
        assertEquals(ProgressEvent.State.ENDED, server.progress.last().second.state)
    }

    private val chapters = listOf(Chapter(0, 60_000, "Eins"), Chapter(60_000, 120_000, "Zwei"), Chapter(120_000, 180_000, "Drei"))

    @Test
    fun chaptersOnTheTimeline() {
        assertNull(AudioRules.chapterIndex(emptyList(), 10))
        assertEquals(1, AudioRules.chapterIndex(chapters, 60_000))
        // Within three seconds of a chapter's start «Vorheriges Kapitel» goes to the one before.
        assertEquals(0L, AudioRules.previousChapterStart(chapters, 61_000))
        assertEquals(60_000L, AudioRules.previousChapterStart(chapters, 70_000))
        assertEquals(0L, AudioRules.previousChapterStart(chapters, 1_000))
        assertEquals(120_000L, AudioRules.nextChapterStart(chapters, 70_000))
        assertNull(AudioRules.nextChapterStart(chapters, 150_000))
        assertEquals(1.3f, AudioRules.clampSpeed(1.3000001f))
        assertEquals(3f, AudioRules.clampSpeed(4f))
    }

    @Test
    fun theSleepTimerCountsOnlyWhilePlaying() {
        val timer = SleepTimer()
        fun tick(at: Long, playing: Boolean) = timer.tick(at, playing, 0, chapters, 180_000, 1f)
        timer.set(SleepChoice.Minutes(15), 0, chapters, 180_000, 1f)
        assertFalse(tick(0, playing = true))
        assertFalse(tick(5 * 60_000, playing = true))
        assertEquals(10 * 60_000L, timer.state?.leftMs)
        // Paused from minute 5 to minute 15: that time does not count.
        assertFalse(tick(15 * 60_000, playing = false))
        assertFalse(tick(16 * 60_000, playing = true))
        assertEquals(10 * 60_000L, timer.state?.leftMs)
        assertTrue(tick(26 * 60_000 + 1, playing = true))
        assertNull(timer.state)
    }

    @Test
    fun theEndOfAChapterInWallTimeAtTheSpeed() {
        val timer = SleepTimer()
        timer.set(SleepChoice.EndOfChapter, 30_000, chapters, 180_000, 2f)
        assertEquals(15_000L, timer.state?.leftMs)
        assertTrue(timer.tick(1, playing = true, positionMs = 60_000, chapters = chapters, durationMs = 180_000, speed = 2f))
    }

    // ---- radio -------------------------------------------------------------------------------

    private fun TestScope.radio(server: FakeRadioServer, engine: FakeEngine) = RadioController(backgroundScope, backgroundScope, server, engine)

    @Test
    fun aStationPlaysFromItsLiveEdgeWithAHeartbeatAlsoWhilePaused() = runTest {
        val server = FakeRadioServer().apply { heartbeatSeconds = 30 }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = radio(server, engine)
        radio.start(station(7))
        runCurrent()
        val (media, start, play) = engine.loads.single()
        assertEquals("http://server.test/api/v1/radio/listen/x/live.m3u8?rt=token", media.url)
        assertTrue(media.hls)
        assertEquals(PlayerEngine.LIVE_EDGE, start)
        assertTrue(play)
        engine.ready()
        runCurrent()
        assertTrue(radio.state.value.playing)
        advanceTimeBy(30_001)
        assertEquals("beat:listen-1:true", server.calls.last { it.startsWith("beat") })
        radio.pause()
        runCurrent()
        // The pause tells the server at once, then the beat goes on.
        assertEquals("beat:listen-1:false", server.calls.last())
        val beats = server.calls.count { it.startsWith("beat") }
        advanceTimeBy(30_000)
        assertEquals(beats + 1, server.calls.count { it.startsWith("beat") })
        radio.stop()
        runCurrent()
        assertEquals("end:listen-1", server.calls.last())
        assertFalse(radio.state.value.active)
    }

    @Test
    fun whatPlaysNowIsTheNewestEntry() = runTest {
        val server = FakeRadioServer().apply {
            nowAnswer = ApiResult.Ok(RadioNow(true, listOf(RadioNowEntry(1, "a", "A", "Old"), RadioNowEntry(2, "b", "B", "New"))))
        }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = radio(server, engine)
        radio.start(station(7))
        runCurrent()
        engine.ready()
        advanceTimeBy(10_001)
        assertEquals("New", radio.state.value.now?.title)
    }

    @Test
    fun aGoneRelayEndsWithoutAnEndOfOurs() = runTest {
        val server = FakeRadioServer().apply { heartbeatSeconds = 10 }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = radio(server, engine)
        radio.start(station(7))
        runCurrent()
        engine.ready()
        server.heartbeatAnswer = ApiResult.Failed(410, "gone")
        advanceTimeBy(10_001)
        assertEquals(RadioError.Unreachable, radio.state.value.error)
        assertFalse(server.calls.any { it.startsWith("end") })
    }

    @Test
    fun theWatchLimitStopsTheRadio() = runTest {
        val server = FakeRadioServer().apply { startAnswer = { ApiResult.NotAllowed("watch_limit_reached") } }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = radio(server, engine)
        radio.start(station(7))
        runCurrent()
        assertEquals(RadioError.WatchLimit, radio.state.value.error)
        assertTrue(engine.loads.isEmpty())
    }

    @Test
    fun aLateStartIsEndedAtOnce() = runTest {
        val server = FakeRadioServer().apply { startGate = CompletableDeferred() }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = radio(server, engine)
        radio.start(station(7))
        runCurrent()
        radio.stop()
        server.startGate!!.complete(Unit)
        runCurrent()
        assertEquals(listOf("start:7", "end:listen-1"), server.calls)
        assertTrue(engine.loads.isEmpty())
    }

    // ---- the playback of the app ---------------------------------------------------------------

    private class Playback(val playback: AudioPlayback, val engines: MutableList<Pair<EngineKind, FakeEngine>>, val titles: FakeServer, val radio: FakeRadioServer, val memory: MemoryAudio)

    private fun TestScope.playback(memory: MemoryAudio = MemoryAudio(), owner: String = "server|#1"): Playback {
        val engines = mutableListOf<Pair<EngineKind, FakeEngine>>()
        val titles = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id, chapters = chapters, durationMs = 180_000)) } }
        val radio = FakeRadioServer()
        val playback = AudioPlayback(
            scope = backgroundScope,
            engines = { kind -> FakeEngine { testScheduler.currentTime }.also { it.duration = 180_000; engines += kind to it } },
            radioServer = { radio },
            capabilities = { CapabilityRules.build(phone, narrowing = it) },
            settings = MemoryDeviceSettingsStore().settings,
            clock = { testScheduler.currentTime },
            memory = memory,
            owner = { owner },
        )
        return Playback(playback, engines, titles, radio, memory)
    }

    private fun Playback.online() = AudioSource("online") { titles }

    @Test
    fun aStationEndsTheTitleAndTheCrossEndsTheStation() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        val book = p.engines.single().second
        assertEquals(EngineKind.Speech, p.engines.single().first)
        book.ready()
        advanceTimeBy(5_000)
        assertTrue(p.playback.now.value is AudioNow.Title)
        assertEquals("book", p.memory.last?.itemId)

        p.playback.playRadio(station(7))
        runCurrent()
        // The title ended: its last progress and its stop, its player released.
        assertEquals(listOf("progress:session-book", "stop:session-book"), p.titles.calls)
        assertTrue(book.released)
        assertEquals(EngineKind.Music, p.engines.last().first)
        assertTrue(p.playback.now.value is AudioNow.Radio)
        assertEquals(p.engines.last().second, p.playback.engine.value)

        p.playback.stop()
        runCurrent()
        assertNull(p.playback.now.value)
        assertNull(p.playback.engine.value)
        assertEquals("end:listen-1", p.radio.calls.last())
        assertNull(p.memory.last)
    }

    @Test
    fun afterTheDeathOfTheProcessTheLastTitleWaitsForAHand() = runTest {
        val memory = MemoryAudio(LastAudio("book", "Silo", "Ann Author", owner = "server|#1"))
        val p = playback(memory)
        p.playback.restoredSource = p.online()
        p.playback.restore()
        runCurrent()
        // Shown, not started: no player, no request.
        assertEquals("Silo", (p.playback.now.value as AudioNow.Restored).last.title)
        assertTrue(p.engines.isEmpty())
        assertTrue(p.titles.starts.isEmpty())
        p.playback.togglePlay()
        runCurrent()
        assertEquals("book", p.titles.starts.single().first)
        // Where the server resumes, not a position of the device.
        assertNull(p.titles.starts.single().second.positionMs)
    }

    @Test
    fun anotherProfileDoesNotSeeTheLastTitle() = runTest {
        val p = playback(MemoryAudio(LastAudio("book", "Silo", null, owner = "server|#2")))
        p.playback.restore()
        runCurrent()
        assertNull(p.playback.now.value)
    }

    @Test
    fun theSleepTimerPausesAtTheEndOfTheChapter() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        val book = p.engines.single().second
        book.ready()
        runCurrent()
        p.playback.setSleep(SleepChoice.EndOfChapter)
        assertEquals(60_000L, (p.playback.now.value as AudioNow.Title).sleep?.leftMs)
        advanceTimeBy(61_000)
        assertFalse(book.wantsPlay)
        assertNull((p.playback.now.value as AudioNow.Title).sleep)
    }

    @Test
    fun chaptersForwardAndBack() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        val book = p.engines.single().second
        book.ready()
        advanceTimeBy(70_000)
        p.playback.nextChapter()
        assertEquals(120_000L, book.positionMs)
        p.playback.previousChapter()
        // Right at the start of «Drei»: back to «Zwei».
        assertEquals(60_000L, book.positionMs)
    }

    @Test
    fun theSameTitleAgainPlaysOnWithoutANewStart() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        p.engines.single().second.ready()
        runCurrent()
        p.playback.playTitle("book", p.online())
        runCurrent()
        assertEquals(1, p.titles.starts.size)
        assertEquals(1, p.engines.size)
    }

    // ---- the correction round (review of stage 7) ---------------------------------------------

    @Test
    fun aChapterOfThePlayingTitleKeepsItsSessionAndSpeed() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        val book = p.engines.single().second
        book.ready()
        runCurrent()
        p.playback.title!!.setSpeed(1.5f)
        p.playback.playTitle("book", p.online(), startAtMs = 120_000)
        runCurrent()
        // No new start, no lost time: the same session jumps and plays on at its speed (m-1).
        assertEquals(1, p.titles.starts.size)
        assertEquals(120_000L, book.positionMs)
        assertEquals(1.5f, p.playback.title!!.state.value.speed)
    }

    @Test
    fun theRetryOfAFailedTitleStartsAgain() = runTest {
        val p = playback()
        p.titles.answer = { _, _ -> ApiResult.Offline }
        p.playback.playTitle("book", p.online())
        runCurrent()
        assertTrue(p.playback.title!!.state.value.error is PlayerError.Network)
        p.titles.answer = { id, _ -> ApiResult.Ok(audioInfo(id)) }
        // «Erneut versuchen» of the mini player (M-5).
        p.playback.togglePlay()
        runCurrent()
        assertEquals(2, p.titles.starts.size)
    }

    @Test
    fun theSignOutWaitsForTheLastProgressAndTheStop() = runTest {
        val p = playback()
        p.playback.playTitle("book", p.online())
        runCurrent()
        p.engines.single().second.ready()
        advanceTimeBy(5_000)
        // Like `beforeSignOut`: when it returns, both requests reached the server (M-2).
        p.playback.stopAndFlush()
        assertEquals(listOf("progress:session-book", "stop:session-book"), p.titles.calls)
    }

    @Test
    fun aStartCutOffByAStationEndsItsSessionWhenTheAnswerComes() = runTest {
        val p = playback()
        p.titles.startGate = CompletableDeferred()
        p.playback.playTitle("book", p.online())
        runCurrent()
        p.playback.playRadio(station(7))
        runCurrent()
        p.titles.startGate!!.complete(Unit)
        runCurrent()
        // The session the server opened for the book ends at once (m-4).
        assertEquals(listOf("stop:session-book"), p.titles.calls)
    }

    @Test
    fun theCrossForgetsARestoredTitle() = runTest {
        val memory = MemoryAudio(LastAudio("book", "Silo", null, owner = "server|#1"))
        val p = playback(memory)
        p.playback.restore()
        runCurrent()
        p.playback.stop()
        runCurrent()
        // The file of the last title goes too (L-4), so nothing shows at the next start.
        assertNull(memory.last)
    }

    @Test
    fun aRestoredDownloadPlaysFromTheDevice() = runTest {
        val memory = MemoryAudio(LastAudio("book", "Silo", null, owner = "server|#1", offline = true))
        val p = playback(memory)
        val local = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id)) } }
        p.playback.restoredSource = p.online()
        p.playback.restoredOfflineSource = { AudioSource("offline:1") { local } }
        p.playback.restore()
        p.playback.togglePlay()
        runCurrent()
        // m-11: from the download, not from the server.
        assertTrue(p.titles.starts.isEmpty())
        assertEquals("book", local.starts.single().first)
    }

    @Test
    fun aLongPauseOfTheRadioStartsANewListen() = runTest {
        var now = 0L
        val server = FakeRadioServer().apply { heartbeatSeconds = 30 }
        val engine = FakeEngine { testScheduler.currentTime }
        val radio = RadioController(backgroundScope, backgroundScope, server, engine) { now }
        radio.start(station(7))
        runCurrent()
        engine.ready()
        runCurrent()
        radio.pause()
        runCurrent()
        now += 10 * 60_000L
        radio.play()
        runCurrent()
        // m-2: behind the live window and past the server's patience, a new listen.
        assertEquals(2, server.calls.count { it.startsWith("start") })
        assertTrue("end:listen-1" in server.calls)
    }
}
