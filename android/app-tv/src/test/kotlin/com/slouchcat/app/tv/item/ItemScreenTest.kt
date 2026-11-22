package com.slouchcat.app.tv.item

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.TestPlayerFeature
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakePeople
import com.slouchcat.core.browse.testing.SEASON_1
import com.slouchcat.core.browse.testing.SERIES_ID
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.homeRow
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.posters
import com.slouchcat.core.browse.testing.progress
import com.slouchcat.core.browse.testing.seasonDetail
import com.slouchcat.core.browse.testing.series
import com.slouchcat.core.browse.testing.ulid
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
 * The detail pages with the remote (mockup `/item/:i`): the actions of a film and their labels,
 * «Gesehen» and «Merken» to the server, «Mehr Infos» and the question before recommending, the
 * person of the cast and back to the same portrait, a series with seasons and episodes, «Abspielen»
 * only with the player, a title not found.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ItemScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote
    private val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))

    private val details = FakeDetails(film(1, title = "Night Train", progress = progress(40, minutes = 145)), series(), seasonDetail(1, watched = 2), seasonDetail(2))

    /** The start page shows one row with [titles]; the first card is opened. */
    private fun open(vararg titles: Pair<Int, String>, player: TestPlayerFeature? = null, kind: ItemKind = ItemKind.MOVIE) {
        val row = titles.map { (n, title) -> itemRef(n, title = title, kind = kind, images = posters(ulid(n))).let { if (n == 200) it.copy(id = SERIES_ID) else it } }
        val home = FakeHome(ApiResult.Ok(HomePage(listOf(homeRow(HomeRowKey.RECENT, row)))))
        pages.show({ it.copy(home = home, detail = details, items = details, person = FakePeople()) }, features = com.slouchcat.app.tv.shell.features + listOfNotNull(player))
        pages.waitFor(titles.first().second)
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
    }

    @Test
    fun aFilmShowsItsActionsAndEveryControlIsReached() {
        open(1 to "Night Train")
        // Stage 2b has no player: no «Abspielen», the first round button has the focus.
        assertTrue(compose.onAllNodes(hasContentDescription(pages.s(R.string.app_item_resume))).fetchSemanticsNodes().isEmpty())
        remote.assertFocused(pages.s(R.string.item_watchlist_add))
        remote.assertEveryControlReachable(pages.page)
        remote.assertFocusVisible(compose.onNode(hasContentDescription(pages.s(R.string.item_watchlist_add))), compose.onNode(hasContentDescription(pages.s(R.string.watched_hint_set))))
        remote.ok()
        remote.right()
        remote.ok()
        assertEquals(listOf("watchlist:${ulid(1)}:true", "watched:${ulid(1)}:true"), details.changes)
        // The page reads the title again (the fake server keeps it unwatched); the focus stays on the button.
        remote.assertFocused(pages.s(R.string.watched_hint_set))
    }

    @Test
    fun moreInfoAndTheQuestionBeforeRecommendingCloseWithBack() {
        open(1 to "Night Train")
        remote.right(2)
        remote.assertFocused(pages.s(R.string.item_recommend_add))
        remote.ok()
        remote.assertFocused(pages.s(R.string.common_cancel))
        remote.back()
        remote.assertFocused(pages.s(R.string.item_recommend_add))
        remote.right()
        remote.assertFocused(pages.s(R.string.item_trailer))
        remote.right()
        remote.assertFocused(pages.s(R.string.tv_item_more))
        remote.ok()
        remote.assertFocused(pages.s(R.string.common_close))
        remote.assertEveryControlReachable(menu)
        remote.back()
        remote.assertFocused(pages.s(R.string.tv_item_more))
        assertTrue(details.changes.isEmpty())
    }

    @Test
    fun aPortraitOpensThePersonAndBackReturnsToIt() {
        open(1 to "Night Train")
        remote.down()
        remote.right()
        val name = remote.focusedLabel()
        remote.ok()
        pages.waitFor(pages.s(R.string.app_person_knownFrom))
        remote.back()
        assertEquals(name, remote.focusedLabel())
    }

    @Test
    fun withThePlayerAbspielenResumesAndVonVornStartsAgain() {
        val player = TestPlayerFeature()
        open(1 to "Night Train", player = player)
        remote.assertFocused(pages.s(R.string.app_item_resume))
        remote.ok()
        remote.back()
        remote.right()
        remote.assertFocused(pages.s(R.string.app_item_fromStart))
        remote.ok()
        assertEquals(listOf(ulid(1) to false, ulid(1) to true), player.played.map { it.itemId to it.fromStart })
    }

    @Test
    fun aSeriesListsTheEpisodesOfTheChosenSeason() {
        open(200 to "Quiet Ring", kind = ItemKind.SERIES)
        remote.down()
        remote.assertFocused((seasonDetail(1) as ItemDetail).title)
        remote.right()
        // The focus chooses the season: its episodes come.
        compose.waitUntil(5_000) { details.asked.contains(seasonDetail(2).id) }
        remote.left()
        remote.down()
        remote.assertFocused("1.")
        assertTrue(details.asked.contains(SEASON_1))
    }

    @Test
    fun aTitleTheProfileDoesNotSeeIsNotFound() {
        val home = FakeHome(ApiResult.Ok(HomePage(listOf(homeRow(HomeRowKey.RECENT, listOf(itemRef(99, title = "Gone", images = posters(ulid(99)))))))))
        pages.show({ it.copy(home = home, detail = details, items = details) })
        pages.waitFor("Gone")
        remote.ok()
        pages.waitFor(pages.s(R.string.app_state_notFound))
        remote.back()
        remote.assertFocused("Gone")
    }
}
