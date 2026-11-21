package com.slouchcat.app.mobile.collections

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
import com.slouchcat.app.mobile.testing.SERVER_IMAGES
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappable
import com.slouchcat.app.mobile.testing.assertEveryControlLabelledAndTappableWhenScrolled
import com.slouchcat.app.mobile.testing.collectionRef
import com.slouchcat.app.mobile.testing.itemRef
import com.slouchcat.app.mobile.testing.reveal
import com.slouchcat.app.mobile.testing.ulid
import com.slouchcat.core.api.generated.models.Collection
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

/** «Sammlungen» and a collection (rule 8): two widths, 200 % font, every button, the questions, Back. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class CollectionsScreenTest {
    @get:Rule
    val compose = createComposeRule()

    private val events = mutableListOf<String>()

    private fun showOverview(state: CollectionsUiState) = compose.setContent {
        SlouchcatTheme {
            CollectionsScreen(state, SERVER_IMAGES, onBack = { events += "back" }, onRetry = { events += "retry" }, onOpen = { id -> { events += "open:$id" } })
        }
    }

    private fun overview() {
        showOverview(CollectionsUiState(Loadable.Ready(listOf(collectionRef("c1", "28 Days Later", 4, cover = itemRef(1)), collectionRef("c2", "A Quiet Place", 2)))))
        compose.onNode(hasText(s(R.string.collection_all_title)) and isHeading()).assertIsDisplayed()
        compose.onNodeWithText(q(R.plurals.collection_all_count, 2, 2)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription("28 Days Later", substring = true).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("open:c1", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun overviewPhone() = overview()

    @Test @Config(qualifiers = TABLET)
    fun overviewTablet() = overview()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun overviewPhoneLargeFont() = overview()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun overviewTabletLargeFont() = overview()

    private fun overviewEmptyAndFailed(failed: Boolean) {
        showOverview(CollectionsUiState(if (failed) Loadable.Failed(LoadError.Offline) else Loadable.Empty))
        if (failed) {
            compose.onNode(hasText(s(R.string.app_state_retry)) and hasClickAction()).performClick()
            assertEquals(listOf("retry"), events)
        } else {
            compose.onNodeWithText(s(R.string.collection_all_empty)).assertIsDisplayed()
        }
    }

    @Test @Config(qualifiers = PHONE)
    fun overviewEmptyPhone() = overviewEmptyAndFailed(false)

    @Test @Config(qualifiers = TABLET)
    fun overviewFailedTablet() = overviewEmptyAndFailed(true)

    private fun page(seen: Int = 0, playable: Boolean = true): CollectionPage {
        val parts = listOf(itemRef(1, "Heartland"), itemRef(2, "Verity", inWatchlist = true), itemRef(3, "Gamer"))
        val collection = Collection(
            id = "c1", name = "28 Days Later", items = parts, total = 3, offset = 0, limit = 200,
            seenCount = seen, started = seen > 0, next = parts[0], nextPart = 1,
        )
        return CollectionPage(collection, parts, parts.map { MediaItem.from(it, SERVER_IMAGES) }).let { if (playable) it else it }
    }

    private fun showPage(state: CollectionUiState) = compose.setContent {
        SlouchcatTheme {
            CollectionScreen(
                state = state,
                images = SERVER_IMAGES,
                snackbar = SnackbarHostState(),
                onBack = { events += "back" },
                onRetry = { events += "retry" },
                onListAll = { events += "list" },
                onWatched = { events += "watched" },
                onConfirm = { events += "confirm:$it" },
                onDismiss = { events += "dismiss" },
                onOpen = { id -> { events += "open:$id" } },
                onPlay = { id -> { events += "play:$id" } },
            )
        }
    }

    private fun collection() {
        showPage(CollectionUiState(Loadable.Ready(page())))
        compose.onNode(hasText("28 Days Later") and isHeading()).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappableWhenScrolled()
        compose.reveal(compose.onNode(hasText(s(R.string.collection_play, "1")) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.collection_watchlist_add)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.watched_hint_set)) and hasClickAction())).performClick()
        compose.reveal(compose.onNode(hasText(s(R.string.app_collections_members)) and isHeading())).assertIsDisplayed()
        compose.reveal(compose.onNodeWithContentDescription("Gamer", substring = true)).performClick()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("play:${ulid(1)}", "list", "watched", "open:${ulid(3)}", "back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun collectionPhone() = collection()

    @Test @Config(qualifiers = TABLET)
    fun collectionTablet() = collection()

    @Test @Config(qualifiers = PHONE, fontScale = 2f)
    fun collectionPhoneLargeFont() = collection()

    @Test @Config(qualifiers = TABLET, fontScale = 2f)
    fun collectionTabletLargeFont() = collection()

    private fun halfAsks() {
        showPage(CollectionUiState(Loadable.Ready(page(seen = 1)), asking = CollectionWatched.Half))
        compose.onNodeWithText(s(R.string.watched_ask_halfTitle, "28 Days Later")).assertIsDisplayed()
        compose.onNode(hasText(s(R.string.collection_ask_markAll)) and hasClickAction()).performClick()
        compose.onNode(hasText(s(R.string.watched_ask_removeAll)) and hasClickAction()).performClick()
        compose.onNode(hasText(s(R.string.common_cancel)) and hasClickAction()).performClick()
        assertEquals(listOf("confirm:true", "confirm:false", "dismiss"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun halfAsksPhone() = halfAsks()

    @Test @Config(qualifiers = TABLET)
    fun halfAsksTablet() = halfAsks()

    private fun notFound() {
        showPage(CollectionUiState(Loadable.Failed(LoadError.NotFound)))
        compose.onNodeWithText(s(R.string.collection_notFound_title)).assertIsDisplayed()
        compose.assertEveryControlLabelledAndTappable()
        compose.onNodeWithContentDescription(s(R.string.common_back)).performClick()
        assertEquals(listOf("back"), events)
    }

    @Test @Config(qualifiers = PHONE)
    fun notFoundPhone() = notFound()

    @Test @Config(qualifiers = TABLET)
    fun notFoundTablet() = notFound()
}
