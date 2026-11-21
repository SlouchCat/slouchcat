package com.slouchcat.app.mobile.search

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.collectionRef
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.library
import com.slouchcat.app.mobile.testing.personRef
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

private fun q(id: Int, n: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().resources.getQuantityString(id, n, *args)

/** The search (rule 8): two widths, 200 % font, the ways in, the matches, nothing found, a failure. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class SearchScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun show(state: SearchUiState) = compose.setContent {
        SlouchcatTheme {
            SearchScreen(
                state = state,
                images = SERVER_IMAGES,
                libraries = listOf(library(1, "Filme", count = 1846), library(2, "Serien", count = 505)),
                onText = { events += "text:$it" },
                onClear = { events += "clear" },
                onSubmit = { events += "submit" },
                onGenre = { events += "genre:$it" },
                onClearGenre = { events += "nogenre" },
                onRetry = { events += "retry" },
                onRetryGenres = { events += "retryGenres" },
                onMore = { events += "more" },
                onItem = { id -> { events += "item:$id" } },
                onPerson = { id -> { events += "person:$id" } },
                onCollection = { id -> { events += "collection:$id" } },
                onLibrary = { id -> { events += "library:$id" } },
            )
        }
    }

    private fun waysIn() {
        show(SearchUiState(genres = Loadable.Ready(listOf(GenreRef("Action", 12), GenreRef("Drama", 3)))))
        compose.onNode(hasText(s(R.string.app_search_browse)) and isHeading()).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_search_genres)) and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNode(hasSetTextAction()).performTextInput("sp")
        compose.reveal(compose.onNode(hasText("Serien") and hasClickAction(), useUnmergedTree = false)).performClick()
        compose.reveal(compose.onNode(hasText("Drama") and hasClickAction())).performClick()
        assertEquals(listOf("text:sp", "library:2", "genre:Drama"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun waysInPhone() = waysIn()

    @Test @Config(qualifiers = TABLET)
    fun waysInTablet() = waysIn()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun waysInPhoneLargeFont() = waysIn()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun waysInTabletLargeFont() = waysIn()

    private fun matches() {
        val hits = SearchHits(
            items = listOf(MediaItem.from(itemRef(1, "Spider-Man", runtimeMinutes = 145), SERVER_IMAGES)),
            total = 1,
            people = listOf(personRef(7, "Spike Lee")),
            collections = listOf(collectionRef("c1", "Spider Collection", 3)),
        )
        show(SearchUiState(text = "sp", hits = Loadable.Ready(hits)))
        compose.onNodeWithText(q(R.plurals.app_search_hits, 3, 3)).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.search_sections_titles)) and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithContentDescription("Spider-Man", substring = true).performClick()
        compose.reveal(compose.onNodeWithContentDescription("Spike Lee", substring = true)).performClick()
        compose.reveal(compose.onNodeWithContentDescription("Spider Collection", substring = true)).performClick()
        compose.onNode(hasContentDescription(s(R.string.app_search_clear)) and hasClickAction()).performClick()
        assertEquals(listOf("item:${ulid(1)}", "person:7", "collection:c1", "clear"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun matchesPhone() = matches()

    @Test @Config(qualifiers = TABLET)
    fun matchesTablet() = matches()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun matchesPhoneLargeFont() = matches()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun matchesTabletLargeFont() = matches()

    private fun nothingAndGenre() {
        show(SearchUiState(genre = "Drama", hits = Loadable.Empty))
        compose.onNodeWithText(s(R.string.search_empty_title)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_search_emptyGenre, "Drama")).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.onNodeWithContentDescription(s(R.string.app_search_genreRemove, "Drama")).performClick()
        assertEquals(listOf("nogenre"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun nothingPhone() = nothingAndGenre()

    @Test @Config(qualifiers = TABLET)
    fun nothingTablet() = nothingAndGenre()

    private fun failed() {
        show(SearchUiState(text = "spider", hits = Loadable.Failed(LoadError.Offline)))
        compose.onNodeWithText(s(R.string.app_state_offline)).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = failed()

    @Test @Config(qualifiers = TABLET)
    fun failedTablet() = failed()

    private fun tooShort() {
        show(SearchUiState(text = "s", tooShort = true))
        compose.onNodeWithText(s(R.string.search_tooShort)).assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun tooShortPhone() = tooShort()

    @Test @Config(qualifiers = TABLET)
    fun tooShortTablet() = tooShort()
}
