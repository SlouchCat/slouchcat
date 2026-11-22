package com.slouchcat.app.mobile.home

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeLibraries
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.browse.testing.library
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/**
 * The real feature in the real NavHost (rule 8, «Zurück führt hinaus»): «Start» is the first
 * page after the profile choice; a library from the media choice returns to it with Back, and
 * Back on «Start» leaves the app. The server is not reachable here: the page shows its error.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class HomeBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val graph = AppGraph(
        ApplicationProvider.getApplicationContext<Context>(),
        features = listOf(HomeFeature, MoreFeature),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        librariesRepository = FakeLibraries(ApiResult.Ok(listOf(library(4, "Test films")))),
    )

    private fun waitFor(matcher: androidx.compose.ui.test.SemanticsMatcher) =
        compose.waitUntil(10_000) { compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }

    private fun back() = compose.runOnUiThread { compose.activity.onBackPressedDispatcher.onBackPressed() }

    private fun backLeaves() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText(s(R.string.app_name)) and isHeading())
        waitFor(hasText("Test films"))
        // Phone: through the menu; from 700 dp the side column shows the entry.
        val menu = compose.onAllNodes(androidx.compose.ui.test.hasContentDescription(s(R.string.app_shell_menuOpen)) and hasClickAction())
        if (menu.fetchSemanticsNodes().isNotEmpty()) menu.onFirst().performClick()
        compose.onAllNodes(hasText("Test films") and hasClickAction()).onFirst().performClick()
        waitFor(hasText("Test films") and isHeading())
        back()
        waitFor(hasText(s(R.string.app_name)) and isHeading())
        compose.waitForIdle()
        assertFalse(compose.activity.isFinishing)
        back()
        compose.waitForIdle()
        assertTrue(compose.activity.isFinishing)
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backLeavesPhone() = backLeaves()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun backLeavesTablet() = backLeaves()
}
