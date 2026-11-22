package com.slouchcat.app.tv.wishes

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.semantics.SemanticsActions
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DiscoverEntry
import com.slouchcat.core.api.generated.models.DiscoverRow
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitleSeason
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeWishes
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.externalTitle
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.rights
import com.slouchcat.core.browse.testing.wish
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
 * «Wünsche» with the remote (mockups `/wishes`, `/wish/:i`, reference photos tv 32, 33, 34, 60,
 * 61): «Entdecken» with Seerr's rows, a tile to its page and back, «Anfordern» of a film and of a
 * series with its seasons in a side menu, «Zurückziehen» with its question, «Meine Wünsche» with the
 * limit and the focus after withdrawing, «Suche» with the own keyboard, wishes off. The focus is
 * never lost: after loading, a menu, a question, a change and the way back from a page.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class WishesScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val wishes = FakeWishes(
        rows = mutableListOf(
            DiscoverRow(DiscoverRowKey.POPULAR_MOVIES, listOf(DiscoverEntry(externalRef(79, "Big Brother"), false), DiscoverEntry(externalRef(80, "In The Library"), false, item = itemRef(5, "In The Library"))), emptyList(), stale = false),
            DiscoverRow(DiscoverRowKey.POPULAR_SHOWS, listOf(DiscoverEntry(externalRef(106, "The Making", ExternalMedia.SHOW), true)), emptyList(), stale = false),
        ),
        titles = mutableMapOf(
            79L to externalTitle(79, "Big Brother"),
            106L to externalTitle(106, "The Making", ExternalMedia.SHOW).copy(seasons = listOf(season(1), season(2), season(3))),
        ),
    )
    private val details = FakeDetails(film(5, title = "In The Library"))

    private fun season(n: Int) = ExternalTitleSeason(n, 8, ExternalTitleSeason.State.MISSING)

    /** «2 Staffeln anfordern». */
    private fun submitSeasons(count: Int) = pages.context.resources.getQuantityString(R.plurals.wishes_dialog_submitSeasons, count, count)

    private fun open() {
        pages.show({ it.copy(home = FakeHome(), wishes = wishes, detail = details, items = details) })
        pages.openRail(pages.s(R.string.nav_wishes))
    }

    @Test
    fun discoverShowsSeerrsRowsAndATileLeadsToItsPageAndBack() {
        open()
        pages.waitFor("Big Brother")
        remote.assertFocused(pages.s(R.string.wishes_area_discover))
        remote.down()
        remote.assertFocused("Big Brother")
        // A title the profile has in the library opens its detail page.
        remote.right()
        remote.assertFocused("In The Library")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("In The Library")
        // The row of series below; a title outside the library opens its own page.
        remote.down()
        remote.assertFocused("The Making")
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        remote.back()
        remote.assertFocused("The Making")
        remote.assertEveryControlReachable(pages.page)
        // The focus is visible on a tab and on a tile.
        val tab = { label: Int -> compose.onNode(hasContentDescription(pages.s(label)) and hasClickAction() and pages.page) }
        remote.assertFocusVisible(tab(R.string.wishes_area_search), tab(R.string.wishes_area_discover))
        remote.assertFocusVisible(compose.onNode(hasContentDescription("Big Brother", substring = true) and hasClickAction()), tab(R.string.wishes_area_discover))
    }

    @Test
    fun aFilmIsRequestedAndWithdrawnAfterTheQuestion() {
        open()
        pages.waitFor("Big Brother")
        remote.down()
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
        remote.assertEveryControlReachable(pages.page)
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        // «Anfordern» opens the side menu with the focus on its own «Anfordern»; Back closes it.
        remote.ok()
        pages.waitFor(pages.s(R.string.app_wishes_requestNote))
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        remote.right()
        remote.assertFocused(pages.s(R.string.common_cancel))
        remote.back()
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        assertTrue(wishes.created.isEmpty())
        remote.ok()
        pages.waitFor(pages.s(R.string.app_wishes_requestNote))
        remote.ok()
        compose.waitUntil(5_000) { wishes.created.isNotEmpty() }
        pages.waitFor(pages.s(R.string.app_wishes_toMine))
        assertEquals(WishCreate.Source.DETAIL, wishes.created.single().source)
        // The page shows the wish; the focus is on «Meine Wünsche», not lost with «Anfordern».
        remote.assertFocused(pages.s(R.string.app_wishes_toMine))
        pages.waitFor(pages.s(R.string.wishes_state_pending))
        remote.right()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_action))
        // «Zurückziehen» asks; «Behalten» first, Back keeps the wish.
        remote.ok()
        remote.assertFocused(pages.s(R.string.app_wishes_keep))
        remote.back()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_action))
        remote.ok()
        remote.left()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_confirm))
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.any { it.startsWith("withdraw:") } }
        pages.waitFor(pages.s(R.string.app_wishes_request))
        remote.assertFocused(pages.s(R.string.app_wishes_request))
    }

    @Test
    fun aSeriesChoosesItsSeasonsInTheSideMenu() {
        open()
        pages.waitFor("The Making")
        remote.down(2)
        remote.assertFocused("The Making")
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
        remote.ok()
        // Season 1 is there, 2 is missing, 3 another profile asked for: 2 and 3 are ticked.
        pages.waitFor(pages.s(R.string.wishes_dialog_season, "3"))
        remote.assertFocused(submitSeasons(2))
        remote.up()
        remote.assertFocused(pages.s(R.string.wishes_dialog_season, "3"))
        remote.ok()
        remote.assertFocused(pages.s(R.string.wishes_dialog_season, "3"))
        // Only season 2 left: one season.
        remote.down()
        remote.assertFocused(submitSeasons(1))
        // Inside the menu every control is reached and none leads out of it.
        remote.assertEveryControlReachable(hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle)))
        remote.ok()
        compose.waitUntil(5_000) { wishes.created.isNotEmpty() }
        assertEquals(listOf(2), wishes.created.single().seasons)
        pages.waitFor(pages.s(R.string.app_wishes_toMine))
        remote.assertFocused(pages.s(R.string.app_wishes_toMine))
    }

    @Test
    fun myWishesShowTheLimitAndTheFocusStaysAfterWithdrawing() {
        wishes.wishList += listOf(wish(1, "Open One"), wish(2, "There Two", WishState.AVAILABLE, item = itemRef(5, "In The Library")), wish(3, "Third Three", WishState.PROCESSING))
        open()
        pages.waitFor("Big Brother")
        remote.right(2)
        remote.assertFocused(pages.s(R.string.wishes_area_mine))
        remote.ok()
        pages.waitFor("Open One")
        // The limit above the list: films and series.
        pages.waitFor(pages.s(R.string.wishes_quota_movie, "1", "5", "7"))
        remote.assertFocused(pages.s(R.string.wishes_area_mine))
        remote.assertEveryControlReachable(pages.page)
        remote.down(2)
        remote.assertFocused("Open One")
        remote.right()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_action))
        remote.ok()
        remote.assertFocused(pages.s(R.string.app_wishes_keep))
        remote.left()
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.contains("withdraw:1") }
        pages.waitFor(pages.s(R.string.wishes_state_withdrawn))
        // The button of the row went; the focus is on the wish itself.
        remote.assertFocused("Open One")
        // «Verfügbar» leads to the title in the library; Back to the same wish.
        remote.down()
        remote.assertFocused("There Two")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("There Two")
    }

    @Test
    fun theSearchTypesWithTheOwnKeyboardAndFindsFilmsAndSeries() {
        open()
        pages.waitFor("Big Brother")
        remote.right()
        remote.assertFocused(pages.s(R.string.wishes_area_search))
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_search_start))
        remote.assertFocused("q")
        for (ch in "found") {
            compose.onNode(hasContentDescription(ch.toString()) and hasClickAction()).performSemanticsAction(SemanticsActions.OnClick)
            remote.settle()
        }
        // Seerr is asked a pause after the last key, once.
        pages.waitFor("Found Seven")
        pages.waitFor("Found Eight")
        assertEquals(listOf("search:found:1"), wishes.calls.filter { it.startsWith("search:") })
        remote.right(12)
        remote.assertFocused("Found Seven")
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_notFound_title))
        // A title gone meanwhile: «Zurück» has the focus and leads back to the hit.
        remote.assertFocused(pages.s(R.string.common_back))
        remote.ok()
        remote.assertFocused("Found Seven")
    }

    @Test
    fun wishesOffSayItAndLeadToStart() {
        wishes.rightsAnswer = ApiResult.Ok(rights(enabled = false))
        open()
        pages.waitFor(pages.s(R.string.app_wishes_off_title))
        remote.assertFocused(pages.s(R.string.app_lists_toStart))
        assertTrue(wishes.calls.none { it == "discover" })
    }

    @Test
    fun seerrUnreachableOffersRetryAndTheEmptyListLeadsToDiscover() {
        wishes.discoverFailure = ApiResult.Offline
        open()
        pages.waitFor(pages.s(R.string.app_state_retry))
        remote.down()
        remote.assertFocused(pages.s(R.string.app_state_retry))
        wishes.discoverFailure = null
        remote.ok()
        pages.waitFor("Big Brother")
        // The retry went with the error: the focus is on the open tab again.
        remote.assertFocused(pages.s(R.string.wishes_area_discover))
        // No wishes yet: «Meine Wünsche» says so and offers «Entdecken».
        remote.right(2)
        remote.assertFocused(pages.s(R.string.wishes_area_mine))
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_page_empty_title))
        remote.down(2)
        remote.assertFocused(pages.s(R.string.wishes_area_discover))
    }
}
