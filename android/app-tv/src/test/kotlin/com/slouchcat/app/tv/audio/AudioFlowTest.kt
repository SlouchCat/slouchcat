package com.slouchcat.app.tv.audio

import android.view.KeyEvent
import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.lifecycle.Lifecycle
import com.slouchcat.app.tv.player.PlayerFeature
import com.slouchcat.app.tv.shell.ShellTags
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.TestAudio
import com.slouchcat.app.tv.testing.TestPlayerFeature
import com.slouchcat.app.tv.testing.withoutAudio
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeRadio
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.homeRow
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.posters
import com.slouchcat.core.browse.testing.progress
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import org.robolectric.shadows.ShadowLooper
import java.util.concurrent.TimeUnit

/**
 * The sound of the TV over the pages (concept 26 5.6, mockup `mdSync`, `mdRailNow`): a book plays
 * on after Back with «Läuft gerade» at the top right and in the rail, which leads back without a
 * new start; Next on the remote changes the chapter; Home ends the session and the book comes back
 * paused; a video pauses it; a profile change ends a book and a station with their last reports;
 * the station's heartbeat runs while it plays.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class AudioFlowTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val audio = TestAudio()
    private val video = TestPlayerFeature()
    private val bookRef = itemRef(40, title = "The Quiet Harbour", kind = ItemKind.AUDIOBOOK, progress = progress(10, minutes = 40), images = posters(ulid(40)))
    private val home = FakeHome(
        ApiResult.Ok(
            HomePage(
                rows = listOf(
                    homeRow(HomeRowKey.SPOTLIGHT, listOf(itemRef(1, title = "Night Train", images = posters(ulid(1))))),
                    homeRow(HomeRowKey.CONTINUE, listOf(bookRef)),
                ),
                heroRow = HomeRowKey.SPOTLIGHT,
            ),
        ),
    )
    private val radio = FakeRadio()

    private fun s(id: Int, vararg args: Any) = pages.s(id, *args)

    private fun open() = pages.show(
        { it.copy(home = home, detail = FakeDetails(film(1, title = "Night Train")), radio = radio) },
        features = withoutAudio.filterNot { it is PlayerFeature } + video + audio.feature,
    )

    /** Start, down to «Weiterschauen», OK on the book: the audio player plays it. */
    private fun playBook() {
        open()
        pages.waitFor("Night Train")
        remote.down()
        remote.assertFocused(bookRef.title)
        remote.ok()
        pages.waitFor(s(R.string.player_seek))
        audio.engine.ready()
        remote.settle()
    }

    private fun idle(ms: Long) {
        compose.mainClock.advanceTimeBy(ms)
        ShadowLooper.idleMainLooper(ms, TimeUnit.MILLISECONDS)
        remote.settle()
    }

    private fun media(code: Int) {
        compose.runOnUiThread { pages.graph.remoteKeys.dispatch(KeyEvent(KeyEvent.ACTION_DOWN, code)) }
        remote.settle()
    }

    @Test
    fun theBookPlaysOnAcrossThePagesAndTheRailLeadsBack() {
        playBook()
        remote.back()
        pages.waitFor("Night Train")
        // «Läuft gerade» at the top right and in the rail.
        // The indicator and the rail entry.
        assertEquals(2, compose.onAllNodes(hasText(s(R.string.tv_audio_nowPlaying))).fetchSemanticsNodes().size)
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.NOW))).assertExists()
        assertTrue(audio.engine.wantsPlay)
        assertTrue(audio.titles.stops.isEmpty())
        pages.openRail(s(R.string.tv_audio_nowPlaying))
        pages.waitFor(s(R.string.player_seek))
        // The same book, no new start.
        assertEquals(1, audio.titles.starts.size)
        assertTrue(audio.engine.wantsPlay)
    }

    @Test
    fun nextAndPreviousOfTheRemoteChangeTheChapter() {
        playBook()
        media(KeyEvent.KEYCODE_MEDIA_NEXT)
        assertTrue(audio.engine.seeks.toString(), audio.chapters[1].startMs in audio.engine.seeks)
        media(KeyEvent.KEYCODE_MEDIA_PREVIOUS)
        assertEquals(audio.chapters[0].startMs, audio.engine.seeks.last())
    }

    @Test
    fun homeEndsTheSessionAndTheBookComesBackPaused() {
        playBook()
        compose.activityRule.scenario.moveToState(Lifecycle.State.CREATED)
        idle(AudioForeground.GRACE_MS * 2)
        assertEquals(1, audio.titles.stops.size)
        compose.activityRule.scenario.moveToState(Lifecycle.State.RESUMED)
        idle(FRAME)
        assertEquals(2, audio.titles.starts.size)
        assertFalse(audio.engine.wantsPlay)
    }

    @Test
    fun aVideoPausesTheBook() {
        playBook()
        remote.back()
        pages.waitFor("Night Train")
        remote.up()
        remote.assertFocused(s(R.string.item_play))
        remote.ok()
        idle(FRAME)
        assertEquals(1, video.played.size)
        assertFalse(audio.engine.wantsPlay)
    }

    @Test
    fun aProfileChangeEndsTheBookWithItsLastProgress() {
        val graph = run { playBook(); pages.graph }
        runBlocking { graph.hooks.clean() }
        compose.waitForIdle()
        idle(FRAME)
        assertEquals(1, audio.titles.stops.size)
        assertTrue(audio.titles.progress.isNotEmpty())
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.NOW))).assertDoesNotExist()
    }

    @Test
    fun aStationBeatsWhileItPlaysAndEndsWithTheProfile() {
        audio.radio.heartbeatSeconds = HEARTBEAT_S
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        remote.ok()
        audio.engine.ready()
        idle(HEARTBEAT_S * 1_000L + FRAME)
        assertTrue(audio.radio.calls.toString(), audio.radio.calls.any { it.startsWith("beat:") })
        // «Jetzt läuft» opens the station's page; Back returns, the station plays on.
        pages.waitFor(s(R.string.radio_player_open, "Testradio 2"))
        compose.onNode(hasTestTag(RadioKeys.NOW)).assertExists()
        runBlocking { pages.graph.hooks.clean() }
        compose.waitForIdle()
        idle(FRAME)
        assertTrue(audio.radio.calls.toString(), audio.radio.calls.any { it.startsWith("end:") })
    }

    @Test
    fun theStationsPageHasPlayHeartAndStop() {
        open()
        pages.openRail(s(R.string.app_radio_title))
        pages.waitFor("Radio 2")
        remote.ok()
        audio.engine.ready()
        pages.waitFor(s(R.string.radio_player_open, "Testradio 2"))
        remote.up(3)
        remote.assertFocused(s(R.string.radio_player_open, "Testradio 2"))
        remote.ok()
        pages.waitFor(s(R.string.radio_player_stop))
        remote.assertFocused(s(R.string.radio_player_pause))
        remote.assertEveryControlReachable()
        remote.right()
        remote.ok()
        assertTrue(radio.calls.toString(), "favorite:2:true" in radio.calls)
        remote.right()
        remote.ok()
        // «Radio beenden»: the station ends, the page closes.
        pages.waitFor("Radio 2")
        assertTrue(audio.radio.calls.any { it.startsWith("end:") })
        compose.onNode(hasTestTag(ShellTags.entry(AudioFeature.NOW))).assertDoesNotExist()
    }

    private companion object {
        const val FRAME = 500L
        const val HEARTBEAT_S = 5
    }
}
