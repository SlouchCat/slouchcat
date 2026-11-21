package com.slouchcat.app.mobile.player

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.item.DetailStartFeature
import com.slouchcat.app.mobile.item.ItemFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeDetails
import com.slouchcat.app.mobile.testing.FakePlaybackServer
import com.slouchcat.app.mobile.testing.FakePlayerEngine
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.film
import com.slouchcat.app.mobile.testing.progress
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import kotlin.coroutines.EmptyCoroutineContext
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * «Zurück führt hinaus» (rule 8) through the real NavHost: «Weiterschauen» of a film opens the
 * player, Back ends the playback (the session is stopped, the player released) and returns to the
 * detail page, which reads the title again; «Von vorn» starts at 0.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PlayerBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val details = FakeDetails(film(progress = progress(40)))
    private val server = FakePlaybackServer()
    private val engines = mutableListOf<FakePlayerEngine>()

    private val graph = AppGraph(
        context,
        features = listOf(
            DetailStartFeature,
            ItemFeature { details },
            // Everything on the main thread: the test waits for the idle main looper, not for a clock.
            PlayerFeature(
                engine = { _, _ -> FakePlayerEngine().also { engines += it } },
                server = { server },
                settings = { MemoryDeviceSettingsStore() },
                facts = { _, type -> DeviceFacts(emptyList(), emptySet(), 2, DisplayInfo(2424, 1080), type, "Test phone") },
                background = EmptyCoroutineContext,
            ),
        ),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
    )

    /** The state after the main looper is idle (no fetch, no thread of its own in this test): no clock. */
    private fun waitFor(matcher: SemanticsMatcher) {
        compose.waitForIdle()
        compose.onAllNodes(matcher).fetchSemanticsNodes().also { assertTrue("not shown: $matcher", it.isNotEmpty()) }
    }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun finishing(): Boolean {
        var result = false
        compose.activityRule.scenario.onActivity { result = it.isFinishing }
        return result
    }

    private val closePlayer = hasContentDescription(context.getString(R.string.app_player_close)) and hasClickAction()

    private fun backLeavesThePlayer() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("start page"))
        assertTrue(graph.links.request("item/${ulid(1)}"))
        waitFor(hasText("A new day begins."))
        val loads = details.asked.size
        compose.reveal(compose.onNode(hasText(context.getString(R.string.app_item_resume)) and hasClickAction())).performClick()
        waitFor(closePlayer)
        assertEquals(listOf(ulid(1)), server.starts)
        assertEquals(null, server.requests.single().positionMs)
        back()
        waitFor(hasText("A new day begins."))
        compose.waitForIdle()
        assertTrue(engines.single().released)
        assertEquals(listOf("session-${ulid(1)}"), server.stops)
        // The detail page read the title again for its new progress.
        compose.waitForIdle()
        assertTrue(details.asked.size > loads)

        // «Von vorn»: the player from the beginning; its own arrow leads back.
        compose.reveal(compose.onNode(hasText(context.getString(R.string.app_item_fromStart)) and hasClickAction())).performClick()
        waitFor(closePlayer)
        assertEquals(2, server.requests.size)
        assertEquals(0L, server.requests.last().positionMs)
        compose.onNode(closePlayer).performClick()
        waitFor(hasText("A new day begins."))
        back()
        waitFor(hasText("start page"))
        back()
        assertTrue(finishing())
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backLeavesThePlayerPhone() = backLeavesThePlayer()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun backLeavesThePlayerTablet() = backLeavesThePlayer()
}
