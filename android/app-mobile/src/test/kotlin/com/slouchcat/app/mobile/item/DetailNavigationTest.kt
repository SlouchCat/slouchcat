package com.slouchcat.app.mobile.item

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performClick
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.SEARCH_TARGET
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.person.PersonFeature
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakePeople
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** A search target that is only a text. */
internal object TextSearchFeature : Feature {
    override val destinations = listOf(TopDestination(SEARCH_TARGET, 20, SearchRoute(), R.string.nav_search, SlouchcatIcon.Search))

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<SearchRoute> { Text("search page") }
    }
}

/**
 * The navigation of a detail page through the real NavHost (code review M-1, M-2): a double tap
 * on a card opens one page, not two; the search of a title opens the target «Suche», marked in the
 * bar, not a page under «Start».
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class DetailNavigationTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val people = FakePeople()

    private val graph = AppGraph(
        context,
        features = listOf(DetailStartFeature, TextSearchFeature, ItemFeature { FakeDetails(film()) }, PersonFeature { people }),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
    )

    private fun waitFor(matcher: SemanticsMatcher) =
        compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun target(name: Int) = compose.onNode(hasContentDescription(context.getString(name), substring = true) and hasClickAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.Selected))

    private fun openFilm() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("start page"))
        assertTrue(graph.links.request("item/${ulid(1)}"))
        waitFor(hasText("A new day begins."))
    }

    @Test
    fun aDoubleTapOnACardOpensOnePage() {
        openFilm()
        val card = compose.reveal(compose.onNode(hasContentDescription("Ann Actor", substring = true) and hasClickAction()))
        // Two taps before the next frame, as a fast finger does.
        val click = checkNotNull(card.fetchSemanticsNode().config[SemanticsActions.OnClick].action)
        compose.runOnUiThread {
            click()
            click()
        }
        waitFor(hasText("Ann Actor") and isHeading())
        assertEquals("one page, one request", 1, people.asked.size)
        back()
        waitFor(hasText("A new day begins."))
    }

    @Test
    fun theSearchOfATitleOpensTheTargetSearch() {
        openFilm()
        target(R.string.nav_home).assertIsSelected()
        // The search over the picture, not the target of the bar (same label).
        compose.onNode(hasContentDescription(context.getString(R.string.app_item_search)) and hasClickAction() and SemanticsMatcher.keyNotDefined(SemanticsProperties.Selected)).performClick()
        waitFor(hasText("search page"))
        target(R.string.nav_search).assertIsSelected()
        target(R.string.nav_home).assertIsNotSelected()
        back()
        waitFor(hasText("start page"))
        target(R.string.nav_home).assertIsSelected()
        target(R.string.nav_search).assertIsNotSelected()
    }
}
