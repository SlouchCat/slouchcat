package com.slouchcat.app.mobile.item

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.person.PersonFeature
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeDetails
import com.slouchcat.app.mobile.testing.FakePeople
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.film
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable internal data object DetailStartRoute

/** A start page that is only a text: the place the detail pages are opened from. */
internal object DetailStartFeature : Feature {
    override val destinations = listOf(TopDestination("start", 10, DetailStartRoute, R.string.nav_home, SlouchcatIcon.Home))

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<DetailStartRoute> { Text("start page") }
    }
}

/**
 * «Zurück führt hinaus» (rule 8) through the real NavHost: an in-app jump `item/<id>` opens the
 * film under «Start», a cast card the person; Back walks back to the film, to «Start», out.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class DetailBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()

    private val graph = AppGraph(
        context,
        features = listOf(DetailStartFeature, ItemFeature { FakeDetails(film()) }, PersonFeature { FakePeople() }),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
    )

    private fun waitFor(matcher: androidx.compose.ui.test.SemanticsMatcher) =
        compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun finishing(): Boolean {
        var result = false
        compose.activityRule.scenario.onActivity { result = it.isFinishing }
        return result
    }

    private fun backLeadsOut() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("start page"))
        assertTrue(graph.links.request("item/${ulid(1)}"))
        assertFalse("an unknown address is no jump", graph.links.request("item/"))
        waitFor(hasText("A new day begins."))
        compose.onNode(hasContentDescription("Ann Actor", substring = true) and hasClickAction()).performScrollTo().performClick()
        waitFor(hasText("Ann Actor") and isHeading())
        back()
        waitFor(hasText("A new day begins."))
        // The page's own arrow does the same.
        compose.onNode(hasContentDescription(context.getString(R.string.common_back)) and hasClickAction()).performClick()
        waitFor(hasText("start page"))
        back()
        assertTrue(finishing())
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backLeadsOutPhone() = backLeadsOut()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun backLeadsOutTablet() = backLeadsOut()
}
