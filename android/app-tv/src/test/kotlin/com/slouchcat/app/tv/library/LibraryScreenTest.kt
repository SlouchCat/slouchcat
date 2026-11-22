package com.slouchcat.app.tv.library

import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.FakeDetails
import com.slouchcat.core.browse.testing.FakeHome
import com.slouchcat.core.browse.testing.FakeLibrary
import com.slouchcat.core.browse.testing.film
import com.slouchcat.core.browse.testing.itemRef
import com.slouchcat.core.browse.testing.posters
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
 * A library with the remote (mockup `/library/:key`): from the rail, the grid takes the focus, the
 * chips open side menus whose choice the server gets, the menu gives the focus back to its chip,
 * a card's title and back to the same card, the empty filter, a failed load.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class LibraryScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val titles = (1..9).map { itemRef(it, title = "Film $it", images = posters(ulid(it))) }
    private val library = FakeLibrary(ApiResult.Ok(titles)).apply { genreOf = { if (it.title.endsWith("1")) "Drama" else "Comedy" } }
    private val details = FakeDetails(film(2, title = "Film 2"))

    private fun open(first: String = "Film 1") {
        pages.show({ it.copy(home = FakeHome(), library = library, detail = details, items = details) })
        pages.openRail("Filme")
        pages.waitFor(first)
    }

    @Test
    fun theGridTakesTheFocusAndEveryControlIsReached() {
        open()
        remote.assertFocused("Film 1")
        remote.assertEveryControlReachable(pages.page)
        remote.right()
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("Film 2")
        // Back from the library: to «Start», below every entry of the rail (2a).
        remote.back()
        remote.assertFocused(pages.s(R.string.home_empty_action))
    }

    @Test
    fun aFilterChipOpensItsMenuAndTheChoiceGoesToTheServer() {
        open()
        remote.up()
        remote.assertFocused(pages.s(R.string.app_browse_all))
        remote.right()
        remote.assertFocused(pages.s(R.string.library_filters_genre))
        remote.ok()
        val menu = hasAnyAncestor(SemanticsMatcher.keyIsDefined(SemanticsProperties.PaneTitle))
        // The current choice has the focus; every choice is reached.
        remote.assertFocused(pages.s(R.string.library_filters_allGenres))
        remote.assertEveryControlReachable(menu)
        remote.down()
        remote.assertFocused("Drama")
        remote.ok()
        pages.waitFor("Film 1")
        assertEquals("Drama", library.queries.last().first.genre)
        remote.assertFocused("Drama")
        // «Zurücksetzen» is there while a filter is on.
        assertTrue(compose.onAllNodes(hasText(pages.s(R.string.library_page_clear)) and hasClickAction()).fetchSemanticsNodes().isNotEmpty())
    }

    @Test
    fun aMenuClosesWithBackAndTheFocusStaysOnItsChip() {
        open()
        remote.up()
        remote.right(3)
        remote.assertFocused(pages.s(R.string.library_sort_added))
        remote.ok()
        remote.back()
        remote.assertFocused(pages.s(R.string.library_sort_added))
    }

    @Test
    fun anEmptyFilterOffersResetAndAFailedLoadRetry() {
        library.all = ApiResult.Offline
        open(pages.s(R.string.app_state_retry))
        remote.assertFocused(pages.s(R.string.app_state_retry))
        library.all = ApiResult.Ok(titles)
        remote.ok()
        pages.waitFor("Film 1")
        remote.assertFocused("Film 1")
    }
}
