package com.slouchcat.app.mobile.search

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.CollectionsRoute
import com.slouchcat.app.mobile.browse.ProfilesSettingsRoute
import com.slouchcat.app.mobile.browse.WatchlistRoute
import com.slouchcat.app.mobile.collections.CollectionsFeature
import com.slouchcat.app.mobile.mylists.MyListsFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.profilesettings.ProfileSettingsFeature
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeCollections
import com.slouchcat.app.mobile.testing.FakeLibraries
import com.slouchcat.app.mobile.testing.FakeMyLists
import com.slouchcat.app.mobile.testing.FakeProfileSettings
import com.slouchcat.app.mobile.testing.FakeSearch
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.collectionRef
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.library
import com.slouchcat.app.mobile.testing.profile
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.serialization.Serializable
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Serializable internal data object PagesStartRoute

/** A start page that is only a text: the place the pages of this part are opened from. */
internal object PagesStartFeature : Feature {
    override val destinations = listOf(TopDestination("start", 10, PagesStartRoute, R.string.nav_home, SlouchcatIcon.Home))

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<PagesStartRoute> { Text("start page") }
    }
}

/**
 * «Zurück führt hinaus» (rule 8) through the real NavHost for the search, the lists, the
 * collections and «Profile verwalten»: each opens under «Start»; Back walks back to «Start», out.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class BrowsePagesBackTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()

    private val collections = FakeCollections(
        list = ApiResult.Ok(listOf(collectionRef("c1", "28 Days Later", 2))),
        parts = mutableMapOf("c1" to mutableListOf(itemRef(1), itemRef(2))),
    )

    private val graph = AppGraph(
        context,
        features = listOf(
            PagesStartFeature,
            SearchFeature { FakeSearch() },
            MyListsFeature { FakeMyLists(mutableListOf(itemRef(3, "Silo"))) },
            CollectionsFeature { collections },
            ProfileSettingsFeature { FakeProfileSettings(profile(1, "Primary", primary = true)) },
        ),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        librariesRepository = FakeLibraries(ApiResult.Ok(listOf(library(1, "Filme")))),
    )

    private fun s(id: Int) = context.getString(id)

    private fun waitFor(matcher: SemanticsMatcher) = compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }

    private fun back() = compose.activityRule.scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }

    private fun finishing(): Boolean {
        var result = false
        compose.activityRule.scenario.onActivity { result = it.isFinishing }
        return result
    }

    private fun heading(text: String) = hasText(text) and isHeading()

    private fun backLeadsOut() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("start page"))
        // The target «Suche» of the bar or the side column.
        val search = (hasText(s(R.string.nav_search)) or hasContentDescription(s(R.string.nav_search), substring = true)) and hasClickAction()
        compose.onAllNodes(search).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(hasContentDescription(s(R.string.search_label)))
        back()
        waitFor(hasText("start page"))
        graph.links.request(AppLink("start", listOf(CollectionsRoute, CollectionRoute("c1"))))
        waitFor(heading("28 Days Later"))
        back()
        waitFor(heading(s(R.string.collection_all_title)))
        back()
        waitFor(hasText("start page"))
        graph.links.request(AppLink("start", listOf(WatchlistRoute)))
        waitFor(heading(s(R.string.nav_watchlist)))
        compose.onNode(hasContentDescription(s(R.string.common_back)) and hasClickAction()).performClick()
        waitFor(hasText("start page"))
        graph.links.request(AppLink("start", listOf(ProfilesSettingsRoute)))
        waitFor(heading(s(R.string.app_profiles_manage_title)))
        back()
        waitFor(hasText("start page"))
        back()
        assertTrue(finishing())
    }

    /** The same media entry of the side column tapped twice: one page, one Back to «Start» (code review M-1). */
    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun aMediaEntryTappedTwiceOpensOnePage() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("start page"))
        val entry = (hasText(s(R.string.nav_watchlist)) or hasContentDescription(s(R.string.nav_watchlist), substring = true)) and hasClickAction()
        waitFor(entry)
        compose.onAllNodes(entry).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(heading(s(R.string.nav_watchlist)))
        compose.onAllNodes(entry).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        compose.waitForIdle()
        back()
        waitFor(hasText("start page"))
        back()
        assertTrue(finishing())
    }

    @Test @Config(qualifiers = "w412dp-h915dp")
    fun backLeadsOutPhone() = backLeadsOut()

    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun backLeadsOutTablet() = backLeadsOut()
}
