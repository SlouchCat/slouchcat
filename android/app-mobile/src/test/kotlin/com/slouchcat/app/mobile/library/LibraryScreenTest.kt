package com.slouchcat.app.mobile.library

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.Paged
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.library
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibrarySort
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

/** A library and its sheets (rule 8): two widths, 200 % font, every control labelled, back leaves. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class LibraryScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private val films = (1..4).map { MediaItem.from(itemRef(it, title = "Film $it"), SERVER_IMAGES) }
    private val ready = LibraryUiState(
        library = library(4, "Films", count = 4),
        items = Paged(Loadable.Ready(films), total = 4),
        genres = listOf(GenreRef("Drama", 3)),
    )

    private val actions = LibraryActions(
        back = { events += "back" },
        retry = { events += "retry" },
        sheet = { events += "sheet:$it" },
        sort = { events += "sort:${it.value}" },
        all = { events += "all" },
        unseen = { events += "unseen" },
        genre = { events += "genre:$it" },
        decade = { events += "decade:${it?.start}" },
        rating = { events += "rating:$it" },
        view = { events += "view" },
        item = { events += "item:${it.title}" },
        search = { events += "search" },
    )

    private fun show(state: LibraryUiState) = compose.setContent { SlouchcatTheme { LibraryScreen(state, actions) } }

    private fun page() {
        show(ready)
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText("Films").assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_browse_all)).assertIsSelected()
        compose.onNodeWithText(s(R.string.library_status_unwatched)).performClick()
        compose.onNodeWithText(s(R.string.library_filters_genre)).performClick()
        compose.onNodeWithText(s(R.string.library_filters_year)).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_browse_sortLabel, s(R.string.library_sort_added))).performClick()
        compose.onNodeWithContentDescription(s(R.string.nav_search)).performClick()
        compose.onNodeWithContentDescription("Film 2", substring = true).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("unseen", "sheet:Genre", "sheet:Year", "sheet:Sort", "search", "item:Film 2", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    private fun sheets(sheet: LibrarySheet, title: Int, pick: String, expected: String) {
        show(ready.copy(sheet = sheet))
        compose.onAllNodesWithText(s(title)).onLast().assertIsDisplayed()
        compose.onNode(hasText(pick) and hasClickAction()).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.common_close)).performScrollTo().performClick()
        assertEquals(listOf(expected, "sheet:null"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun sortSheetPhone() = sheets(LibrarySheet.Sort, R.string.app_browse_sortTitle, s(R.string.app_browse_sortTitleAz), "sort:${LibrarySort.TITLE.value}")

    @Test @Config(qualifiers = TABLET)
    fun sortSheetTablet() = sheets(LibrarySheet.Sort, R.string.app_browse_sortTitle, s(R.string.app_browse_sortTitleAz), "sort:${LibrarySort.TITLE.value}")

    @Test @Config(qualifiers = PHONE)
    fun genreSheetPhone() = sheets(LibrarySheet.Genre, R.string.library_filters_genre, "Drama", "genre:Drama")

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun genreSheetTabletLargeFont() = sheets(LibrarySheet.Genre, R.string.library_filters_genre, "Drama", "genre:Drama")

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun yearSheetPhoneLargeFont() = sheets(LibrarySheet.Year, R.string.app_browse_yearTitle, s(R.string.app_browse_decade, "2010"), "decade:2010")

    @Test @Config(qualifiers = TABLET)
    fun yearSheetTablet() = sheets(LibrarySheet.Year, R.string.app_browse_yearTitle, s(R.string.app_browse_decade, "2010"), "decade:2010")

    private fun listView() {
        show(ready.copy(list = true, ratings = listOf(0, 12, 16), items = Paged(Loadable.Ready(films), total = 1846)))
        compose.assertEveryControlLabelledAndTappable()
        // The number in the format of the language, the plural after the number.
        compose.onNodeWithText("1,846", substring = true).assertIsDisplayed()
        compose.onNodeWithContentDescription(s(R.string.app_browse_showGrid)).performClick()
        compose.onNodeWithText(s(R.string.library_filters_rating)).performClick()
        compose.onNodeWithContentDescription("Film 3", substring = true).performClick()
        assertEquals(listOf("view", "sheet:Rating", "item:Film 3"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun listViewPhone() = listView()

    @Test @Config(qualifiers = TABLET)
    fun listViewTablet() = listView()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun listViewPhoneLargeFont() = listView()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun listViewTabletLargeFont() = listView()

    /** «1’846 Titel» in Swiss German: the separator of the language, not a bare «1846». */
    @Test @Config(qualifiers = "de-rCH-w412dp-h915dp")
    fun theCountHasTheThousandsSeparatorOfTheLanguage() {
        show(ready.copy(items = Paged(Loadable.Ready(films), total = 1846)))
        val number = java.text.NumberFormat.getIntegerInstance(java.util.Locale.forLanguageTag("de-CH")).format(1846)
        compose.onNodeWithText("$number Titel").assertIsDisplayed()
    }

    @Test @Config(qualifiers = PHONE)
    fun ratingSheetPhone() {
        show(ready.copy(ratings = listOf(0, 12, 16), sheet = LibrarySheet.Rating))
        compose.onNode(hasText(s(R.string.rating_upTo, "12")) and hasClickAction()).performScrollTo().performClick()
        compose.onNode(hasText(s(R.string.library_filters_anyRating)) and hasClickAction()).performScrollTo().performClick()
        compose.onNodeWithText(s(R.string.common_close)).performScrollTo().performClick()
        assertEquals(listOf("rating:12", "rating:null", "sheet:null"), events)
    }

    @Test @Config(qualifiers = TABLET)
    fun ratingSheetTablet() {
        show(ready.copy(ratings = listOf(0, 12, 16), sheet = LibrarySheet.Rating))
        compose.onNode(hasText(s(R.string.rating_upTo, "16")) and hasClickAction()).performScrollTo().performClick()
        assertEquals(listOf("rating:16"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun nothingMatchesOffersTheReset() {
        show(ready.copy(filters = LibraryFilters(genre = "Drama"), items = Paged(Loadable.Empty)))
        compose.onNodeWithText(s(R.string.app_browse_noMatch)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_browse_reset)).performClick()
        compose.onNodeWithText("Drama").assertIsSelected()
        compose.assertEveryControlLabelledAndTappable()
        assertEquals(listOf("all"), events)
    }

    @Test @Config(qualifiers = TABLET)
    fun aLibraryNotThereSaysSo() {
        show(LibraryUiState(items = Paged(Loadable.Failed(LoadError.NotFound))))
        compose.onNodeWithText(s(R.string.app_state_notFound)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = PHONE)
    fun podcastsAreSquareWithoutTheYear() {
        val pods = listOf(MediaItem.from(itemRef(9, title = "Talk", kind = ItemKind.PODCAST, author = "Host"), SERVER_IMAGES))
        show(LibraryUiState(library = library(5, "Pods", LibraryKind.PODCASTS, count = 1), items = Paged(Loadable.Ready(pods), 1)))
        compose.onNodeWithText(s(R.string.library_status_unheard)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.library_filters_year)).assertDoesNotExist()
        val card = compose.onNodeWithContentDescription("Talk, Host").fetchSemanticsNode().boundsInRoot
        // Square cover plus two lines of text: wider than tall would be wrong for a 2:3 poster.
        assertEquals(true, card.height < card.width * 1.5f + 80)
        compose.assertEveryControlLabelledAndTappable()
    }
}
