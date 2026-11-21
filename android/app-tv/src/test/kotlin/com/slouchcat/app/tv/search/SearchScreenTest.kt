package com.slouchcat.app.tv.search

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.performSemanticsAction
import com.slouchcat.app.tv.testing.FakeDetails
import com.slouchcat.app.tv.testing.FakeHome
import com.slouchcat.app.tv.testing.FakePeople
import com.slouchcat.app.tv.testing.FakeSearch
import com.slouchcat.app.tv.testing.Pages
import com.slouchcat.app.tv.testing.TV
import com.slouchcat.app.tv.testing.film
import com.slouchcat.app.tv.testing.itemRef
import com.slouchcat.app.tv.testing.personRef
import com.slouchcat.app.tv.testing.posters
import com.slouchcat.app.tv.testing.ulid
import com.slouchcat.core.api.ApiResult
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
 * The search and the person with the remote (mockups `/search`, `/person/:id`): the keyboard takes
 * the focus, typed letters ask the server, hits as rows by kind with people, a hit opens its title
 * and back to it, nothing found, a failure; the person with «Bekannt aus».
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [34], qualifiers = TV)
class SearchScreenTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()
    private val pages = Pages(compose)
    private val remote = pages.remote

    private val search = FakeSearch(
        titles = listOf(
            itemRef(1, title = "Night Train", images = posters(ulid(1))),
            itemRef(2, title = "Night Shift", kind = ItemKind.SERIES, images = posters(ulid(2))),
        ),
        people = listOf(personRef(101, "Nick Night")),
    )
    private val details = FakeDetails(film(1, title = "Night Train"))

    private fun open() {
        pages.show({ it.copy(home = FakeHome(), search = search, detail = details, items = details, person = FakePeople()) })
        pages.openRail(pages.s(R.string.nav_search))
        pages.waitFor(pages.s(R.string.tv_search_space))
    }

    /** Types [text] with the keyboard of the page: each letter by its key. */
    private fun type(text: String) {
        for (ch in text) {
            compose.onNode(hasContentDescription(ch.toString()) and androidx.compose.ui.test.hasClickAction()).performSemanticsAction(SemanticsActions.OnClick)
            remote.settle()
        }
    }

    @Test
    fun theKeyboardTakesTheFocusAndEveryKeyIsReached() {
        open()
        remote.assertFocused("q")
        // The keys and the libraries and genres: more than one walk; the keyboard alone.
        remote.assertEveryControlReachable(pages.page, max = 60)
        remote.assertFocusVisible(compose.onNode(hasContentDescription("q")), compose.onNode(hasContentDescription("w")))
    }

    @Test
    fun typedLettersFindTitlesAndPeople() {
        open()
        type("n")
        // One letter is too short; the server is asked from two.
        pages.waitFor(pages.s(R.string.search_tooShort))
        assertTrue(search.asked.isEmpty())
        type("ight")
        pages.waitFor("Night Train")
        assertEquals("night", search.asked.last().first)
        pages.waitFor(pages.s(R.string.watched_page_contents_movie))
        pages.waitFor(pages.s(R.string.watched_page_contents_series))
    }

    @Test
    fun aHitOpensItsTitleAndBackReturnsToIt() {
        open()
        type("night")
        pages.waitFor("Night Train")
        // From the keyboard to the right: the hits.
        remote.right(12)
        remote.assertFocused("Night Train")
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("Night Train")
    }

    @Test
    fun nothingFoundSaysSoAndAFailureOffersRetry() {
        open()
        type("zz")
        pages.waitFor(pages.s(R.string.search_empty_title))
        search.failure = ApiResult.Offline
        type("z")
        pages.waitFor(pages.s(R.string.app_state_retry))
        search.failure = null
        remote.right(12)
        remote.assertFocused(pages.s(R.string.app_state_retry))
    }

    @Test
    fun aPersonShowsTheTitlesOfTheProfile() {
        open()
        type("nick")
        pages.waitFor("Nick Night")
        remote.right(12)
        remote.assertFocused("Nick Night")
        remote.ok()
        pages.waitFor(pages.s(R.string.app_person_knownFrom))
        remote.assertFocused("Night Train")
        remote.assertEveryControlReachable(pages.page)
        remote.ok()
        pages.waitFor(pages.s(R.string.tv_item_more))
        remote.back()
        remote.assertFocused("Night Train")
        remote.back()
        remote.assertFocused("Nick Night")
    }
}
