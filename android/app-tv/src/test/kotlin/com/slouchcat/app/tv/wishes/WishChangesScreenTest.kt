package com.slouchcat.app.tv.wishes

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.performSemanticsAction
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DiscoverEntry
import com.slouchcat.core.api.generated.models.DiscoverRow
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitleSeason
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeSearch
import com.slouchcat.core.browse.testing.FakeWishes
import com.slouchcat.core.browse.testing.externalRef
import com.slouchcat.core.browse.testing.externalTitle
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.ulid
import com.slouchcat.core.browse.testing.wish
import com.slouchcat.core.strings.R
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The corrections of the code review of K26 stage 5 with the remote: «Zurückziehen» only where the
 * server allows it and the words of a joined wish (M-1); the focus when a withdrawn wish leaves the
 * list (M-2); a title that came into the library meanwhile (m-3); «Noch nicht in der Bibliothek?»
 * of the main search (m-5); a refused request, seasons that fail and Seerr's discover gone (m-8).
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class WishChangesScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val wishes = FakeWishes(
        rows = mutableListOf(
            DiscoverRow(DiscoverRowKey.POPULAR_MOVIES, listOf(DiscoverEntry(externalRef(79, "Big Brother"), false)), emptyList(), stale = false),
            DiscoverRow(DiscoverRowKey.POPULAR_SHOWS, listOf(DiscoverEntry(externalRef(106, "The Making", ExternalMedia.SHOW), false)), emptyList(), stale = false),
        ),
        titles = mutableMapOf(
            79L to externalTitle(79, "Big Brother"),
            106L to externalTitle(106, "The Making", ExternalMedia.SHOW).copy(seasons = listOf(1, 2, 3).map { ExternalTitleSeason(it, 8, ExternalTitleSeason.State.MISSING) }),
        ),
    )
    private val details = FakeDetails(film(5, title = "In The Library"))

    private fun open(search: FakeSearch = FakeSearch()) {
        pages.show({ it.copy(home = FakeHome(), wishes = wishes, detail = details, items = details, search = search) })
        pages.openRail(pages.s(R.string.nav_wishes))
        pages.waitFor("Big Brother")
    }

    /** «Meine Wünsche» from the tab «Entdecken». */
    private fun toMine() {
        remote.right(2)
        remote.assertFocused(pages.s(R.string.wishes_area_mine))
        remote.ok()
    }

    /** The title [title] in «Meine Wünsche»: its page. */
    private fun openWish(title: String) {
        toMine()
        pages.waitFor(title)
        repeat(MAX) { if (!remote.focusedLabel().contains(title)) remote.down() }
        remote.assertFocused(title)
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
    }

    @Test
    fun theTitlePageWithdrawsOnlyWhatTheServerAllowsAndSaysWhenTheProfileOnlyJoined() {
        // Seerr fetches it already: no «Zurückziehen» (the server says canWithdraw = false).
        wishes.wishList += wish(5, "Busy Five", WishState.PROCESSING)
        wishes.titles[1005] = externalTitle(1005, "Busy Five")
        wishes.wishList += wish(6, "Joined Six", joined = true)
        wishes.titles[1006] = externalTitle(1006, "Joined Six")
        open()
        openWish("Busy Five")
        pages.waitFor(pages.s(R.string.wishes_state_processing))
        remote.assertFocused(pages.s(R.string.app_wishes_toMine))
        assertTrue(compose.onAllNodes(hasText(pages.s(R.string.wishes_withdraw_action))).fetchSemanticsNodes().isEmpty())
        remote.right()
        remote.assertFocused(pages.s(R.string.app_wishes_toMine))
        remote.back()
        // A wish of another profile the profile joined: «Nicht mehr mitwünschen», the joined question.
        repeat(MAX) { if (!remote.focusedLabel().contains("Joined Six")) remote.down() }
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_withdraw_actionJoined))
        remote.right()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_actionJoined))
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_withdraw_titleJoined))
        remote.assertFocused(pages.s(R.string.app_wishes_keep))
        remote.left()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_confirmJoined))
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.contains("withdraw:6") }
        // Left: the page offers «Anfordern» again, with the focus on it.
        pages.waitFor(pages.s(R.string.app_wishes_request))
        remote.assertFocused(pages.s(R.string.app_wishes_request))
    }

    @Test
    fun aWithdrawnWishThatLeavesTheListHandsTheFocusToItsNeighbour() {
        wishes.wishList += listOf(wish(1, "Open One"), wish(2, "Open Two"), wish(3, "Joined Three", joined = true), wish(4, "Open Four"))
        wishes.titles[1004] = externalTitle(1004, "Open Four")
        open()
        toMine()
        pages.waitFor("Open One")
        // Filter «Offen»: a withdrawn wish is not open any more and goes.
        remote.down()
        val openChip = pages.s(R.string.wishes_page_filters_open)
        repeat(3) { if (!remote.focusedLabel().contains(openChip)) remote.left() }
        remote.assertFocused(openChip)
        remote.ok()
        remote.down()
        remote.assertFocused("Open One")
        remote.right()
        remote.ok()
        remote.left()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_confirm))
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.contains("withdraw:1") }
        pages.waitFor(pages.s(R.string.titles_done_withdrawn))
        remote.assertFocused("Open Two")
        // A joined wish leaves every list of the profile: its neighbour gets the focus.
        remote.down()
        remote.assertFocused("Joined Three")
        remote.right()
        remote.assertFocused(pages.s(R.string.wishes_withdraw_actionJoined))
        remote.ok()
        remote.left()
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.contains("withdraw:3") }
        compose.waitUntil(5_000) { compose.onAllNodes(hasText("Joined Three"), useUnmergedTree = true).fetchSemanticsNodes().isEmpty() }
        remote.settle()
        remote.assertFocused("Open Four")
        // Withdrawn on its page: back in the list (onResume) the focus goes to the neighbour, too.
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_withdraw_action))
        remote.right()
        remote.ok()
        remote.left()
        remote.ok()
        compose.waitUntil(5_000) { wishes.calls.contains("withdraw:4") }
        pages.waitFor(pages.s(R.string.app_wishes_request))
        remote.back()
        compose.waitUntil(5_000) { compose.onAllNodes(hasText("Open Four"), useUnmergedTree = true).fetchSemanticsNodes().isEmpty() }
        remote.settle()
        remote.assertFocused("Open Two")
    }

    @Test
    fun aTitleInTheLibraryMeanwhileOpensItsDetailPage() {
        wishes.titleFailures[79] = ApiResult.Failed(409, "already_available").also {
            it.problem = JsonObject(mapOf("item" to JsonObject(mapOf("id" to JsonPrimitive(ulid(5))))))
        }
        open()
        remote.down()
        remote.assertFocused("Big Brother")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        // The page of the title outside the library is gone: Back returns to the tile.
        remote.back()
        remote.assertFocused("Big Brother")
    }

    @Test
    fun aRefusedRequestStaysInTheMenuAndFailingSeasonsOfferRetry() {
        open()
        remote.down()
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
        wishes.changeFailure = ApiResult.Failed(429, "quota_exceeded")
        remote.ok()
        pages.waitFor(pages.s(R.string.app_wishes_requestNote))
        remote.ok()
        pages.waitFor(pages.s(R.string.errors_code_quota_exceeded))
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        remote.back()
        remote.assertFocused(pages.s(R.string.app_wishes_request))
        // A series whose seasons do not come: the note and «Erneut versuchen» in the menu.
        wishes.changeFailure = null
        remote.back()
        remote.down()
        remote.assertFocused("The Making")
        remote.ok()
        pages.waitFor(pages.s(R.string.titles_missing).uppercase())
        wishes.failure = ApiResult.Offline
        remote.ok()
        pages.waitFor(pages.s(R.string.app_state_retry))
        remote.up()
        remote.assertFocused(pages.s(R.string.app_state_retry))
        wishes.failure = null
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_dialog_season, "3"))
    }

    @Test
    fun discoverGoneForTheProfileLeadsToMyWishes() {
        wishes.discoverFailure = ApiResult.NotFound
        pages.show({ it.copy(home = FakeHome(), wishes = wishes, detail = details, items = details) })
        pages.openRail(pages.s(R.string.nav_wishes))
        pages.waitFor(pages.s(R.string.wishes_discover_gone_title))
        remote.down()
        remote.assertFocused(pages.s(R.string.wishes_discover_gone_action))
        remote.ok()
        pages.waitFor(pages.s(R.string.wishes_page_empty_title))
    }

    @Test
    fun theMainSearchOffersTitlesNotInTheLibraryAndTheWishesSearch() {
        open()
        pages.openRail(pages.s(R.string.nav_search))
        pages.waitFor(pages.s(R.string.tv_search_space))
        for (ch in "found") {
            compose.onNode(hasContentDescription(ch.toString()) and hasClickAction()).performSemanticsAction(SemanticsActions.OnClick)
            remote.settle()
        }
        pages.waitFor(pages.s(R.string.app_wishes_missingTitle))
        pages.waitFor("Found Seven")
        remote.right(12)
        repeat(MAX) { if (!remote.focusedLabel().contains("Found Seven")) remote.left() }
        remote.assertFocused("Found Seven")
        val card = pages.s(R.string.app_wishes_searchCard, "found")
        repeat(MAX) { if (!remote.focusedLabel().contains(card)) remote.right() }
        remote.assertFocused(card)
        // The card opens the wishes' search with the query.
        remote.ok()
        pages.waitFor("Found Eight")
        assertTrue(wishes.calls.contains("search:found:1"))
        remote.back()
        remote.assertFocused(card)
    }

    private companion object {
        const val MAX = 8
    }
}
