package com.slouchcat.core.player

import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.info
import com.slouchcat.core.player.testing.ref
import com.slouchcat.core.player.testing.audio
import com.slouchcat.core.player.testing.subtitle
import com.slouchcat.core.player.testing.group
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.Quality
import com.slouchcat.core.api.generated.models.QualityCap
import com.slouchcat.core.api.generated.models.Segment
import com.slouchcat.core.api.generated.models.TrackSelection
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The controller against a fake player and a fake server, in the virtual time of the test. */
@OptIn(ExperimentalCoroutinesApi::class)
class PlaybackControllerTest {
    private val emulator = DeviceFacts(
        decoders = listOf(DecoderInfo("video/avc"), DecoderInfo("audio/mp4a-latm"), DecoderInfo("audio/flac")),
        passthrough = emptySet(),
        outputChannels = 2,
        display = DisplayInfo(2424, 1080),
        type = DeviceType.Phone,
        name = "Test phone",
    )

    private class Setup(val controller: PlaybackController, val engine: FakeEngine, val server: FakeServer, val settings: MemoryDeviceSettingsStore)

    private fun TestScope.setup(server: FakeServer = FakeServer(), settings: DeviceSettings = DeviceSettings(), metered: Boolean = false): Setup {
        val clock = { testScheduler.currentTime }
        val engine = FakeEngine(clock)
        val store = MemoryDeviceSettingsStore(settings)
        val controller = PlaybackController(
            scope = backgroundScope,
            finalScope = backgroundScope,
            server = server,
            engine = engine,
            capabilities = { narrowing -> CapabilityRules.build(emulator, narrowing = narrowing) },
            settings = store.settings,
            metered = { metered },
            clock = clock,
        )
        runCurrent()
        return Setup(controller, engine, server, store)
    }

    private fun Setup.progressStates() = server.progress.map { it.second.state }

    @Test
    fun aDirectStartPlaysTheFileAndReportsEveryTenSeconds() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        val request = s.server.starts.single().second
        assertEquals(listOf("h264"), request.capabilities!!.video)
        assertEquals(true, request.capabilities!!.selectsTracks)
        assertEquals(QualityCap._1080P, request.qualityCap)
        val (media, start, play) = s.engine.loads.single()
        assertEquals("http://server.test/api/v1/items/film/stream?st=token", media.url)
        assertFalse(media.hls)
        assertEquals(0L, start)
        assertTrue(play)

        s.engine.ready()
        runCurrent()
        assertTrue(s.controller.state.value.playing)
        assertEquals(0L, s.controller.state.value.firstFrameMs)
        advanceTimeBy(10_001)
        val first = s.server.progress.single().second
        assertEquals(ProgressEvent.State.PLAYING, first.state)
        assertEquals(1L, first.seq)
        assertEquals(10_000L, first.playedMs)
        assertEquals("session-film", first.playbackSessionId)
        advanceTimeBy(10_000)
        assertEquals(listOf(1L, 2L), s.server.progress.map { it.second.seq })
        assertEquals(20_000L, s.server.progress.last().second.positionMs)
    }

    @Test
    fun theMobileNetworkUsesItsOwnCap() = runTest {
        val s = setup(metered = true)
        s.controller.start("film")
        runCurrent()
        assertEquals(QualityCap._720P, s.server.starts.single().second.qualityCap)
    }

    @Test
    fun pauseEndAndLeavingReport() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(3_000)
        s.controller.pause()
        runCurrent()
        assertEquals(listOf(ProgressEvent.State.PAUSED), s.progressStates())
        assertEquals(3_000L, s.server.progress.last().second.playedMs)
        s.controller.play()
        runCurrent()
        s.engine.end()
        runCurrent()
        assertEquals(ProgressEvent.State.ENDED, s.progressStates().last())
        assertTrue(s.controller.state.value.ended)

        s.controller.close()
        runCurrent()
        // Ended: no further progress, but the session ends and the player is released.
        assertEquals(ProgressEvent.State.ENDED, s.progressStates().last())
        assertEquals(listOf("session-film"), s.server.stops)
        assertTrue(s.engine.released)
    }

    @Test
    fun leavingThePageReportsWhereItStoodAndEndsTheSession() = runTest {
        val s = setup()
        s.controller.start("film", startAtMs = 5_000)
        runCurrent()
        assertEquals(5_000L, s.server.starts.single().second.positionMs)
        s.engine.ready()
        runCurrent()
        advanceTimeBy(2_000)
        s.controller.close()
        runCurrent()
        val last = s.server.progress.last().second
        assertEquals(ProgressEvent.State.PAUSED, last.state)
        assertEquals(7_000L, last.positionMs)
        assertEquals(listOf("session-film"), s.server.stops)
    }

    @Test
    fun fromTheStartAsksForPositionZero() = runTest {
        val s = setup()
        s.controller.start("film", startAtMs = 9_000, fromStart = true)
        runCurrent()
        assertEquals(0L, s.server.starts.single().second.positionMs)
    }

    @Test
    fun aStopByTheServerEndsThePlayback() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(4_001)
        assertEquals(1, s.server.stateCalls)
        s.server.state = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateStopped, PlaybackSessionState.StoppedBy.PlaybackStoppedByAdmin))
        advanceTimeBy(4_000)
        assertEquals(PlayerError.StoppedElsewhere, s.controller.state.value.error)
        val before = s.server.progress.size
        advanceTimeBy(30_000)
        assertEquals(before, s.server.progress.size)
        s.controller.close()
        runCurrent()
        // The server ended it: no stop of ours.
        assertTrue(s.server.stops.isEmpty())
    }

    @Test
    fun theWatchLimitStopsWithItsOwnPicture() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.server.state = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateStopped, PlaybackSessionState.StoppedBy.PlaybackStoppedByLimit))
        advanceTimeBy(4_001)
        assertEquals(PlayerError.WatchLimit, s.controller.state.value.error)
    }

    @Test
    fun theStartErrorsHaveTheirPictures() = runTest {
        val cases = mapOf(
            ApiResult.NotFound to PlayerError.NotFound,
            ApiResult.NotAllowed("watch_limit_reached") to PlayerError.WatchLimit,
            ApiResult.Failed(409, "stream_limit_reached") to PlayerError.StreamLimit,
            ApiResult.Offline to PlayerError.Network(retrying = false),
            ApiResult.Failed(500, null) to PlayerError.Failed,
        )
        for ((answer, expected) in cases) {
            val server = FakeServer().apply { this.answer = { _, _ -> answer } }
            val s = setup(server)
            s.controller.start("film")
            runCurrent()
            assertEquals(expected, s.controller.state.value.error)
            assertTrue(s.engine.loads.isEmpty())
        }
    }

    @Test
    fun aBusyServerIsAskedAgain() = runTest {
        val server = FakeServer()
        var calls = 0
        server.answer = { id, _ -> if (++calls == 1) ApiResult.Failed(503, "transcode_busy") else ApiResult.Ok(info(id)) }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        assertEquals(PlayerError.Busy(retrying = true), s.controller.state.value.error)
        advanceTimeBy(5_001)
        assertEquals(2, server.starts.size)
        assertNull(s.controller.state.value.error)
        assertEquals(1, s.engine.loads.size)
    }

    @Test
    fun anotherAudioTrackSwitchesInsideThePlayerWhenTheGroupsMatch() = runTest {
        val tracks = listOf(audio(10, 1, "deu", title = "Deutsch"), audio(11, 2, "eng", title = "English"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, audio = tracks)) } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.engine.ready(audio = listOf(group("de", "Deutsch", selected = true), group("en", "English")))
        runCurrent()
        assertEquals(listOf(0), s.engine.audioSelections)
        s.controller.selectAudio(tracks[1])
        runCurrent()
        assertEquals(listOf(0, 1), s.engine.audioSelections)
        assertEquals(1, server.starts.size)
        assertEquals(tracks[1], s.controller.state.value.selectedAudio)
    }

    @Test
    fun anotherAudioTrackOverHlsStartsAgainWhereItStood() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng"))
        val server = FakeServer().apply { answer = { id, request -> ApiResult.Ok(info(id, direct = false, audio = tracks, resumeMs = request.positionMs ?: 0, session = "s${request.audioTrack}")) } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        assertTrue(s.engine.loads.single().first.hls)
        s.engine.ready(audio = listOf(group("de", selected = true)))
        runCurrent()
        advanceTimeBy(8_000)
        s.controller.selectAudio(tracks[1])
        runCurrent()
        val again = server.starts.last().second
        assertEquals(2, again.audioTrack)
        assertEquals(8_000L, again.positionMs)
        assertEquals(listOf("snull"), s.server.stops)
        assertEquals(8_000L, s.engine.loads.last().second)
    }

    @Test
    fun groupsThatDoNotMatchLetTheServerChoose() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, audio = tracks, selection = TrackSelection(audioTrack = 2))) } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.engine.ready(audio = listOf(group("fr"), group("it", selected = true)))
        runCurrent()
        assertEquals(2, server.starts.size)
        assertEquals(false, server.starts.last().second.capabilities!!.selectsTracks)
    }

    @Test
    fun aSilentTrackStartsAgainWithoutItsCodec() = runTest {
        val tracks = listOf(audio(10, 1, "deu", codec = "aac"), audio(11, 2, "eng", codec = "ac3"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, audio = tracks, selection = TrackSelection(audioTrack = 2))) } }
        val s = setup(server, settings = DeviceSettings())
        s.controller.start("film")
        runCurrent()
        // The narrowing remembers the codec, so a second silent start does not loop.
        s.engine.ready(audio = listOf(group("de"), group("en", mime = "audio/ac3", supported = false)))
        runCurrent()
        assertEquals(2, server.starts.size)
        val again = server.starts.last().second
        assertFalse("ac3" in again.capabilities!!.audio)
        assertEquals(2, again.audioTrack)
        // Silent again with the same codec: no third start.
        s.engine.ready(audio = listOf(group("de"), group("en", mime = "audio/ac3", supported = false)))
        runCurrent()
        assertEquals(2, server.starts.size)
    }

    @Test
    fun aVideoDecoderErrorNarrowsOnceThenShowsTheError() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.engine.emit(EngineEvent.Failed(EngineFailure.VideoDecoder))
        runCurrent()
        assertEquals(2, s.server.starts.size)
        val narrowed = s.server.starts.last().second.capabilities!!
        assertFalse("hevc" in narrowed.video)
        assertEquals(false, narrowed.hdr)
        s.engine.emit(EngineEvent.Failed(EngineFailure.VideoDecoder))
        runCurrent()
        assertEquals(2, s.server.starts.size)
        assertEquals(PlayerError.Unplayable, s.controller.state.value.error)
    }

    @Test
    fun theNetworkComesBackAndThePlaybackGoesOn() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(5_000)
        s.server.state = ApiResult.Offline
        s.engine.emit(EngineEvent.Failed(EngineFailure.Network))
        runCurrent()
        assertEquals(PlayerError.Network(retrying = true), s.controller.state.value.error)
        advanceTimeBy(3_001)
        assertEquals(1, s.engine.loads.size)
        s.server.state = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateActive))
        advanceTimeBy(3_000)
        assertEquals(2, s.engine.loads.size)
        assertEquals(5_000L, s.engine.loads.last().second)
        assertNull(s.controller.state.value.error)
    }

    @Test
    fun subtitlesFromTheFileAndFromTheServer() = runTest {
        val subs = listOf(subtitle(20, 3, "deu"), subtitle(21, -1, "eng", source = com.slouchcat.core.api.generated.models.Track.Source.SIDECAR))
        val server = FakeServer().apply {
            answer = { id, request ->
                if (request.subtitleTrack == 21L) {
                    ApiResult.Ok(info(id, subtitles = subs, selection = TrackSelection(audioTrack = 1, subtitleTrack = 21, subtitleDelivery = TrackSelection.SubtitleDelivery.TEXT), subtitleUrl = "/api/v1/playback/s/subtitle.vtt?st=t", session = "s2"))
                } else {
                    ApiResult.Ok(info(id, subtitles = subs))
                }
            }
        }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.engine.ready(audio = listOf(group("de", selected = true)), text = listOf(EngineTrack("de", null, "application/x-subrip")))
        runCurrent()
        assertEquals(listOf<Int?>(null), s.engine.textSelections)
        assertTrue(s.controller.state.value.subtitlesOff)
        // Embedded text track: inside the player.
        s.controller.selectSubtitle(subs[0])
        runCurrent()
        assertEquals(listOf(null, 0), s.engine.textSelections)
        assertEquals(1, server.starts.size)
        // A file next to the video: a new start, the WebVTT of the server comes with the stream.
        s.controller.selectSubtitle(subs[1])
        runCurrent()
        assertEquals(2, server.starts.size)
        assertEquals(21L, server.starts.last().second.subtitleTrack)
        assertEquals("http://server.test/api/v1/playback/s/subtitle.vtt?st=t", s.engine.loads.last().first.subtitleUrl)
        s.engine.ready(audio = listOf(group("de", selected = true)), text = listOf(EngineTrack("de", null, "application/x-subrip"), EngineTrack("en", null, "text/vtt", sideloaded = true)))
        runCurrent()
        assertEquals(1, s.engine.textSelections.last())
        // Off: inside the player.
        s.controller.selectSubtitle(null)
        runCurrent()
        assertNull(s.engine.textSelections.last())
        assertEquals(2, server.starts.size)
    }

    @Test
    fun aQualityStartsAgainWithTheQuality() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.controller.selectQuality(Quality._720P)
        runCurrent()
        assertEquals(Quality._720P, s.server.starts.last().second.quality)
        assertNull(s.server.starts.last().second.qualityCap)
        s.controller.selectQuality(Quality.AUTO)
        runCurrent()
        assertNull(s.server.starts.last().second.quality)
        assertEquals(QualityCap._1080P, s.server.starts.last().second.qualityCap)
    }

    @Test
    fun theIntroCanBeSkipped() = runTest {
        val episode = ref("e1", kind = ItemKind.EPISODE, season = 1, episode = 1, parent = "s1")
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, item = episode, intro = Segment(2_000, 20_000, Segment.Source.CHAPTER))) } }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(1_000)
        assertFalse(s.controller.state.value.showSkipIntro)
        advanceTimeBy(2_000)
        assertTrue(s.controller.state.value.showSkipIntro)
        s.controller.skipIntro()
        advanceTimeBy(300)
        assertFalse(s.controller.state.value.showSkipIntro)
        assertTrue(s.engine.positionMs >= 20_000)
    }

    @Test
    fun theDeviceCanHideTheSkipButton() = runTest {
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, intro = Segment(0, 20_000, Segment.Source.CHAPTER))) } }
        val s = setup(server, settings = DeviceSettings(offerSkipIntro = false))
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        advanceTimeBy(3_000)
        assertFalse(s.controller.state.value.showSkipIntro)
    }

    @Test
    fun theCountdownStartsAtTheCreditsAndPlaysTheNextEpisode() = runTest {
        val next = ref("e2", kind = ItemKind.EPISODE, season = 1, episode = 2, parent = "s1")
        val server = FakeServer().apply {
            answer = { id, _ -> ApiResult.Ok(info(id, item = ref(id, kind = ItemKind.EPISODE, season = 1, episode = 1), next = next.takeIf { id == "e1" }, outro = Segment(40_000, 60_000, Segment.Source.CHAPTER))) }
        }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(39_000)
        assertNull(s.controller.state.value.countdown)
        advanceTimeBy(1_500)
        assertEquals(Countdown(next, 10), s.controller.state.value.countdown)
        advanceTimeBy(5_000)
        assertEquals(5, s.controller.state.value.countdown?.secondsLeft)
        advanceTimeBy(5_000)
        assertEquals("e2", server.starts.last().first)
        // The first episode reported where it stood and ended its session.
        assertTrue(server.progress.any { it.first == "e1" && it.second.state == ProgressEvent.State.PAUSED })
        assertEquals(listOf("session-e1"), server.stops)
    }

    @Test
    fun aCancelledCountdownStaysAwayAndShowsTheSkipButton() = runTest {
        val next = ref("e2", kind = ItemKind.EPISODE, season = 1, episode = 2)
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, next = next, outro = Segment(40_000, 60_000, Segment.Source.CHAPTER))) } }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        advanceTimeBy(41_000)
        s.controller.cancelCountdown()
        advanceTimeBy(15_000)
        assertNull(s.controller.state.value.countdown)
        assertTrue(s.controller.state.value.showSkipOutro)
        assertEquals(1, server.starts.size)
    }

    @Test
    fun withoutAutoplayTheCardWaits() = runTest {
        val next = ref("e2", kind = ItemKind.EPISODE, season = 1, episode = 2)
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, next = next)) } }
        val s = setup(server, settings = DeviceSettings(autoplayNext = false))
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.engine.end()
        advanceTimeBy(20_000)
        assertEquals(Countdown(next, null), s.controller.state.value.countdown)
        assertEquals(1, server.starts.size)
        s.controller.playNext()
        runCurrent()
        assertEquals("e2", server.starts.last().first)
    }

    @Test
    fun theBackgroundEndsTheSessionAndTheForegroundStartsAgainPaused() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(12_500)
        s.controller.moveToBackground()
        runCurrent()
        assertEquals(ProgressEvent.State.PAUSED, s.server.progress.last().second.state)
        assertEquals(listOf("session-film"), s.server.stops)
        val calls = s.server.stateCalls
        advanceTimeBy(60_000)
        assertEquals(calls, s.server.stateCalls)
        s.controller.resumeFromBackground()
        runCurrent()
        assertEquals(12_500L, s.server.starts.last().second.positionMs)
        assertFalse(s.engine.loads.last().third)
    }

    @Test
    fun theEpisodesOfTheSeason() = runTest {
        val server = FakeServer().apply {
            answer = { id, _ -> ApiResult.Ok(info(id, item = ref(id, kind = ItemKind.EPISODE, season = 2, episode = 1, parent = "season-2"))) }
            seasonEpisodes = listOf(ref("a", episode = 1), ref("b", episode = 2))
        }
        val s = setup(server)
        s.controller.start("a")
        runCurrent()
        s.controller.loadEpisodes()
        runCurrent()
        assertEquals(SeasonEpisodes(2, server.seasonEpisodes), s.controller.state.value.episodes)
        s.controller.playEpisode("b")
        runCurrent()
        assertEquals("b", server.starts.last().first)
        assertNull(server.starts.last().second.positionMs)
    }

    // ---- the background (B-1, B-2) ------------------------------------------------------------

    @Test
    fun theCountdownStopsInTheBackgroundAndNothingStartsThere() = runTest {
        val next = ref("e2", kind = ItemKind.EPISODE, season = 1, episode = 2)
        val server = FakeServer().apply { answer = { id, r -> ApiResult.Ok(info(id, next = next.takeIf { id == "e1" }, outro = Segment(40_000, 60_000, Segment.Source.CHAPTER), resumeMs = r.positionMs ?: 0)) } }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(41_000)
        assertEquals(10, s.controller.state.value.countdown?.secondsLeft)
        s.controller.moveToBackground()
        advanceTimeBy(PlaybackController.AUTO_NEXT_MS + 5_000)
        assertEquals(listOf("e1"), server.starts.map { it.first })
        assertNull(s.controller.state.value.countdown)
        // Back: the same episode where it stood, paused.
        s.controller.resumeFromBackground()
        runCurrent()
        assertEquals("e1", server.starts.last().first)
        assertTrue(server.starts.last().second.positionMs!! in 40_000L..42_000L)
        assertFalse(s.engine.loads.last().third)
    }

    @Test
    fun aFirstStartThatAnswersInTheBackgroundNeverPlays() = runTest {
        val server = FakeServer().apply { startGate = CompletableDeferred() }
        val s = setup(server)
        s.controller.start("film", startAtMs = 7_000)
        runCurrent()
        s.controller.moveToBackground()
        server.startGate!!.complete(Unit)
        advanceTimeBy(30_000)
        assertTrue("nothing loaded in the background", s.engine.loads.isEmpty())
        server.startGate = null
        s.controller.resumeFromBackground()
        runCurrent()
        assertEquals(7_000L, server.starts.last().second.positionMs)
        assertFalse(s.engine.loads.single().third)
    }

    @Test
    fun aRestartForAnotherQualityThatAnswersInTheBackgroundNeverPlays() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(6_000)
        s.server.startGate = CompletableDeferred()
        s.controller.selectQuality(Quality._720P)
        runCurrent()
        s.controller.moveToBackground()
        s.server.startGate!!.complete(Unit)
        advanceTimeBy(30_000)
        assertEquals(1, s.engine.loads.size)
        s.server.startGate = null
        s.controller.resumeFromBackground()
        runCurrent()
        assertEquals(Quality._720P, s.server.starts.last().second.quality)
        assertEquals(6_000L, s.server.starts.last().second.positionMs)
        assertFalse(s.engine.loads.last().third)
    }

    @Test
    fun theBusyRetriesStopInTheBackground() = runTest {
        val server = FakeServer().apply { answer = { _, _ -> ApiResult.Failed(503, "transcode_busy") } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.controller.moveToBackground()
        advanceTimeBy(60_000)
        assertEquals(1, server.starts.size)
        assertTrue(s.engine.loads.isEmpty())
    }

    @Test
    fun aReconnectionDoesNotLoadInTheBackground() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.engine.emit(EngineEvent.Failed(EngineFailure.Network))
        runCurrent()
        s.controller.moveToBackground()
        advanceTimeBy(30_000)
        assertEquals(1, s.engine.loads.size)
        assertEquals(listOf("session-film"), s.server.stops)
    }

    // ---- old answers and old events (M-1, M-2) ---------------------------------------------------

    @Test
    fun aLateAnswerOfTheOldSessionDoesNotEndTheNewOne() = runTest {
        val server = FakeServer().apply { answer = { id, r -> ApiResult.Ok(info(id, session = "s-${r.quality?.value ?: "auto"}")) } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        server.stateGate = CompletableDeferred()
        advanceTimeBy(4_001)
        assertEquals(1, server.stateCalls)
        s.controller.selectQuality(Quality._720P)
        runCurrent()
        s.engine.ready()
        runCurrent()
        // The old session was stopped by the restart; its answer arrives now.
        server.state = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateStopped, PlaybackSessionState.StoppedBy.PlaybackStoppedByClient))
        server.stateGate!!.complete(Unit)
        runCurrent()
        assertNull(s.controller.state.value.error)
        server.stateGate = null
        server.state = ApiResult.Ok(PlaybackSessionState(PlaybackSessionState.State.PlaybackStateActive))
        s.controller.close()
        runCurrent()
        assertEquals(listOf("s-auto", "s-720p"), server.stops)
    }

    @Test
    fun theEmptyTracksOfStopDoNotTakeTheNewLoad() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng"))
        val subs = listOf(subtitle(20, 3, "deu"))
        val server = FakeServer().apply {
            answer = { id, r -> ApiResult.Ok(info(id, audio = tracks, subtitles = subs, selection = TrackSelection(audioTrack = 2, subtitleTrack = 20), session = "s-${r.quality?.value ?: "auto"}")) }
        }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        val groups = listOf(group("de"), group("en", selected = true))
        val text = listOf(EngineTrack("de", null, "application/x-subrip"))
        s.engine.ready(groups, text)
        runCurrent()
        assertEquals(listOf(1), s.engine.audioSelections)
        assertEquals(listOf<Int?>(0), s.engine.textSelections)
        // A new quality: stop() sends empty tracks first; they must not count as the new file's.
        s.controller.selectQuality(Quality._720P)
        runCurrent()
        assertEquals(2, server.starts.size)
        s.engine.ready(groups, text)
        runCurrent()
        assertEquals("no third start from empty groups", 2, server.starts.size)
        assertEquals(listOf(1, 1), s.engine.audioSelections)
        assertEquals(listOf<Int?>(0, 0), s.engine.textSelections)
        // A late failure of the first load means nothing for the second.
        s.engine.emit(EngineEvent.Failed(EngineFailure.VideoDecoder), load = 1)
        runCurrent()
        assertEquals(2, server.starts.size)
        assertNull(s.controller.state.value.error)
    }

    // ---- busy, settings, played time, countdown, choices (M-4, m-1 to m-6, L-5, L-7) ----------

    @Test
    fun aBusyServerAfterTheLastAttemptOffersRetry() = runTest {
        val server = FakeServer().apply { answer = { _, _ -> ApiResult.Failed(503, "transcode_busy") } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        assertEquals(PlayerError.Busy(retrying = true), s.controller.state.value.error)
        advanceTimeBy(PlaybackController.BUSY_RETRY_MS * (PlaybackController.BUSY_ATTEMPTS + 2))
        assertEquals(PlaybackController.BUSY_ATTEMPTS + 1, server.starts.size)
        assertEquals(PlayerError.Busy(retrying = false), s.controller.state.value.error)
        server.answer = { id, _ -> ApiResult.Ok(info(id)) }
        s.controller.retry()
        runCurrent()
        assertNull(s.controller.state.value.error)
    }

    @Test
    fun theFirstStartWaitsForTheStoredSettings() = runTest {
        val stored = MutableSharedFlow<DeviceSettings>(replay = 1)
        val clock = { testScheduler.currentTime }
        val engine = FakeEngine(clock)
        val server = FakeServer()
        val controller = PlaybackController(backgroundScope, backgroundScope, server, engine, { CapabilityRules.build(emulator, narrowing = it) }, stored, { false }, clock)
        runCurrent()
        controller.start("film")
        runCurrent()
        assertTrue("no start before the settings are known", server.starts.isEmpty())
        stored.emit(DeviceSettings(qualityWifi = QualityChoice.Low))
        runCurrent()
        assertEquals(QualityCap._480P, server.starts.single().second.qualityCap)
    }

    @Test
    fun aRestartReportsThePlayedTimeBeforeItStopsTheSession() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(7_000)
        s.controller.selectQuality(Quality._720P)
        runCurrent()
        val last = s.server.progress.single().second
        assertEquals(7_000L, last.playedMs)
        assertEquals(listOf("progress:session-film", "stop:session-film"), s.server.calls)
    }

    @Test
    fun leavingSendsTheProgressBeforeTheStop() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(3_000)
        s.controller.close()
        runCurrent()
        assertEquals(listOf("progress:session-film", "stop:session-film"), s.server.calls)
    }

    @Test
    fun thePausedCountdownStands() = runTest {
        val next = ref("e2", kind = ItemKind.EPISODE, season = 1, episode = 2)
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, next = next.takeIf { id == "e1" }, outro = Segment(40_000, 60_000, Segment.Source.CHAPTER))) } }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(43_000)
        s.controller.pause()
        runCurrent()
        val left = s.controller.state.value.countdown?.secondsLeft
        advanceTimeBy(30_000)
        assertEquals(1, server.starts.size)
        assertEquals(left, s.controller.state.value.countdown?.secondsLeft)
        s.controller.play()
        runCurrent()
        advanceTimeBy(8_000)
        assertEquals("e2", server.starts.last().first)
    }

    @Test
    fun whatIsChosenAlreadyStartsNothing() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, direct = false, audio = tracks)) } }
        val s = setup(server)
        s.controller.start("film")
        runCurrent()
        s.engine.ready(audio = listOf(group("de", selected = true)))
        runCurrent()
        s.controller.selectAudio(tracks[0])
        s.controller.selectSubtitle(null)
        s.controller.selectQuality(Quality.AUTO)
        runCurrent()
        assertEquals(1, server.starts.size)
    }

    @Test
    fun theTrackChoiceOfOneFileDoesNotNarrowTheNextEpisode() = runTest {
        val tracks = listOf(audio(10, 1, "deu"), audio(11, 2, "eng"))
        val server = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(info(id, audio = tracks, selection = TrackSelection(audioTrack = 2))) } }
        val s = setup(server)
        s.controller.start("e1")
        runCurrent()
        s.engine.ready(audio = listOf(group("fr"), group("it", selected = true)))
        runCurrent()
        assertEquals(false, server.starts.last().second.capabilities!!.selectsTracks)
        s.controller.playEpisode("e2")
        runCurrent()
        assertEquals(true, server.starts.last().second.capabilities!!.selectsTracks)
    }

    @Test
    fun anUnreadableContainerStartsOnceWithoutIt() = runTest {
        val s = setup()
        s.controller.start("film")
        runCurrent()
        s.engine.emit(EngineEvent.Failed(EngineFailure.Container))
        runCurrent()
        assertFalse("mkv" in s.server.starts.last().second.capabilities!!.containers!!)
        s.engine.emit(EngineEvent.Failed(EngineFailure.Container))
        runCurrent()
        assertEquals(2, s.server.starts.size)
        assertEquals(PlayerError.Unplayable, s.controller.state.value.error)
    }

    @Test
    fun twoQuickEpisodeChangesEndEachSessionInOrder() = runTest {
        val s = setup()
        s.controller.start("e1")
        runCurrent()
        s.engine.ready()
        runCurrent()
        advanceTimeBy(2_000)
        s.controller.playEpisode("e2")
        s.controller.playEpisode("e3")
        runCurrent()
        // The start of e2 never went out: e3 took its place before the server was asked.
        assertEquals(listOf("e1", "e3"), s.server.starts.map { it.first })
        assertEquals(listOf("progress:session-e1", "stop:session-e1"), s.server.calls)
        assertEquals(1, s.engine.loads.count { it.first.title == "Title e3" })
    }

    @Test
    fun theStateLeavesTheStreamAddressesOut() {
        val state = PlayerState(info = info("film"))
        assertFalse(state.toString(), state.toString().contains("st="))
        assertFalse(EngineMedia("http://server.test/a?st=secret", hls = false).toString().contains("secret"))
        assertFalse(TrickplayTile("http://server.test/t?st=secret", 0, 0, 1, 1, 0).toString().contains("secret"))
    }

    /**
     * Media3 tells what it wants while load() runs; with the immediate main dispatcher of a page the
     * controller hears it at once. The number of the load must be known before (else pause played).
     */
    @Test
    fun theWishToPlayDuringTheLoadCounts() = runTest {
        val clock = { testScheduler.currentTime }
        val engine = FakeEngine(clock)
        val server = FakeServer()
        val immediate = CoroutineScope(backgroundScope.coroutineContext + UnconfinedTestDispatcher(testScheduler))
        val controller = PlaybackController(immediate, backgroundScope, server, engine, { CapabilityRules.build(emulator, narrowing = it) }, MemoryDeviceSettingsStore().settings, { false }, clock)
        controller.start("film")
        runCurrent()
        assertTrue(engine.wantsPlay)
        controller.togglePlay()
        assertFalse("the toggle paused", engine.wantsPlay)
    }
}
