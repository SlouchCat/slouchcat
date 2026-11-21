package com.slouchcat.app.mobile.home

import android.content.Context
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasScrollToNodeAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToNode
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.testing.PRIMARY
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.progress
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.strings.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private const val PHONE = "w412dp-h915dp"
private const val TABLET = "w1280dp-h800dp-land"

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/** The start page (rule 8): stage, rows, chips, two widths, 200 % font, every control labelled. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class HomeScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()
    private fun card(n: Int, title: String, rating: Int? = null) = MediaItem.from(itemRef(n, title = title, rating = rating, runtimeMinutes = 145), SERVER_IMAGES)

    private val rows = listOf(
        HomeSection.Spotlight("spotlight", listOf(card(1, "Stage One", rating = 12), card(2, "Stage Two")), start = 0, intervalSeconds = 7),
        HomeSection.Cards("continue", HomeRowKey.CONTINUE, listOf(MediaItem.from(itemRef(3, title = "Half Seen", progress = progress(40)), SERVER_IMAGES)), list = HomeListKey.CONTINUE),
        HomeSection.Cards("recent", HomeRowKey.RECENT, listOf(card(4, "Fresh"), card(5, "Newer")), list = HomeListKey.RECENT),
        HomeSection.Genres("genres", listOf(GenreRef("Drama", 3))),
        HomeSection.Libraries("libraries", listOf(LibraryChip(7, "Films", LibraryKind.VIDEO, seriesOnly = false, count = 3))),
    )

    private val links = HomeLinks(
        item = { events += "item:${it.title}" },
        play = { events += "play:${it.title}" },
        showAll = { row -> { events += "all:${row.id}" } },
        genre = { events += "genre:$it" },
        library = { events += "library:${it.name}" },
        profiles = { events += "profiles" },
    )

    private fun show(state: HomeUiState, links: HomeLinks = this.links) = compose.setContent {
        SlouchcatTheme {
            HomeScreen(state, links, onRetry = { events += "retry" }, onRefresh = { events += "refresh" }, onRowEnd = { events += "end:$it" }, onWatchlist = { events += "watch:${it.title}" }, rotate = false)
        }
    }

    private fun page() {
        show(HomeUiState(Loadable.Ready(rows), profile = PRIMARY))
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithText("Stage One").assertIsDisplayed()
        compose.onNodeWithContentDescription(s(R.string.rating_age, "12")).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.item_play)).performClick()
        compose.onNodeWithText(s(R.string.item_watchlist_add)).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_home_details, "Stage One")).performClick()
        compose.onNodeWithContentDescription(s(R.string.app_home_slide, 2, "2")).performClick()
        compose.onNodeWithContentDescription(s(R.string.nav_switchProfile, PRIMARY.name)).performClick()
        val list = compose.onAllNodes(hasScrollToNodeAction()).onFirst()
        list.performScrollToNode(hasContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_rows_continue))))
        compose.onNodeWithContentDescription(s(R.string.app_browse_showAllLabel, s(R.string.home_rows_continue))).performClick()
        compose.onNodeWithContentDescription("Half Seen", substring = true).performClick()
        list.performScrollToNode(hasText("Drama"))
        compose.onNodeWithText("Drama").performClick()
        list.performScrollToNode(hasText("Films"))
        compose.onNodeWithText("Films").performClick()
        // The bell only once the page «Benachrichtigungen» exists.
        compose.onNodeWithContentDescription(s(R.string.nav_notifications)).assertDoesNotExist()
        assertEquals(
            listOf("play:Stage One", "watch:Stage One", "item:Stage One", "profiles", "all:continue", "play:Half Seen", "genre:Drama", "library:Films"),
            events,
        )
    }

    @Test @Config(qualifiers = PHONE)
    fun pagePhone() = page()

    @Test @Config(qualifiers = TABLET)
    fun pageTablet() = page()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun pagePhoneLargeFont() = page()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun pageTabletLargeFont() = page()

    private fun withoutPages() {
        // No detail page, no player, no search yet: nothing that would lead nowhere is a control.
        show(HomeUiState(Loadable.Ready(rows), profile = PRIMARY), HomeLinks(profiles = { events += "profiles" }))
        compose.onNodeWithText(s(R.string.item_play)).assertDoesNotExist()
        compose.onNodeWithContentDescription(s(R.string.app_home_details, "Stage One")).assertDoesNotExist()
        compose.onNodeWithContentDescription("Fresh", substring = true).assertHasNoClickAction()
        compose.onNodeWithText("Drama").assertDoesNotExist()
        compose.onNodeWithText(s(R.string.item_watchlist_add)).assertHasClickAction()
        compose.assertEveryControlLabelledAndTappable()
    }

    @Test @Config(qualifiers = PHONE)
    fun withoutPagesPhone() = withoutPages()

    @Test @Config(qualifiers = TABLET)
    fun withoutPagesTablet() = withoutPages()

    private fun states() {
        show(HomeUiState(Loadable.Failed(LoadError.Offline), profile = PRIMARY))
        compose.onNodeWithText(s(R.string.app_state_offline)).assertIsDisplayed()
        compose.onNodeWithText(s(R.string.app_state_retry)).performClick()
        compose.assertEveryControlLabelledAndTappable()
        assertTrue("retry" in events)
    }

    @Test @Config(qualifiers = PHONE)
    fun failedPhone() = states()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun failedTabletLargeFont() = states()

    @Test @Config(qualifiers = PHONE)
    fun emptyPhone() {
        show(HomeUiState(Loadable.Empty, profile = PRIMARY))
        compose.onNodeWithText(s(R.string.home_empty_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
    }
}
