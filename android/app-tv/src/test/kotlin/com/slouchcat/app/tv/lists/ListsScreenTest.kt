package com.slouchcat.app.tv.lists

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.FakeCollections
import com.slouchcat.app.tv.testing.FakeDetails
import com.slouchcat.app.tv.testing.FakeHome
import com.slouchcat.app.tv.testing.FakeMyLists
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.collectionRef
import com.slouchcat.app.tv.testing.film
import com.slouchcat.app.tv.testing.homeRow
import com.slouchcat.app.tv.testing.itemRef
import com.slouchcat.app.tv.testing.posters
import com.slouchcat.app.tv.testing.ulid
import com.slouchcat.app.tv.testing.watchedEntry
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRowKey
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
 * The lists with the remote (mockups `/list/:key`, `/watchlist`, `/watched`, `/collections`,
 * `/collection/:id`): from the rail and from a row of the start page, «Bearbeiten» takes titles
 * off and moves the focus on, a series asks before its marks go, the empty lists, a collection
 * with «Alle merken».
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class ListsScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val recent = (1..3).map { itemRef(it, title = "Recent $it", images = posters(ulid(it))) }
    private val home = FakeHome(ApiResult.Ok(HomePage(listOf(homeRow(HomeRowKey.RECENT, recent))))).apply {
        lists[HomeListKey.RECENT] = ApiResult.Ok(recent + (4..9).map { itemRef(it, title = "Recent $it", images = posters(ulid(it))) })
    }
    private val lists = FakeMyLists(
        watchlist = (1..3).map { itemRef(it, title = "Kept $it", inWatchlist = true, images = posters(ulid(it))) }.toMutableList(),
        watched = mutableListOf(watchedEntry(5, "Seen Film"), watchedEntry(6, "Seen Series", episodes = 8, watchedCount = 8)),
    )
    private val parts = (11..13).map { itemRef(it, title = "Part $it", images = posters(ulid(it))) }
    private val collections = FakeCollections(ApiResult.Ok(listOf(collectionRef(ulid(900), "Night Trains", 3))), mutableMapOf(ulid(900) to parts.toMutableList()))
    private val details = FakeDetails(film(2, title = "Kept 2"))

    private fun show() = pages.show({ it.copy(home = home, myLists = lists, collections = collections, detail = details, items = details) })

    @Test
    fun allOfARowOpensTheWholeList() {
        show()
        pages.waitFor("Recent 1")
        remote.right(recent.size)
        remote.ok()
        pages.waitFor("Recent 9")
        remote.assertFocused("Recent 1")
        remote.assertEveryControlReachable(pages.page)
        remote.back()
        remote.assertFocused(pages.s(R.string.app_browse_showAllLabel, pages.s(R.string.home_rows_recent)))
    }

    @Test
    fun theWatchlistTakesTitlesOffInEditMode() {
        show()
        pages.openRail(pages.s(R.string.watchlist_title))
        pages.waitFor("Kept 1")
        remote.assertFocused("Kept 1")
        remote.right()
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("Kept 2")
        remote.up()
        remote.assertFocused(pages.s(R.string.app_lists_edit))
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_lists_removeHint))
        // Down: the grid again, on the card it had.
        remote.down()
        remote.assertFocused("Kept 2")
        remote.ok()
        compose.waitUntil(5_000) { lists.calls.contains("set:${ulid(2)}:false") }
        remote.settle()
        // The focus moves on to the next card.
        remote.assertFocused("Kept 3")
    }

    @Test
    fun watchedAsksBeforeTheMarksOfASeriesGo() {
        show()
        pages.openRail(pages.s(R.string.watched_page_title))
        pages.waitFor("Seen Film")
        remote.up()
        remote.ok()
        remote.down()
        remote.right()
        remote.assertFocused("Seen Series")
        remote.ok()
        remote.assertFocused(pages.s(R.string.common_cancel))
        remote.back()
        remote.assertFocused("Seen Series")
        assertTrue(lists.calls.none { it.startsWith("watched:${ulid(6)}") })
    }

    @Test
    fun anEmptyWatchlistLeadsBackToStart() {
        lists.watchlist.clear()
        show()
        pages.openRail(pages.s(R.string.watchlist_title))
        pages.waitFor(pages.s(R.string.watchlist_empty_title))
        remote.assertFocused(pages.s(R.string.app_lists_toStart))
    }

    @Test
    fun aCollectionListsItsPartsAndMarksThemAll() {
        show()
        pages.openRail(pages.s(R.string.collection_all_title))
        pages.waitFor("Night Trains")
        remote.assertFocused("Night Trains")
        remote.ok()
        pages.waitFor("Part 11")
        remote.assertFocused(pages.s(R.string.collection_watchlist_add))
        remote.assertEveryControlReachable(pages.page)
        remote.ok()
        compose.waitUntil(5_000) { collections.calls.count { it.startsWith("watchlist:") } == parts.size }
        remote.back()
        remote.assertFocused("Night Trains")
        assertEquals(3, collections.calls.count { it.startsWith("watchlist:") && it.endsWith(":true") })
    }
}
