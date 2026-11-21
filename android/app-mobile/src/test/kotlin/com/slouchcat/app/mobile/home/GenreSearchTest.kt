package com.slouchcat.app.mobile.home

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.material3.Text
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.LibraryRoute
import com.slouchcat.app.mobile.browse.SEARCH_TARGET
import com.slouchcat.app.mobile.browse.SearchRoute
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.app.mobile.testing.EMAIL
import com.slouchcat.app.mobile.testing.FakeAccounts
import com.slouchcat.app.mobile.testing.FakeHome
import com.slouchcat.app.mobile.testing.FakeLibraries
import com.slouchcat.app.mobile.testing.FakeLibrary
import com.slouchcat.app.mobile.testing.FakeServers
import com.slouchcat.app.mobile.testing.FakeTokens
import com.slouchcat.app.mobile.testing.HTTPS_SERVER
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.homeRow
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.library
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** A search target that writes its genre. */
internal object GenreSearchFeature : Feature {
    override val destinations = listOf(TopDestination(SEARCH_TARGET, 20, SearchRoute(), R.string.nav_search, SlouchcatIcon.Search))

    override fun NavGraphBuilder.register(app: AppScope) {
        composable<SearchRoute> { Text("search page ${it.toRoute<SearchRoute>().genre ?: "empty"}") }
    }
}

/**
 * A genre of the start page opens the target «Suche» with its genre, marked in the bar; «Start»
 * keeps its own pages, and the search target does not mix them (code review M-2 of part a).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class GenreSearchTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()

    private val page = HomePage(
        rows = listOf(
            homeRow(HomeRowKey.RECENT, listOf(itemRef(1, title = "Fresh"))),
            homeRow(HomeRowKey.GENRES, emptyList(), genres = listOf(GenreRef("Drama", 3))),
        ),
    )

    private val graph = AppGraph(
        context,
        features = listOf(HomeFeature, GenreSearchFeature),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        mediaData = { graph -> MediaData.api(graph.api, graph.http).copy(home = FakeHome(ApiResult.Ok(page))) },
    )

    private fun waitFor(matcher: SemanticsMatcher) = compose.waitUntil(5_000) { compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty() }

    private fun target(name: Int) =
        compose.onNode(hasContentDescription(context.getString(name), substring = true) and hasClickAction() and SemanticsMatcher.keyIsDefined(SemanticsProperties.Selected))

    @Test
    fun aGenreOpensTheTargetSearch() {
        compose.setContent { SlouchcatNavHost(graph) }
        waitFor(hasText("Drama"))
        compose.onAllNodes(hasText("Drama") and hasClickAction()).onFirst().performScrollTo().performClick()
        waitFor(hasText("search page Drama"))
        target(R.string.nav_search).assertIsSelected()
        target(R.string.nav_home).assertIsNotSelected()
        // «Start» and back to «Suche»: the start page and the search, not mixed.
        target(R.string.nav_home).performClick()
        waitFor(hasText("Drama") and hasClickAction())
        target(R.string.nav_home).assertIsSelected()
        target(R.string.nav_search).performClick()
        waitFor(hasText("search page", substring = true))
        target(R.string.nav_search).assertIsSelected()
    }

    /**
     * «Neu in Filme › Alle» opens the library with its order; the marked entry of the side column
     * tapped then opens nothing new, one Back returns to «Start» (code review m-7 of part a).
     */
    @Test @Config(qualifiers = "w1280dp-h800dp-land")
    fun theMarkedLibraryEntryOpensNoSecondPage() {
        val withLibrary = AppGraph(
            context,
            features = listOf(HomeFeature, GenreSearchFeature),
            serverStore = FakeServers(HTTPS_SERVER),
            tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
            accountStore = FakeAccounts(EMAIL, PRIMARY),
            librariesRepository = FakeLibraries(ApiResult.Ok(listOf(library(4, "Test films")))),
            mediaData = { graph -> MediaData.api(graph.api, graph.http).copy(home = FakeHome(ApiResult.Ok(page)), library = FakeLibrary(ApiResult.Ok(listOf(itemRef(1, title = "Fresh"))))) },
        )
        compose.setContent { SlouchcatNavHost(withLibrary) }
        waitFor(hasText("Test films"))
        compose.runOnUiThread { withLibrary.links.request(AppLink("start", listOf(LibraryRoute(4, LibrarySort.ADDED.value)))) }
        waitFor(hasText("Test films") and isHeading())
        compose.onAllNodes(hasText("Test films") and hasClickAction()).onFirst().performClick()
        compose.waitForIdle()
        compose.runOnUiThread { compose.activity.onBackPressedDispatcher.onBackPressed() }
        waitFor(hasText("Drama"))
        assertTrue(compose.onAllNodes(hasText("Test films") and isHeading()).fetchSemanticsNodes().isEmpty())
    }
}
