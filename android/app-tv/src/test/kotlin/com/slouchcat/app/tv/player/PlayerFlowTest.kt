package com.slouchcat.app.tv.player

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.EMULATOR_TV
import com.slouchcat.app.tv.testing.FakeDetails
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.app.tv.testing.FakeHome
import com.slouchcat.app.tv.testing.tvServer
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.SERIES_ID
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.film
import com.slouchcat.app.tv.testing.homeRow
import com.slouchcat.app.tv.testing.itemRef
import com.slouchcat.app.tv.testing.posters
import com.slouchcat.app.tv.testing.progress
import com.slouchcat.app.tv.testing.series
import com.slouchcat.app.tv.testing.ulid
import com.slouchcat.app.tv.testing.withoutPlayer
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import kotlin.coroutines.EmptyCoroutineContext

/**
 * The player in the real shell and NavHost (rule 8: Zurück führt hinaus): «Abspielen» on the stage
 * of Start (a series plays its next episode), Zurück hides the controls and then leaves, the
 * session ends at the server, Start reads its rows again; a profile change stops a playback first.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class PlayerFlowTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val server = tvServer()
    private val engines = mutableListOf<FakeEngine>()
    private val details = FakeDetails(film(1, title = "Night Train", progress = progress(40, minutes = 145)), series())
    private val home = FakeHome(
        ApiResult.Ok(
            HomePage(
                rows = listOf(
                    homeRow(HomeRowKey.SPOTLIGHT, listOf(itemRef(200, title = "Quiet Ring", kind = ItemKind.SERIES, images = posters(SERIES_ID)).copy(id = SERIES_ID))),
                    homeRow(HomeRowKey.CONTINUE, listOf(itemRef(1, title = "Night Train", progress = progress(40), images = posters(ulid(1))))),
                ),
                heroRow = HomeRowKey.SPOTLIGHT,
            ),
        ),
    )

    private val player = PlayerFeature(
        engine = { _, _ -> FakeEngine { 0L }.also { engines += it } },
        server = { server },
        deviceSettings = { MemoryDeviceSettingsStore() },
        facts = { _, _ -> EMULATOR_TV },
        outputs = { emptyFlow() },
        // Everything on the main thread: the test waits for the idle main looper.
        background = EmptyCoroutineContext,
    )

    private fun show() = pages.show({ it.copy(home = home, detail = details, items = details) }, features = withoutPlayer + player)

    @Test
    fun abspielenOnTheStagePlaysTheNextEpisodeAndBackLeavesInTwoSteps() {
        show()
        pages.waitFor("Quiet Ring")
        remote.assertFocused(pages.s(R.string.item_play))
        val loads = home.asked.size
        remote.ok()
        pages.waitFor(pages.s(R.string.player_seek))
        // The series id resolved to «Cowardice» (S1 E3) through nextUp.
        assertEquals(listOf(series().nextUp!!.episode.id), server.starts.map { it.first })
        remote.assertFocused(pages.s(R.string.player_seek))
        // Zurück hides the controls, the page stays.
        remote.back()
        assertTrue(server.stops.isEmpty())
        // Zurück again leaves: the session ends, the player is released, Start reads its rows again.
        remote.back()
        pages.waitFor("Night Train")
        compose.waitForIdle()
        assertEquals(listOf("session-${series().nextUp!!.episode.id}"), server.stops)
        assertTrue(engines.single().released)
        assertTrue("Start reads again: ${home.asked}", home.asked.size > loads)
        remote.assertFocused(pages.s(R.string.item_play))
    }

    @Test
    fun aProfileChangeStopsThePlaybackFirst() {
        val graph = show()
        pages.waitFor("Quiet Ring")
        remote.ok()
        pages.waitFor(pages.s(R.string.player_seek))
        runBlocking { graph.hooks.clean() }
        compose.waitForIdle()
        assertEquals(1, server.stops.size)
        assertTrue(engines.single().released)
    }
}
