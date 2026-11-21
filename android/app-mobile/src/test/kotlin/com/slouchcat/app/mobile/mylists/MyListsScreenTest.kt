package com.slouchcat.app.mobile.mylists

import android.content.Context
import androidx.compose.material3.SnackbarHostState
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.Paged
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.posters
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.app.mobile.testing.watchedEntry
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

/** «Merkliste» and «Gesehen» (rule 8): two widths, 200 % font, «Bearbeiten», the ✕, empty, failed, Back. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class MyListsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun items() = listOf(
        MediaItem.from(itemRef(1, "Silo", images = posters(ulid(1))), SERVER_IMAGES),
        MediaItem.from(itemRef(2, "Level", images = posters(ulid(2))), SERVER_IMAGES),
    )

    private fun showWatchlist(state: MyListUiState<MediaItem>) = compose.setContent {
        SlouchcatTheme {
            WatchlistScreen(
                state = state,
                snackbar = SnackbarHostState(),
                onBack = { events += "back" },
                onRetry = { events += "retry" },
                onMore = {},
                onToggleEdit = { events += "edit" },
                onRemove = { events += "remove:${it.id}" },
                onOpen = { id -> { events += "open:$id" } },
                onHome = { events += "home" },
            )
        }
    }

    private fun watchlist(editing: Boolean) {
        showWatchlist(MyListUiState(Paged(Loadable.Ready(items()), total = 2), editing = editing))
        compose.onNode(hasText(s(R.string.nav_watchlist)) and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNode(hasText(s(if (editing) R.string.app_lists_done else R.string.app_lists_edit)) and hasClickAction()).performClick()
        if (editing) {
            compose.onNodeWithContentDescription(s(R.string.watchlist_removeLabel, "Silo")).performClick()
        } else {
            compose.onNodeWithContentDescription("Silo", substring = true).performClick()
        }
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("edit", if (editing) "remove:${ulid(1)}" else "open:${ulid(1)}", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchlistPhone() = watchlist(editing = false)

    @Test @Config(qualifiers = TABLET)
    fun watchlistTablet() = watchlist(editing = false)

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun watchlistEditingPhoneLargeFont() = watchlist(editing = true)

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun watchlistEditingTabletLargeFont() = watchlist(editing = true)

    private fun watchlistEmpty() {
        showWatchlist(MyListUiState(Paged(Loadable.Empty)))
        compose.onNodeWithText(s(R.string.watchlist_empty_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNode(hasText(s(R.string.app_lists_toStart)) and hasClickAction()).performClick()
        assertEquals(listOf("home"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchlistEmptyPhone() = watchlistEmpty()

    @Test @Config(qualifiers = TABLET)
    fun watchlistEmptyTablet() = watchlistEmpty()

    private fun watchlistFailed() {
        showWatchlist(MyListUiState(Paged(Loadable.Failed(LoadError.Offline))))
        compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
        assertEquals(listOf("retry"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchlistFailedPhone() = watchlistFailed()

    @Test @Config(qualifiers = TABLET)
    fun watchlistFailedTablet() = watchlistFailed()

    private fun showWatched(state: MyListUiState<WatchedCard>) = compose.setContent {
        SlouchcatTheme {
            WatchedScreen(
                state = state,
                snackbar = SnackbarHostState(),
                onBack = { events += "back" },
                onRetry = { events += "retry" },
                onMore = {},
                onToggleEdit = { events += "edit" },
                onAsk = { events += "ask:${it.id}" },
                onConfirm = { events += "confirm" },
                onDismiss = { events += "dismiss" },
                onOpen = { id -> { events += "open:$id" } },
                onHome = { events += "home" },
            )
        }
    }

    private fun cards() = listOf(
        WatchedCard.from(watchedEntry(1, "Film"), SERVER_IMAGES),
        WatchedCard.from(watchedEntry(2, "Series", episodes = 96, watchedCount = 19), SERVER_IMAGES),
    )

    private fun watched() {
        showWatched(MyListUiState(Paged(Loadable.Ready(cards()), total = 2), editing = true))
        compose.onNode(hasText(s(R.string.watched_page_title)) and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription(s(R.string.app_lists_unwatchLabel, "Series")).performClick()
        assertEquals(listOf("ask:${ulid(2)}"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchedPhone() = watched()

    @Test @Config(qualifiers = TABLET)
    fun watchedTablet() = watched()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun watchedPhoneLargeFont() = watched()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun watchedTabletLargeFont() = watched()

    private fun watchedAsks() {
        val series = cards()[1]
        showWatched(MyListUiState(Paged(Loadable.Ready(cards()), total = 2), editing = true, asking = series))
        compose.onNodeWithText(s(R.string.watched_ask_removeTitle, "Series")).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.common_cancel)) and hasClickAction()).performClick()
        compose.onNode(hasText(s(R.string.watched_ask_removeAll)) and hasClickAction()).performClick()
        assertEquals(listOf("dismiss", "confirm"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchedAsksPhone() = watchedAsks()

    @Test @Config(qualifiers = TABLET)
    fun watchedAsksTablet() = watchedAsks()

    private fun watchedEmpty() {
        showWatched(MyListUiState(Paged(Loadable.Empty)))
        compose.onNodeWithText(s(R.string.watched_page_empty_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun watchedEmptyPhone() = watchedEmpty()

    @Test @Config(qualifiers = TABLET)
    fun watchedEmptyTablet() = watchedEmpty()
}
