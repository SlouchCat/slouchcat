package com.slouchcat.app.tv.player

import com.slouchcat.app.tv.testing.EMULATOR_TV
import com.slouchcat.app.tv.testing.FakeDetails
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.app.tv.testing.tvServer
import com.slouchcat.app.tv.testing.SERIES_ID
import com.slouchcat.app.tv.testing.film
import com.slouchcat.app.tv.testing.playbackInfo
import com.slouchcat.app.tv.testing.playerEpisode
import com.slouchcat.app.tv.testing.series
import com.slouchcat.app.tv.testing.ulid
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.Segment
import com.slouchcat.core.player.AudioSetting
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceSettings
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.PlaybackController
import com.slouchcat.core.player.PlayerError
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.media.data.ItemRepository
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import okhttp3.HttpUrl
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * The TV's player page with `:core:player` against the attrappe that behaves like Media3: a series
 * plays its next episode, the background stops everything (no start, no countdown), a new output
 * makes the server decide again, a profile change ends the playback at the server.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class PlayerViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun main() = Dispatchers.setMain(dispatcher)

    @After
    fun reset() = Dispatchers.resetMain()

    private class Setup(
        val vm: PlayerViewModel,
        val engine: FakeEngine,
        val server: FakeServer,
        val details: FakeDetails,
        val outputs: MutableSharedFlow<Unit>,
        val store: MemoryDeviceSettingsStore,
    ) {
        var facts: DeviceFacts = EMULATOR_TV
    }

    private fun TestScope.setup(
        id: String,
        details: FakeDetails = FakeDetails(film()),
        server: FakeServer = tvServer(),
        settings: DeviceSettings = DeviceSettings(),
        items: ItemRepository = details,
    ): Setup {
        val clock = { testScheduler.currentTime }
        val engine = FakeEngine(clock)
        val outputs = MutableSharedFlow<Unit>(extraBufferCapacity = 4)
        val store = MemoryDeviceSettingsStore(settings)
        lateinit var setup: Setup
        val vm = PlayerViewModel(
            itemId = id,
            startAtMs = null,
            fromStart = false,
            engine = engine,
            server = server,
            items = items,
            serverUrl = { "https://media.example.test/".toHttpUrl() },
            facts = { setup.facts },
            settings = store.settings,
            outputChanges = outputs,
            metered = { false },
            finalScope = backgroundScope,
            clock = clock,
            runIn = backgroundScope,
        )
        setup = Setup(vm, engine, server, details, outputs, store)
        runCurrent()
        return setup
    }

    @Test
    fun aSeriesPlaysTheEpisodeNextUpNames() = runTest(dispatcher) {
        val s = setup(SERIES_ID, details = FakeDetails(series()))
        // The stage of Start opens the series itself; the player plays «Cowardice» (S1 E3).
        val next = series().nextUp!!.episode.id
        assertEquals(listOf(next), s.server.starts.map { it.first })
        assertEquals(PlaybackCapabilities.Type.TV, s.server.starts.single().second.capabilities!!.type)
        assertEquals(PlaybackCapabilities.Hls.MSE, s.server.starts.single().second.capabilities!!.hls)
    }

    @Test
    fun aFilmStartsAtOnceWithTheTvsReport() = runTest(dispatcher) {
        val s = setup(ulid(1))
        val request = s.server.starts.single().second
        assertEquals(ulid(1), s.server.starts.single().first)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, request.capabilities!!.maxAudioChannels)
        assertEquals(true, request.capabilities!!.selectsTracks)
        assertEquals(SoundOutput(emptyList(), 2), s.vm.sound.value.output)
    }

    @Test
    fun aSeriesWithoutAnEpisodeOrAGoneTitleShowsNotFoundAndRetries() = runTest(dispatcher) {
        val s = setup(ulid(9), details = FakeDetails())
        assertEquals(PlayerError.NotFound, s.vm.state.value.error)
        assertTrue(s.server.starts.isEmpty())
        // The title is there again: «Erneut versuchen» starts it.
        s.details.details[ulid(9)] = ApiResult.Ok(film(9))
        s.vm.onRetry()
        runCurrent()
        assertNull(s.vm.state.value.error)
        assertEquals(listOf(ulid(9)), s.server.starts.map { it.first })
    }

    @Test
    fun theBackgroundEndsTheSessionAndComesBackPaused() = runTest(dispatcher) {
        val s = setup(ulid(1))
        s.engine.ready()
        runCurrent()
        advanceTimeBy(30_000)
        s.vm.onBackground()
        runCurrent()
        assertEquals(listOf("session-${ulid(1)}"), s.server.stops)
        // The player is stopped and empty (Media3 keeps playWhenReady, nothing plays).
        assertTrue(s.engine.stops > 0)
        assertEquals(0L, s.engine.positionMs)
        assertFalse(s.vm.state.value.playing)
        // Nothing starts in the background, however long it lasts.
        advanceTimeBy(60_000)
        assertEquals(1, s.server.starts.size)
        s.vm.onForeground()
        runCurrent()
        assertEquals(2, s.server.starts.size)
        val (_, at, play) = s.engine.loads.last()
        assertTrue("back where it stood: $at", at in 29_000L..31_000L)
        assertFalse("back paused", play)
    }

    @Test
    fun noCountdownAndNoNextEpisodeInTheBackground() = runTest(dispatcher) {
        val server = tvServer()
        val ep = playerEpisode(2)
        server.answer = { id, _ ->
            ApiResult.Ok(if (id == ep.id) playbackInfo(id, ep, next = playerEpisode(3), outro = Segment(50_000, 60_000, Segment.Source.CHAPTER)) else playbackInfo(id, playerEpisode(3)))
        }
        val s = setup(ep.id, details = FakeDetails(film(1).copy(id = ep.id)), server = server)
        s.engine.ready()
        runCurrent()
        s.vm.onSeekTo(51_000)
        advanceTimeBy(PlaybackController.TICK_MS * 2)
        assertTrue("the card of the next episode", s.vm.state.value.countdown != null)
        s.vm.onBackground()
        advanceTimeBy(PlaybackController.AUTO_NEXT_MS * 2)
        assertEquals(listOf(ep.id), s.server.starts.map { it.first })

        // Back in the foreground (paused): once it plays, the countdown starts the next episode.
        s.vm.onForeground()
        runCurrent()
        s.engine.ready()
        runCurrent()
        s.vm.onPlay()
        s.vm.onSeekTo(51_000)
        advanceTimeBy(PlaybackController.AUTO_NEXT_MS + PlaybackController.TICK_MS * 4)
        assertEquals(playerEpisode(3).id, s.server.starts.last().first)
    }

    @Test
    fun aNewSoundSystemMakesTheServerDecideAgain() = runTest(dispatcher) {
        val s = setup(ulid(1))
        s.engine.ready()
        runCurrent()
        advanceTimeBy(5_000)
        // The receiver is switched on: Dolby Digital passes through now.
        s.facts = EMULATOR_TV.copy(passthrough = setOf("ac3", "eac3"))
        s.outputs.tryEmit(Unit)
        s.outputs.tryEmit(Unit)
        advanceTimeBy(1_500)
        assertEquals(2, s.server.starts.size)
        val again = s.server.starts.last().second
        assertEquals(PlaybackCapabilities.MaxAudioChannels._6, again.capabilities!!.maxAudioChannels)
        assertTrue(again.capabilities!!.audio!!.containsAll(listOf("ac3", "eac3")))
        assertTrue("where it stood: ${again.positionMs}", again.positionMs!! in 4_000L..6_000L)
        assertEquals(SoundOutput(listOf("ac3", "eac3"), 6), s.vm.sound.value.output)
    }

    @Test
    fun withAlwaysStereoAnOutputChangesNothing() = runTest(dispatcher) {
        val s = setup(ulid(1), settings = DeviceSettings(audio = AudioSetting.Stereo))
        s.engine.ready()
        runCurrent()
        s.facts = EMULATOR_TV.copy(passthrough = setOf("ac3"), outputChannels = 6)
        s.outputs.tryEmit(Unit)
        advanceTimeBy(1_500)
        assertEquals(1, s.server.starts.size)
        assertEquals(PlaybackCapabilities.MaxAudioChannels._2, s.server.starts.single().second.capabilities!!.maxAudioChannels)
    }

    @Test
    fun aProfileChangeEndsThePlaybackAtTheServer() = runTest(dispatcher) {
        val s = setup(ulid(1))
        s.engine.ready()
        runCurrent()
        advanceTimeBy(12_000)
        s.vm.end()
        runCurrent()
        assertEquals(listOf("session-${ulid(1)}"), s.server.stops)
        assertTrue(s.engine.released)
        // The last progress went before the stop.
        assertTrue(s.server.progress.isNotEmpty())
        // A second end (the page goes as well) does nothing more.
        s.vm.end()
        runCurrent()
        assertEquals(1, s.server.stops.size)
    }

    @Test
    fun homeWhileTheTitleIsResolvedComesBackPausedNotLoadingForever() = runTest(dispatcher) {
        val details = FakeDetails(film())
        val answer = CompletableDeferred<Unit>()
        // getItem still under way when Home is pressed (review M-1).
        val slow = object : ItemRepository by details {
            override suspend fun item(server: HttpUrl, id: String): ApiResult<ItemDetail> {
                answer.await()
                return details.item(server, id)
            }
        }
        val s = setup(ulid(1), details = details, items = slow)
        s.vm.onBackground()
        answer.complete(Unit)
        runCurrent()
        // Nothing started in the background: no session that lives a second.
        assertTrue(s.server.starts.isEmpty())
        assertTrue(s.server.stops.isEmpty())
        s.vm.onForeground()
        runCurrent()
        assertEquals(listOf(ulid(1)), s.server.starts.map { it.first })
        assertFalse("comes paused", s.engine.loads.single().third)
        assertFalse("no endless ring", s.vm.state.value.loading)
    }
}
