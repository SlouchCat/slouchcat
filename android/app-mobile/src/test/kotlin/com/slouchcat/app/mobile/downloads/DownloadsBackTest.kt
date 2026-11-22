package com.slouchcat.app.mobile.downloads

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
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeOfflineServer
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.testEngine
import com.slouchcat.app.mobile.testing.testOfflineData
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.junit.After
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** «Downloads» as a target of the real NavHost: it opens from the bar, and Back leads out of the app. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DownloadsBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    @get:Rule val temp = TemporaryFolder()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    private val graph by lazy {
        AppGraph(
            context,
            features = listOf(MoreFeature, DownloadsFeature),
            serverStore = FakeServers(HTTPS_SERVER),
            tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
            accountStore = FakeAccounts(EMAIL, PRIMARY),
            offlineData = { testOfflineData(testEngine(context, temp.newFolder("offline"), FakeOfflineServer(), scope, { com.slouchcat.app.mobile.testing.offlineIdentity(PRIMARY.id) })) },
        )
    }

    @After
    fun tearDown() = scope.cancel()

    private fun waitFor(matcher: SemanticsMatcher) {
        compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun backLeadsOut() {
        compose.setContent { SlouchcatNavHost(graph) }
        // «Downloads» comes before «Mehr»: the first target, where the app lands after the profile.
        waitFor(hasText(context.getString(R.string.app_downloads_emptyTitle)))
        compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
        compose.waitForIdle()
        compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
        var finishing = false
        compose.activityRule.scenario.onActivity { finishing = it.isFinishing }
        assertTrue(finishing)
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backLeadsOutPhone() = backLeadsOut()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun backLeadsOutTablet() = backLeadsOut()
}
