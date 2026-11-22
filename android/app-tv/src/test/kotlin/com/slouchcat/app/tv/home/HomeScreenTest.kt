package com.slouchcat.app.tv.home

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import com.slouchcat.app.tv.testing.FakeDetails
import com.slouchcat.app.tv.testing.FakeHome
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.TestPlayerFeature
import com.slouchcat.app.tv.testing.film
import com.slouchcat.app.tv.testing.homeRow
import com.slouchcat.app.tv.testing.itemRef
import com.slouchcat.app.tv.testing.posters
import com.slouchcat.app.tv.testing.progress
import com.slouchcat.app.tv.testing.ulid
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * «Start» with the remote (concept 26, 9; mockup `/`): the start focus on the stage, every control
 * reached with the D-pad, the focus visible, a card's detail page and back to the same card, Back
 * into the rail and out, the states loading, failed and empty, labels for TalkBack.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class HomeScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val spot = listOf(itemRef(1, title = "Night Train", images = posters(ulid(1)), runtimeMinutes = 145, rating = 12), itemRef(2, title = "Quiet Ring", kind = ItemKind.SERIES, images = posters(ulid(2))))
    private val recent = (10..14).map { itemRef(it, title = "Recent $it", images = posters(ulid(it))) }
    private val home = FakeHome(
        ApiResult.Ok(
            HomePage(
                rows = listOf(
                    homeRow(HomeRowKey.SPOTLIGHT, spot),
                    homeRow(HomeRowKey.CONTINUE, listOf(itemRef(20, title = "Half Seen", progress = progress(40), images = posters(ulid(20))))),
                    homeRow(HomeRowKey.RECENT, recent),
                ),
                heroRow = HomeRowKey.SPOTLIGHT,
            ),
        ),
    )
    private val details = FakeDetails(film(1, title = "Night Train"), film(11, title = "Recent 11"), film(20, title = "Half Seen"))

    private fun show(player: TestPlayerFeature? = null) = pages.show(
        { it.copy(home = home, detail = details, items = details) },
        features = com.slouchcat.app.tv.shell.features + listOfNotNull(player),
    )

    @Test
    fun theStageTakesTheFocusAndEveryControlIsReached() {
        show()
        pages.waitFor("Night Train")
        // No player in stage 2b: «Details» is the main button, «Abspielen» is not there.
        remote.assertFocused(pages.s(R.string.item_details))
        assertTrue(compose.onAllNodes(hasContentDescription(pages.s(R.string.item_play))).fetchSemanticsNodes().isEmpty())
        remote.assertEveryControlReachable()
        remote.assertFocusVisible(compose.onNode(hasText(pages.s(R.string.item_details)) and hasClickAction()), compose.onNode(hasText(pages.s(R.string.item_watchlist_add)) and hasClickAction()))
        remote.down(3)
        remote.assertFocused("Recent 10")
        remote.assertFocusVisible(compose.onNode(hasContentDescription("Recent 10", substring = true)), compose.onNode(hasContentDescription("Recent 11", substring = true)))
    }

    @Test
    fun aCardOpensItsTitleAndBackReturnsToTheSameCard() {
        show()
        pages.waitFor("Night Train")
        remote.down(3)
        remote.right()
        remote.assertFocused("Recent 11")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        pages.waitFor("Recent 11")
        remote.back()
        remote.assertFocused("Recent 11")
        // Back on the first page: into the rail, then the hint, then out.
        remote.back()
        remote.assertFocused(pages.s(R.string.nav_home))
    }

    @Test
    fun watchlistOnTheStageAsksTheServerAndTheRowEndOpensTheList() {
        show()
        pages.waitFor("Night Train")
        remote.left()
        remote.assertFocused(pages.s(R.string.item_watchlist_add))
        remote.ok()
        assertEquals(listOf("watchlist:${ulid(1)}:true"), details.changes)
        remote.assertFocused(pages.s(R.string.item_watchlist_added))
        // «Alle anzeigen» at the end of «Neu hinzugefügt».
        remote.down(3)
        remote.right(recent.size)
        remote.assertFocused(pages.s(R.string.app_browse_showAllLabel, pages.s(R.string.home_rows_recent)))
    }

    @Test
    fun withThePlayerAbspielenLeadsTheStageAndContinueResumes() {
        val player = TestPlayerFeature()
        show(player)
        pages.waitFor("Night Train")
        remote.assertFocused(pages.s(R.string.item_play))
        remote.ok()
        assertEquals(ulid(1), player.played.single().itemId)
    }

    @Test
    fun aFailedStartPageOffersRetryAndAnEmptyOneSaysSo() {
        home.page = ApiResult.Offline
        show()
        pages.waitFor(pages.s(R.string.app_state_retry))
        remote.assertFocused(pages.s(R.string.app_state_retry))
        home.page = ApiResult.Ok(HomePage(emptyList()))
        remote.ok()
        pages.waitFor(pages.s(R.string.home_empty_title))
        remote.assertFocused(pages.s(R.string.home_empty_action))
    }
}
